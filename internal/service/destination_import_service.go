package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/ports"
)

var (
	ErrImportEmptyFile        = errors.New("csv file is empty")
	ErrImportTooLarge         = errors.New("csv file exceeds maximum size")
	ErrImportInvalidHeaders   = errors.New("csv headers missing required columns")
	ErrImportRowLimitExceeded = errors.New("csv exceeds maximum allowed rows")
)

type destinationWorkflow interface {
	CreateDraft(ctx context.Context, authorID uuid.UUID, input DestinationDraftInput) (*domain.DestinationChangeRequest, error)
	SubmitDraft(ctx context.Context, changeID uuid.UUID, authorID uuid.UUID) (*domain.DestinationChangeRequest, error)
	ValidateFields(action domain.DestinationChangeAction, fields domain.DestinationChangeFields, requireAll bool) error
}

type DestinationImportServiceConfig struct {
	Bucket        string
	MaxRows       int
	MaxFileBytes  int64
	MaxPendingIDs int
}

type destinationLookup interface {
	FindBySlug(ctx context.Context, slug string) (*domain.Destination, error)
}

type DestinationImportService struct {
	repo          ports.DestinationImportRepository
	destinations  destinationLookup
	workflow      destinationWorkflow
	storage       ports.ObjectStorage
	bucket        string
	maxRows       int
	maxFileBytes  int64
	maxPendingIDs int
	now           func() time.Time
}

func NewDestinationImportService(repo ports.DestinationImportRepository, destRepo destinationLookup, workflow destinationWorkflow, storage ports.ObjectStorage, cfg DestinationImportServiceConfig) *DestinationImportService {
	maxRows := cfg.MaxRows
	if maxRows <= 0 {
		maxRows = 500
	}
	maxFile := cfg.MaxFileBytes
	if maxFile <= 0 {
		maxFile = 5 * 1024 * 1024
	}
	maxPending := cfg.MaxPendingIDs
	if maxPending <= 0 {
		maxPending = 25
	}

	return &DestinationImportService{
		repo:          repo,
		destinations:  destRepo,
		workflow:      workflow,
		storage:       storage,
		bucket:        cfg.Bucket,
		maxRows:       maxRows,
		maxFileBytes:  maxFile,
		maxPendingIDs: maxPending,
		now:           time.Now,
	}
}

