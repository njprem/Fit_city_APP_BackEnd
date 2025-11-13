package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/media"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/ports"
)

var (
	ErrDestinationChangeNotFound   = errors.New("destination change request not found")
	ErrDestinationNotFound         = errors.New("destination not found")
	ErrChangeNotEditable           = errors.New("change request is not editable")
	ErrInvalidChangeAction         = errors.New("invalid change action")
	ErrInvalidChangeState          = errors.New("invalid change state for operation")
	ErrReviewerConflict            = errors.New("reviewer cannot approve own submission")
	ErrHardDeleteNotAllowed        = errors.New("hard delete not allowed")
	ErrHeroImageRequired           = errors.New("hero image required")
	ErrDestinationChangeValidation = errors.New("destination change validation failed")
	ErrHeroImageTooLarge           = errors.New("hero image exceeds maximum size")
	ErrHeroImageUnsupportedType    = errors.New("unsupported hero image content type")
	ErrGalleryImageRequired        = errors.New("gallery image required")
	ErrGalleryImageTooLarge        = errors.New("gallery image exceeds maximum size")
	ErrGalleryImageUnsupportedType = errors.New("unsupported gallery image content type")
	ErrChangeAlreadyProcessed      = errors.New("change request already processed")
	errDefaultImageContentType     = "image/jpeg"
	supportedImageTypes            = map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
		"image/webp": {},
	}
)

var slugAllowed = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type HeroImageUpload struct {
	Reader      io.Reader
	Size        int64
	FileName    string
	ContentType string
}

type GalleryImageUpload struct {
	Reader      io.Reader
	Size        int64
	FileName    string
	ContentType string
}

type GalleryUploadResult struct {
	UploadID string
	URL      string
	Ordering int
}

type DestinationDraftInput struct {
	Action        domain.DestinationChangeAction
	DestinationID *uuid.UUID
	Fields        domain.DestinationChangeFields
}

type DestinationWorkflowConfig struct {
	Bucket            string
	PublicBaseURL     string
	ImageMaxBytes     int64
	ImageMaxDimension int
	AllowedCategories []string
	ApprovalRequired  bool
	HardDeleteAllowed bool
	ImageProcessor    media.Processor
}

type DestinationWorkflowService struct {
	destinations ports.DestinationRepository
	changes      ports.DestinationChangeRepository
	versions     ports.DestinationVersionRepository
	storage      ports.ObjectStorage

	bucket            string
	publicBase        string
	imageMaxBytes     int64
	imageMaxDimension int
	allowedCategories map[string]struct{}
	approvalRequired  bool
	hardDeleteAllowed bool
	now               func() time.Time
	imageProcessor    media.Processor
}

func NewDestinationWorkflowService(destRepo ports.DestinationRepository, changeRepo ports.DestinationChangeRepository, versionRepo ports.DestinationVersionRepository, storage ports.ObjectStorage, cfg DestinationWorkflowConfig) *DestinationWorkflowService {
	allowed := make(map[string]struct{}, len(cfg.AllowedCategories))
	for _, cat := range cfg.AllowedCategories {
		trimmed := strings.ToLower(strings.TrimSpace(cat))
		if trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}
	imageMax := cfg.ImageMaxBytes
	if imageMax <= 0 {
		imageMax = 5 * 1024 * 1024
	}
	publicBase := strings.TrimRight(cfg.PublicBaseURL, "/")
	maxDimension := cfg.ImageMaxDimension
	if maxDimension <= 0 {
		maxDimension = media.DefaultMaxDimension
	}

	return &DestinationWorkflowService{
		destinations:      destRepo,
		changes:           changeRepo,
		versions:          versionRepo,
		storage:           storage,
		bucket:            cfg.Bucket,
		publicBase:        publicBase,
		imageMaxBytes:     imageMax,
		imageMaxDimension: maxDimension,
		allowedCategories: allowed,
		approvalRequired:  cfg.ApprovalRequired,
		hardDeleteAllowed: cfg.HardDeleteAllowed,
		now:               time.Now,
		imageProcessor:    cfg.ImageProcessor,
	}
}