func (s *DestinationImportService) Import(ctx context.Context, uploadedBy uuid.UUID, filename string, contents []byte, dryRun bool, notes *string) (_ *domain.DestinationImportJob, _ []domain.DestinationImportRow, err error) {
	if len(contents) == 0 {
		return nil, nil, ErrImportEmptyFile
	}
	if s.maxFileBytes > 0 && int64(len(contents)) > s.maxFileBytes {
		return nil, nil, ErrImportTooLarge
	}

	header, records, err := parseCSV(contents)
	if err != nil {
		return nil, nil, err
	}
	if len(records) == 0 {
		return nil, nil, ErrImportEmptyFile
	}
	if s.maxRows > 0 && len(records) > s.maxRows {
		return nil, nil, ErrImportRowLimitExceeded
	}

	required := []string{"name", "category", "city", "country", "description", "latitude", "longitude", "contact", "hero_image_url"}
	if missing := missingColumns(header, required); len(missing) > 0 {
		return nil, nil, fmt.Errorf("%w: %s", ErrImportInvalidHeaders, strings.Join(missing, ", "))
	}

	jobID := uuid.New()
	objectName := buildObjectName(jobID, filename)
	if s.storage != nil && s.bucket != "" {
		if _, err := s.storage.Upload(ctx, s.bucket, objectName, "text/csv", bytes.NewReader(contents), int64(len(contents))); err != nil {
			return nil, nil, err
		}
	}

	job := &domain.DestinationImportJob{
		ID:             jobID,
		UploadedBy:     uploadedBy,
		Status:         domain.DestinationImportStatusProcessing,
		DryRun:         dryRun,
		FileKey:        objectName,
		Notes:          notes,
		SubmittedAt:    s.now(),
		TotalRows:      len(records),
		ProcessedRows:  0,
		RowsFailed:     0,
		ChangesCreated: 0,
	}

	insertedJob, err := s.repo.CreateJob(ctx, job)
	if err != nil {
		return nil, nil, err
	}
	job = insertedJob

	defer func() {
		if err != nil && job != nil {
			s.failJob(ctx, job)
		}
	}()

	seenSlugs := make(map[string]int)
	existingSlugs := make(map[string]bool)
	pendingIDs := make([]uuid.UUID, 0, s.maxPendingIDs)
	rows := make([]domain.DestinationImportRow, 0, len(records))

	for idx, record := range records {
		rowNumber := idx + 2 // account for header line
		normalized := rowToMap(header, record)

		fields, parseErrs := buildChangeFields(normalized)
		rowErrors := append([]string{}, parseErrs...)

		slugVal := normalized["slug"]
		if slugVal != "" {
			lower := strings.ToLower(slugVal)
			if prev, ok := seenSlugs[lower]; ok {
				rowErrors = append(rowErrors, fmt.Sprintf("slug duplicates row %d", prev))
			} else {
				seenSlugs[lower] = rowNumber
				exists, err := s.slugExists(ctx, lower, existingSlugs)
				if err != nil {
					return nil, nil, err
				}
				if exists {
					rowErrors = append(rowErrors, "slug already exists")
				}
			}
		}

		if fields.HeroImageURL == nil || strings.TrimSpace(*fields.HeroImageURL) == "" {
			rowErrors = append(rowErrors, "hero image url is required")
		}

		if err := s.workflow.ValidateFields(domain.DestinationChangeActionCreate, fields, true); err != nil {
			rowErrors = append(rowErrors, err.Error())
		}

		var changeID *uuid.UUID
		if len(rowErrors) == 0 && !dryRun {
			change, err := s.workflow.CreateDraft(ctx, uploadedBy, DestinationDraftInput{
				Action: domain.DestinationChangeActionCreate,
				Fields: fields,
			})
			if err != nil {
				rowErrors = append(rowErrors, err.Error())
			} else {
				change, err = s.workflow.SubmitDraft(ctx, change.ID, uploadedBy)
				if err != nil {
					rowErrors = append(rowErrors, err.Error())
				} else if change != nil {
					changeID = &change.ID
					if len(pendingIDs) < s.maxPendingIDs {
						pendingIDs = append(pendingIDs, change.ID)
					}
					job.ChangesCreated++
				}
			}
		}

		row := domain.DestinationImportRow{
			JobID:     job.ID,
			RowNumber: rowNumber,
			Action:    domain.DestinationChangeActionCreate,
			Payload:   fields,
		}

		if len(rowErrors) > 0 {
			job.RowsFailed++
			row.Status = domain.DestinationImportRowStatusFailed
			message := strings.Join(rowErrors, "; ")
			row.ErrorMessage = &message
		} else if dryRun {
			row.Status = domain.DestinationImportRowStatusSkipped
		} else {
			row.Status = domain.DestinationImportRowStatusPendingReview
			row.ChangeID = changeID
		}

		insertedRow, err := s.repo.InsertRow(ctx, &row)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, *insertedRow)
		job.ProcessedRows++
	}

	completed := s.now()
	job.Status = domain.DestinationImportStatusCompleted
	job.CompletedAt = &completed
	job.PendingIDs = pendingIDs
	job.Rows = rows

	updatedJob, err := s.repo.UpdateJob(ctx, job)
	if err != nil {
		return nil, nil, err
	}
	updatedJob.PendingIDs = pendingIDs
	updatedJob.Rows = rows
	return updatedJob, rows, nil
}

func (s *DestinationImportService) GetJob(ctx context.Context, jobID uuid.UUID) (*domain.DestinationImportJob, []domain.DestinationImportRow, error) {
	job, err := s.repo.FindJobByID(ctx, jobID)
	if err != nil {
		return nil, nil, err
	}
	rows, err := s.repo.ListRowsByJob(ctx, job.ID)
	if err != nil {
		return nil, nil, err
	}
	job.Rows = rows
	job.PendingIDs = extractPendingIDs(rows, s.maxPendingIDs)
	return job, rows, nil
}

func (s *DestinationImportService) slugExists(ctx context.Context, slug string, cache map[string]bool) (bool, error) {
	if val, ok := cache[slug]; ok {
		return val, nil
	}
	dest, err := s.destinations.FindBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			cache[slug] = false
			return false, nil
		}
		return false, err
	}
	cache[slug] = dest != nil
	return cache[slug], nil
}

func parseCSV(contents []byte) ([]string, [][]string, error) {
	reader := csv.NewReader(bytes.NewReader(contents))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil, ErrImportEmptyFile
		}
		return nil, nil, err
	}

	normHeader := make([]string, len(header))
	for i, h := range header {
		normHeader[i] = normalizeHeader(h)
	}

	rows := make([][]string, 0)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if isRecordEmpty(record) {
			continue
		}
		rows = append(rows, record)
	}
	return normHeader, rows, nil
}

func missingColumns(header []string, required []string) []string {
	set := make(map[string]struct{}, len(header))
	for _, h := range header {
		set[h] = struct{}{}
	}
	var missing []string
	for _, req := range required {
		if _, ok := set[req]; !ok {
			missing = append(missing, req)
		}
	}
	return missing
}

func rowToMap(header []string, record []string) map[string]string {
	out := make(map[string]string, len(header))
	for idx, key := range header {
		val := ""
		if idx < len(record) {
			val = strings.TrimSpace(record[idx])
		}
		out[key] = val
	}
	return out
}