func (s *DestinationWorkflowService) SetClock(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

func (s *DestinationWorkflowService) CreateDraft(ctx context.Context, authorID uuid.UUID, input DestinationDraftInput) (*domain.DestinationChangeRequest, error) {
	if err := s.validateDraftInput(ctx, authorID, input, true); err != nil {
		return nil, err
	}

	change := &domain.DestinationChangeRequest{
		ID:            uuid.Nil,
		DestinationID: input.DestinationID,
		Action:        input.Action,
		Payload:       input.Fields,
		Status:        domain.DestinationChangeStatusDraft,
		DraftVersion:  1,
		SubmittedBy:   authorID,
		CreatedAt:     s.now(),
		UpdatedAt:     s.now(),
	}

	if !s.isDeleteAction(input.Action) {
		requireAll := input.Action == domain.DestinationChangeActionCreate
		if err := s.validateFields(input.Action, input.Fields, requireAll); err != nil {
			return nil, err
		}
	}

	return s.changes.Create(ctx, change)
}

func (s *DestinationWorkflowService) UpdateDraft(ctx context.Context, changeID uuid.UUID, authorID uuid.UUID, expectedDraftVersion int, fields domain.DestinationChangeFields) (*domain.DestinationChangeRequest, error) {
	change, err := s.changes.FindByID(ctx, changeID)
	if err != nil {
		return nil, ErrDestinationChangeNotFound
	}
	if change.SubmittedBy != authorID {
		return nil, ErrForbidden
	}
	if change.Status != domain.DestinationChangeStatusDraft && change.Status != domain.DestinationChangeStatusRejected {
		return nil, ErrChangeNotEditable
	}
	if expectedDraftVersion != change.DraftVersion {
		return nil, fmt.Errorf("%w: stale draft version", ErrInvalidChangeState)
	}

	if !s.isDeleteAction(change.Action) {
		if err := s.validateFields(change.Action, fields, change.Action == domain.DestinationChangeActionCreate); err != nil {
			return nil, err
		}
	} else if fields.HardDelete != nil && *fields.HardDelete && !s.hardDeleteAllowed {
		return nil, ErrHardDeleteNotAllowed
	}

	change.Payload = fields
	change.DraftVersion++
	change.UpdatedAt = s.now()
	return s.changes.Update(ctx, change)
}

func (s *DestinationWorkflowService) SubmitDraft(ctx context.Context, changeID uuid.UUID, authorID uuid.UUID) (*domain.DestinationChangeRequest, error) {
	change, err := s.changes.FindByID(ctx, changeID)
	if err != nil {
		return nil, ErrDestinationChangeNotFound
	}
	if change.SubmittedBy != authorID {
		return nil, ErrForbidden
	}
	if change.Status != domain.DestinationChangeStatusDraft && change.Status != domain.DestinationChangeStatusRejected {
		return nil, ErrInvalidChangeState
	}
	if !s.isDeleteAction(change.Action) {
		if err := s.validateFields(change.Action, change.Payload, change.Action == domain.DestinationChangeActionCreate); err != nil {
			return nil, err
		}
	}

	now := s.now()
	return s.changes.MarkSubmitted(ctx, change.ID, now)
}

func (s *DestinationWorkflowService) Approve(ctx context.Context, changeID uuid.UUID, reviewerID uuid.UUID) (*domain.DestinationChangeRequest, *domain.Destination, error) {
	change, err := s.changes.FindByID(ctx, changeID)
	if err != nil {
		return nil, nil, ErrDestinationChangeNotFound
	}
	if change.Status != domain.DestinationChangeStatusPendingReview {
		return nil, nil, ErrInvalidChangeState
	}
	if s.approvalRequired && change.SubmittedBy == reviewerID {
		return nil, nil, ErrReviewerConflict
	}
	if !s.isDeleteAction(change.Action) {
		if err := s.validateFields(change.Action, change.Payload, change.Action == domain.DestinationChangeActionCreate); err != nil {
			return nil, nil, err
		}
	}

	var destination *domain.Destination
	switch change.Action {
	case domain.DestinationChangeActionCreate:
		destination, err = s.applyCreate(ctx, change, reviewerID)
	case domain.DestinationChangeActionUpdate:
		destination, err = s.applyUpdate(ctx, change, reviewerID)
	case domain.DestinationChangeActionDelete:
		destination, err = s.applyDelete(ctx, change, reviewerID)
	default:
		return nil, nil, ErrInvalidChangeAction
	}
	if err != nil {
		return nil, nil, err
	}

	now := s.now()
	change.Status = domain.DestinationChangeStatusApproved
	change.ReviewedAt = &now
	change.ReviewedBy = &reviewerID
	change.UpdatedAt = now
	if destination != nil {
		change.PublishedVersion = &destination.Version
	}

	change, err = s.changes.SetStatus(ctx, change.ID, change.Status, change.ReviewedBy, change.ReviewMessage, change.PublishedVersion)
	if err != nil {
		return nil, nil, err
	}

	return change, destination, nil
}

func (s *DestinationWorkflowService) Reject(ctx context.Context, changeID uuid.UUID, reviewerID uuid.UUID, message string) (*domain.DestinationChangeRequest, error) {
	change, err := s.changes.FindByID(ctx, changeID)
	if err != nil {
		return nil, ErrDestinationChangeNotFound
	}
	if change.Status != domain.DestinationChangeStatusPendingReview {
		return nil, ErrInvalidChangeState
	}
	if s.approvalRequired && change.SubmittedBy == reviewerID {
		return nil, ErrReviewerConflict
	}
	now := s.now()
	change.Status = domain.DestinationChangeStatusRejected
	change.ReviewedAt = &now
	change.ReviewedBy = &reviewerID
	change.ReviewMessage = stringPtr(strings.TrimSpace(message))
	change.UpdatedAt = now

	return s.changes.SetStatus(ctx, change.ID, change.Status, change.ReviewedBy, change.ReviewMessage, nil)
}

func (s *DestinationWorkflowService) GetChange(ctx context.Context, changeID uuid.UUID) (*domain.DestinationChangeRequest, error) {
	change, err := s.changes.FindByID(ctx, changeID)
	if err != nil {
		return nil, ErrDestinationChangeNotFound
	}
	return change, nil
}

func (s *DestinationWorkflowService) ListChanges(ctx context.Context, filter domain.DestinationChangeFilter) ([]domain.DestinationChangeRequest, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.changes.List(ctx, filter)
}

func (s *DestinationWorkflowService) UploadHeroImage(ctx context.Context, changeID uuid.UUID, authorID uuid.UUID, image HeroImageUpload) (*domain.DestinationChangeRequest, error) {
	if image.Size <= 0 || image.Reader == nil {
		return nil, fmt.Errorf("%w: empty upload", ErrHeroImageRequired)
	}
	if image.Size > s.imageMaxBytes {
		return nil, ErrHeroImageTooLarge
	}

	change, err := s.changes.FindByID(ctx, changeID)
	if err != nil {
		return nil, ErrDestinationChangeNotFound
	}
	if change.SubmittedBy != authorID {
		return nil, ErrForbidden
	}
	if change.Status != domain.DestinationChangeStatusDraft && change.Status != domain.DestinationChangeStatusRejected {
		return nil, ErrChangeNotEditable
	}

	contentType := strings.TrimSpace(image.ContentType)
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(image.FileName))
	}
	if contentType == "" {
		contentType = errDefaultImageContentType
	}
	if _, ok := supportedImageTypes[contentType]; !ok {
		return nil, ErrHeroImageUnsupportedType
	}

	exts, _ := mime.ExtensionsByType(contentType)
	ext := ".img"
	if len(exts) > 0 {
		ext = exts[0]
	} else if fileExt := filepath.Ext(image.FileName); fileExt != "" {
		ext = fileExt
	}

	objectName := fmt.Sprintf("destinations/changes/%s/%s%s", changeID.String(), uuid.NewString(), ext)
	reader, size, contentType, err := prepareImageForUpload(ctx, s.imageProcessor, media.Upload{
		Reader:      image.Reader,
		Size:        image.Size,
		FileName:    image.FileName,
		ContentType: contentType,
	}, s.imageMaxDimension)
	if err != nil {
		return nil, err
	}

	publicURL, err := s.storage.Upload(ctx, s.bucket, objectName, contentType, reader, size)
	if err != nil {
		return nil, err
	}
	if s.publicBase != "" {
		publicURL = strings.TrimRight(s.publicBase, "/") + "/" + strings.TrimLeft(objectName, "/")
	}

	change.HeroImageTempKey = stringPtr(objectName)
	change.Payload.HeroImageUploadID = stringPtr(objectName)
	change.Payload.HeroImageURL = stringPtr(publicURL)
	change.UpdatedAt = s.now()
	change.DraftVersion++

	return s.changes.Update(ctx, change)
}

func (s *DestinationWorkflowService) UploadGalleryImages(ctx context.Context, changeID uuid.UUID, authorID uuid.UUID, uploads []GalleryImageUpload) (*domain.DestinationChangeRequest, []GalleryUploadResult, error) {
	if len(uploads) == 0 {
		return nil, nil, ErrGalleryImageRequired
	}

	change, err := s.changes.FindByID(ctx, changeID)
	if err != nil {
		return nil, nil, ErrDestinationChangeNotFound
	}
	if change.SubmittedBy != authorID {
		return nil, nil, ErrForbidden
	}
	if change.Status != domain.DestinationChangeStatusDraft && change.Status != domain.DestinationChangeStatusRejected {
		return nil, nil, ErrChangeNotEditable
	}

	existing := domain.DestinationGallery(nil)
	if change.Payload.Gallery != nil {
		existing = cloneGallery(*change.Payload.Gallery)
	}
	baseOrdering := len(existing)
	results := make([]GalleryUploadResult, 0, len(uploads))

	for idx, upload := range uploads {
		if upload.Reader == nil || upload.Size <= 0 {
			return nil, nil, ErrGalleryImageRequired
		}
		if upload.Size > s.imageMaxBytes {
			return nil, nil, ErrGalleryImageTooLarge
		}
		contentType := strings.TrimSpace(upload.ContentType)
		if contentType == "" {
			contentType = mime.TypeByExtension(filepath.Ext(upload.FileName))
		}
		if contentType == "" {
			contentType = errDefaultImageContentType
		}
		if _, ok := supportedImageTypes[contentType]; !ok {
			return nil, nil, ErrGalleryImageUnsupportedType
		}

		exts, _ := mime.ExtensionsByType(contentType)
		ext := ".img"
		if len(exts) > 0 {
			ext = exts[0]
		} else if fileExt := filepath.Ext(upload.FileName); fileExt != "" {
			ext = fileExt
		}

		objectName := fmt.Sprintf("destinations/changes/%s/gallery/%s%s", changeID.String(), uuid.NewString(), ext)
		reader, size, contentType, err := prepareImageForUpload(ctx, s.imageProcessor, media.Upload{
			Reader:      upload.Reader,
			Size:        upload.Size,
			FileName:    upload.FileName,
			ContentType: contentType,
		}, s.imageMaxDimension)
		if err != nil {
			return nil, nil, err
		}

		publicURL, err := s.storage.Upload(ctx, s.bucket, objectName, contentType, reader, size)
		if err != nil {
			return nil, nil, err
		}
		if s.publicBase != "" {
			publicURL = strings.TrimRight(s.publicBase, "/") + "/" + strings.TrimLeft(objectName, "/")
		}

		ordering := baseOrdering + idx
		existing = append(existing, domain.DestinationMedia{
			URL:      publicURL,
			Ordering: ordering,
		})
		results = append(results, GalleryUploadResult{
			UploadID: objectName,
			URL:      publicURL,
			Ordering: ordering,
		})
	}

	if len(existing) == 0 {
		change.Payload.Gallery = nil
	} else {
		cloned := cloneGallery(existing)
		change.Payload.Gallery = &cloned
	}
	change.UpdatedAt = s.now()
	change.DraftVersion++

	updated, err := s.changes.Update(ctx, change)
	if err != nil {
		return nil, nil, err
	}
	return updated, results, nil
}