func buildChangeFields(values map[string]string) (domain.DestinationChangeFields, []string) {
	var errs []string
	fields := domain.DestinationChangeFields{}

	if v := strings.TrimSpace(values["name"]); v != "" {
		fields.Name = stringPointer(v)
	}
	if v := strings.TrimSpace(values["slug"]); v != "" {
		fields.Slug = stringPointer(strings.ToLower(v))
	}
	if v := strings.TrimSpace(values["category"]); v != "" {
		fields.Category = stringPointer(v)
	}
	if v := strings.TrimSpace(values["city"]); v != "" {
		fields.City = stringPointer(v)
	}
	if v := strings.TrimSpace(values["country"]); v != "" {
		fields.Country = stringPointer(v)
	}
	if v := strings.TrimSpace(values["description"]); v != "" {
		fields.Description = stringPointer(v)
	}
	if v := strings.TrimSpace(values["contact"]); v != "" {
		fields.Contact = stringPointer(v)
	}
	if v := strings.TrimSpace(values["opening_time"]); v != "" {
		fields.OpeningTime = stringPointer(v)
	}
	if v := strings.TrimSpace(values["closing_time"]); v != "" {
		fields.ClosingTime = stringPointer(v)
	}
	if latStr := strings.TrimSpace(values["latitude"]); latStr != "" {
		if lat, err := parseFloat(latStr); err == nil {
			fields.Latitude = &lat
		} else {
			errs = append(errs, fmt.Sprintf("invalid latitude: %s", err.Error()))
		}
	}
	if lngStr := strings.TrimSpace(values["longitude"]); lngStr != "" {
		if lng, err := parseFloat(lngStr); err == nil {
			fields.Longitude = &lng
		} else {
			errs = append(errs, fmt.Sprintf("invalid longitude: %s", err.Error()))
		}
	}

	if hero := strings.TrimSpace(values["hero_image_url"]); hero != "" {
		fields.HeroImageURL = stringPointer(hero)
	}

	status := strings.TrimSpace(values["status"])
	if status == "" {
		status = string(domain.DestinationStatusPublished)
	}
	switch strings.ToLower(status) {
	case "draft":
		fields.Status = statusPtr(domain.DestinationStatusDraft)
	case "published":
		fields.Status = statusPtr(domain.DestinationStatusPublished)
	case "archived":
		fields.Status = statusPtr(domain.DestinationStatusArchived)
	default:
		errs = append(errs, "status must be draft, published, or archived")
	}

	if gallery := buildGallery(values); gallery != nil {
		fields.Gallery = gallery
	}

	return fields, errs
}

func buildGallery(values map[string]string) *domain.DestinationGallery {
	items := make([]domain.DestinationMedia, 0, 3)
	for idx := 1; idx <= 3; idx++ {
		urlKey := fmt.Sprintf("gallery_%d_url", idx)
		captionKey := fmt.Sprintf("gallery_%d_caption", idx)
		if url := strings.TrimSpace(values[urlKey]); url != "" {
			item := domain.DestinationMedia{
				URL:      url,
				Ordering: idx - 1,
			}
			if caption := strings.TrimSpace(values[captionKey]); caption != "" {
				item.Caption = stringPointer(caption)
			}
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return nil
	}
	gallery := domain.DestinationGallery(items)
	return &gallery
}

func stringPointer(val string) *string {
	if val == "" {
		return nil
	}
	return &val
}

func statusPtr(val domain.DestinationStatus) *domain.DestinationStatus {
	return &val
}

func parseFloat(raw string) (float64, error) {
	return strconv.ParseFloat(raw, 64)
}

func isRecordEmpty(record []string) bool {
	for _, field := range record {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}

func buildObjectName(jobID uuid.UUID, filename string) string {
	name := strings.TrimSpace(filename)
	if name == "" {
		name = "upload.csv"
	}
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, " ", "_")
	return fmt.Sprintf("destinations/imports/%s/%s", jobID.String(), name)
}

func extractPendingIDs(rows []domain.DestinationImportRow, max int) []uuid.UUID {
	result := make([]uuid.UUID, 0, max)
	for _, row := range rows {
		if row.Status == domain.DestinationImportRowStatusPendingReview && row.ChangeID != nil {
			result = append(result, *row.ChangeID)
			if len(result) == max {
				break
			}
		}
	}
	return result
}

func normalizeHeader(h string) string {
	return strings.TrimSpace(strings.ToLower(h))
}

func (s *DestinationImportService) failJob(ctx context.Context, job *domain.DestinationImportJob) {
	if job == nil {
		return
	}
	job.Status = domain.DestinationImportStatusFailed
	now := s.now()
	job.CompletedAt = &now
	_, _ = s.repo.UpdateJob(ctx, job)
}