func (s *DestinationWorkflowService) validateDraftInput(ctx context.Context, authorID uuid.UUID, input DestinationDraftInput, creating bool) error {
	if input.Action != domain.DestinationChangeActionCreate &&
		input.Action != domain.DestinationChangeActionUpdate &&
		input.Action != domain.DestinationChangeActionDelete {
		return ErrInvalidChangeAction
	}

	if s.isDeleteAction(input.Action) {
		if input.DestinationID == nil {
			return fmt.Errorf("%w: destination_id required for delete", ErrDestinationChangeValidation)
		}
		if _, err := s.destinations.FindByID(ctx, *input.DestinationID); err != nil {
			return ErrDestinationNotFound
		}
		if input.Fields.HardDelete != nil && *input.Fields.HardDelete && !s.hardDeleteAllowed {
			return ErrHardDeleteNotAllowed
		}
		return nil
	}

	if input.Action == domain.DestinationChangeActionCreate {
		if input.DestinationID != nil {
			return fmt.Errorf("%w: destination_id must be empty for creates", ErrDestinationChangeValidation)
		}
	} else if input.DestinationID == nil {
		return fmt.Errorf("%w: destination_id required", ErrDestinationChangeValidation)
	} else {
		if _, err := s.destinations.FindByID(ctx, *input.DestinationID); err != nil {
			return ErrDestinationNotFound
		}
	}
	return nil
}

func (s *DestinationWorkflowService) validateFields(action domain.DestinationChangeAction, fields domain.DestinationChangeFields, requireAll bool) error {
	var problems []string
	trim := func(ptr *string) *string {
		if ptr == nil {
			return nil
		}
		val := strings.TrimSpace(*ptr)
		return &val
	}

	fields.Name = trim(fields.Name)
	fields.Description = trim(fields.Description)
	fields.City = trim(fields.City)
	fields.Country = trim(fields.Country)
	fields.Category = trim(fields.Category)
	fields.Slug = trim(fields.Slug)
	fields.Contact = trim(fields.Contact)
	fields.OpeningTime = trim(fields.OpeningTime)
	fields.ClosingTime = trim(fields.ClosingTime)

	if requireAll || fields.Name != nil {
		if fields.Name == nil || *fields.Name == "" {
			problems = append(problems, "name is required")
		}
	}

	if fields.Slug != nil && *fields.Slug != "" {
		if !slugAllowed.MatchString(*fields.Slug) {
			problems = append(problems, "slug must contain lowercase letters, numbers, and hyphens only")
		}
	}

	if fields.Category != nil && len(s.allowedCategories) > 0 {
		if _, ok := s.allowedCategories[strings.ToLower(*fields.Category)]; !ok {
			problems = append(problems, "category not allowed")
		}
	}

	if fields.Latitude != nil {
		if lat := *fields.Latitude; lat < -90 || lat > 90 {
			problems = append(problems, "latitude must be between -90 and 90")
		}
	}
	if fields.Longitude != nil {
		if lng := *fields.Longitude; lng < -180 || lng > 180 {
			problems = append(problems, "longitude must be between -180 and 180")
		}
	}

	validateTime := func(label string, raw *string) {
		if raw == nil || *raw == "" {
			return
		}
		if _, err := time.Parse("15:04", *raw); err != nil {
			problems = append(problems, fmt.Sprintf("%s must be in HH:MM (24h) format", label))
		}
	}

	validateTime("opening_time", fields.OpeningTime)
	validateTime("closing_time", fields.ClosingTime)

	if fields.Gallery != nil {
		gallery := *fields.Gallery
		for idx := range gallery {
			gallery[idx].URL = strings.TrimSpace(gallery[idx].URL)
			if gallery[idx].Caption != nil {
				if trimmed := strings.TrimSpace(*gallery[idx].Caption); trimmed == "" {
					gallery[idx].Caption = nil
				} else {
					gallery[idx].Caption = &trimmed
				}
			}
			if gallery[idx].URL == "" {
				problems = append(problems, fmt.Sprintf("gallery[%d] url is required", idx))
			}
			if gallery[idx].Ordering < 0 {
				problems = append(problems, fmt.Sprintf("gallery[%d] ordering must be non-negative", idx))
			}
		}
	}

	if action == domain.DestinationChangeActionCreate && fields.Status != nil {
		if *fields.Status != domain.DestinationStatusDraft && *fields.Status != domain.DestinationStatusPublished {
			problems = append(problems, "create status must be draft or published")
		}
	}

	if action == domain.DestinationChangeActionUpdate && fields.Status != nil {
		switch *fields.Status {
		case domain.DestinationStatusDraft, domain.DestinationStatusPublished, domain.DestinationStatusArchived:
		default:
			problems = append(problems, "status must be draft, published, or archived")
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrDestinationChangeValidation, strings.Join(problems, "; "))
	}
	return nil
}

func (s *DestinationWorkflowService) ValidateFields(action domain.DestinationChangeAction, fields domain.DestinationChangeFields, requireAll bool) error {
	return s.validateFields(action, fields, requireAll)
}

func (s *DestinationWorkflowService) applyCreate(ctx context.Context, change *domain.DestinationChangeRequest, reviewerID uuid.UUID) (*domain.Destination, error) {
	status := domain.DestinationStatusPublished
	if change.Payload.Status != nil {
		status = *change.Payload.Status
	}
	heroURL := change.Payload.HeroImageURL
	dest, err := s.destinations.Create(ctx, change.Payload, reviewerID, status, heroURL)
	if err != nil {
		return nil, err
	}
	if _, err = s.versions.Create(ctx, &domain.DestinationVersion{
		ID:              uuid.Nil,
		DestinationID:   dest.ID,
		ChangeRequestID: &change.ID,
		Version:         dest.Version,
		Snapshot:        snapshotFromDestination(dest),
		CreatedAt:       s.now(),
		CreatedBy:       reviewerID,
	}); err != nil {
		return nil, err
	}
	return dest, nil
}

func (s *DestinationWorkflowService) applyUpdate(ctx context.Context, change *domain.DestinationChangeRequest, reviewerID uuid.UUID) (*domain.Destination, error) {
	if change.DestinationID == nil {
		return nil, fmt.Errorf("%w: destination_id required for update", ErrDestinationChangeValidation)
	}
	dest, err := s.destinations.FindByID(ctx, *change.DestinationID)
	if err != nil {
		return nil, ErrDestinationNotFound
	}
	statusOverride := change.Payload.Status
	heroURL := change.Payload.HeroImageURL
	dest, err = s.destinations.Update(ctx, dest.ID, change.Payload, reviewerID, statusOverride, heroURL)
	if err != nil {
		return nil, err
	}
	if _, err = s.versions.Create(ctx, &domain.DestinationVersion{
		ID:              uuid.Nil,
		DestinationID:   dest.ID,
		ChangeRequestID: &change.ID,
		Version:         dest.Version,
		Snapshot:        snapshotFromDestination(dest),
		CreatedAt:       s.now(),
		CreatedBy:       reviewerID,
	}); err != nil {
		return nil, err
	}
	return dest, nil
}

func (s *DestinationWorkflowService) applyDelete(ctx context.Context, change *domain.DestinationChangeRequest, reviewerID uuid.UUID) (*domain.Destination, error) {
	if change.DestinationID == nil {
		return nil, fmt.Errorf("%w: destination_id required for delete", ErrDestinationChangeValidation)
	}
	dest, err := s.destinations.FindByID(ctx, *change.DestinationID)
	if err != nil {
		return nil, ErrDestinationNotFound
	}

	snapshot := snapshotFromDestination(dest)
	hardDelete := false
	if change.Payload.HardDelete != nil {
		hardDelete = *change.Payload.HardDelete
	}
	if hardDelete && !s.hardDeleteAllowed {
		return nil, ErrHardDeleteNotAllowed
	}

	var updated *domain.Destination
	if hardDelete {
		if err := s.destinations.HardDelete(ctx, dest.ID); err != nil {
			return nil, err
		}
	} else {
		if updated, err = s.destinations.Archive(ctx, dest.ID, reviewerID); err != nil {
			return nil, err
		}
	}

	recordSnapshot := snapshot
	if updated != nil {
		recordSnapshot = snapshotFromDestination(updated)
	}

	if _, err = s.versions.Create(ctx, &domain.DestinationVersion{
		ID:              uuid.Nil,
		DestinationID:   dest.ID,
		ChangeRequestID: &change.ID,
		Version:         recordSnapshot.Version,
		Snapshot:        recordSnapshot,
		CreatedAt:       s.now(),
		CreatedBy:       reviewerID,
	}); err != nil {
		return updated, err
	}
	return updated, nil
}

func (s *DestinationWorkflowService) isDeleteAction(action domain.DestinationChangeAction) bool {
	return action == domain.DestinationChangeActionDelete
}

func snapshotFromDestination(dest *domain.Destination) domain.DestinationSnapshot {
	if dest == nil {
		return domain.DestinationSnapshot{}
	}
	return domain.DestinationSnapshot{
		ID:          dest.ID,
		Name:        dest.Name,
		Slug:        dest.Slug,
		Status:      dest.Status,
		Version:     dest.Version,
		City:        dest.City,
		Country:     dest.Country,
		Category:    dest.Category,
		Description: dest.Description,
		Latitude:    dest.Latitude,
		Longitude:   dest.Longitude,
		Contact:     dest.Contact,
		OpeningTime: dest.OpeningTime,
		ClosingTime: dest.ClosingTime,
		Gallery: func() domain.DestinationGallery {
			if len(dest.Gallery) == 0 {
				return nil
			}
			return append(domain.DestinationGallery(nil), dest.Gallery...)
		}(),
		HeroImage: dest.HeroImage,
		UpdatedAt: dest.UpdatedAt,
		UpdatedBy: dest.UpdatedBy,
	}
}

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func cloneGallery(src domain.DestinationGallery) domain.DestinationGallery {
	if src == nil {
		return nil
	}
	if len(src) == 0 {
		return domain.DestinationGallery{}
	}
	out := make(domain.DestinationGallery, len(src))
	for i := range src {
		out[i] = domain.DestinationMedia{
			URL:      src[i].URL,
			Ordering: src[i].Ordering,
		}
		if src[i].Caption != nil {
			caption := *src[i].Caption
			out[i].Caption = &caption
		}
	}
	return out
}
