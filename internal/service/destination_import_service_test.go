package service

import (
	"context"
	"database/sql"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

func TestDestinationImportService_ImportCreatesPendingChanges(t *testing.T) {
	repo := newMemoryImportRepo()
	destRepo := &stubDestinationLookup{slugs: map[string]bool{}}
	workflow := &stubWorkflow{}
	storage := &noopStorage{}
	cfg := DestinationImportServiceConfig{
		Bucket:        "fitcity-destinations",
		MaxRows:       10,
		MaxFileBytes:  1024 * 1024,
		MaxPendingIDs: 5,
	}

	svc := NewDestinationImportService(repo, destRepo, workflow, storage, cfg)
	fixedTime := time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixedTime }

	csvData := strings.Join([]string{
		"slug,name,status,category,city,country,description,latitude,longitude,contact,opening_time,closing_time,hero_image_url",
		"central-park,Central Park,published,Nature,New York,USA,Iconic park,40.785091,-73.968285,+1 212-310-6600,06:00,22:00,https://cdn/hero.jpg",
		"city-museum,City Museum,published,Museum,St. Louis,USA,Art museum,38.633,-90.200,+1 314-231-2489,09:00,17:00,https://cdn/museum.jpg",
	}, "\n")

	job, rows, err := svc.Import(context.Background(), uuid.New(), "import.csv", []byte(csvData), false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Status != domain.DestinationImportStatusCompleted {
		t.Fatalf("expected completed status, got %s", job.Status)
	}
	if job.ChangesCreated != 2 {
		t.Fatalf("expected 2 changes created, got %d", job.ChangesCreated)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	for _, row := range rows {
		if row.Status != domain.DestinationImportRowStatusPendingReview {
			t.Fatalf("row %d expected pending review, got %s", row.RowNumber, row.Status)
		}
		if row.ChangeID == nil {
			t.Fatalf("row %d missing change id", row.RowNumber)
		}
	}
	if len(job.PendingIDs) != 2 {
		t.Fatalf("expected pending IDs, got %d", len(job.PendingIDs))
	}
	if len(workflow.created) != 2 {
		t.Fatalf("expected workflow create calls")
	}
}

func TestDestinationImportService_DryRunSkipsPersistence(t *testing.T) {
	repo := newMemoryImportRepo()
	destRepo := &stubDestinationLookup{slugs: map[string]bool{}}
	workflow := &stubWorkflow{}
	svc := NewDestinationImportService(repo, destRepo, workflow, &noopStorage{}, DestinationImportServiceConfig{
		Bucket:        "",
		MaxRows:       10,
		MaxFileBytes:  1024 * 1024,
		MaxPendingIDs: 5,
	})

	csvData := "slug,name,status,category,city,country,description,latitude,longitude,contact,hero_image_url\n" +
		"dry-run,Example,published,Nature,City,Country,Desc,10.1,20.2,+123,https://cdn/example.jpg"

	job, rows, err := svc.Import(context.Background(), uuid.New(), "dry.csv", []byte(csvData), true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.ChangesCreated != 0 {
		t.Fatalf("expected 0 changes, got %d", job.ChangesCreated)
	}
	if len(rows) != 1 || rows[0].Status != domain.DestinationImportRowStatusSkipped {
		t.Fatalf("row should be skipped in dry run: %#v", rows)
	}
	if len(workflow.created) != 0 {
		t.Fatalf("workflow should not create drafts during dry run")
	}
}

func TestDestinationImportService_DetectsDuplicateSlugs(t *testing.T) {
	repo := newMemoryImportRepo()
	destRepo := &stubDestinationLookup{slugs: map[string]bool{"central-park": true}}
	workflow := &stubWorkflow{}
	svc := NewDestinationImportService(repo, destRepo, workflow, &noopStorage{}, DestinationImportServiceConfig{
		Bucket:        "",
		MaxRows:       10,
		MaxFileBytes:  1024 * 1024,
		MaxPendingIDs: 5,
	})

	csvData := "slug,name,status,category,city,country,description,latitude,longitude,contact,hero_image_url\n" +
		"central-park,Central Park,published,Nature,NYC,USA,Desc,40,-73,+1,https://cdn/hero.jpg"

	job, rows, err := svc.Import(context.Background(), uuid.New(), "dup.csv", []byte(csvData), false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.RowsFailed != 1 {
		t.Fatalf("expected failure count 1, got %d", job.RowsFailed)
	}
	if rows[0].Status != domain.DestinationImportRowStatusFailed {
		t.Fatalf("row should be failed")
	}
	if rows[0].ErrorMessage == nil || !strings.Contains(*rows[0].ErrorMessage, "slug already exists") {
		t.Fatalf("expected slug error, got %v", rows[0].ErrorMessage)
	}
}

type stubWorkflow struct {
	created []*domain.DestinationChangeRequest
}

func (s *stubWorkflow) CreateDraft(ctx context.Context, authorID uuid.UUID, input DestinationDraftInput) (*domain.DestinationChangeRequest, error) {
	req := &domain.DestinationChangeRequest{
		ID:          uuid.New(),
		SubmittedBy: authorID,
		Payload:     input.Fields,
	}
	s.created = append(s.created, req)
	return req, nil
}

func (s *stubWorkflow) SubmitDraft(ctx context.Context, changeID uuid.UUID, authorID uuid.UUID) (*domain.DestinationChangeRequest, error) {
	return &domain.DestinationChangeRequest{
		ID:     changeID,
		Status: domain.DestinationChangeStatusPendingReview,
	}, nil
}

func (s *stubWorkflow) ValidateFields(action domain.DestinationChangeAction, fields domain.DestinationChangeFields, requireAll bool) error {
	return nil
}

type stubDestinationLookup struct {
	slugs map[string]bool
}

func (s *stubDestinationLookup) FindBySlug(ctx context.Context, slug string) (*domain.Destination, error) {
	if s.slugs[strings.ToLower(strings.TrimSpace(slug))] {
		return &domain.Destination{ID: uuid.New()}, nil
	}
	return nil, sql.ErrNoRows
}

type noopStorage struct{}

func (n *noopStorage) Upload(ctx context.Context, bucket, objectName, contentType string, reader io.Reader, size int64) (string, error) {
	return objectName, nil
}

type memoryImportRepo struct {
	job  *domain.DestinationImportJob
	rows []domain.DestinationImportRow
}

func newMemoryImportRepo() *memoryImportRepo {
	return &memoryImportRepo{
		rows: make([]domain.DestinationImportRow, 0),
	}
}

func (m *memoryImportRepo) CreateJob(ctx context.Context, job *domain.DestinationImportJob) (*domain.DestinationImportJob, error) {
	clone := *job
	if clone.ID == uuid.Nil {
		clone.ID = uuid.New()
	}
	now := time.Now()
	clone.CreatedAt = now
	clone.UpdatedAt = now
	m.job = &clone
	return m.job, nil
}

func (m *memoryImportRepo) UpdateJob(ctx context.Context, job *domain.DestinationImportJob) (*domain.DestinationImportJob, error) {
	if m.job == nil {
		return nil, sql.ErrNoRows
	}
	copy := *job
	copy.UpdatedAt = time.Now()
	m.job = &copy
	return m.job, nil
}

func (m *memoryImportRepo) FindJobByID(ctx context.Context, id uuid.UUID) (*domain.DestinationImportJob, error) {
	if m.job == nil || m.job.ID != id {
		return nil, sql.ErrNoRows
	}
	copy := *m.job
	return &copy, nil
}

func (m *memoryImportRepo) InsertRow(ctx context.Context, row *domain.DestinationImportRow) (*domain.DestinationImportRow, error) {
	inserted := *row
	inserted.ID = uuid.New()
	inserted.CreatedAt = time.Now()
	m.rows = append(m.rows, inserted)
	return &inserted, nil
}

func (m *memoryImportRepo) ListRowsByJob(ctx context.Context, jobID uuid.UUID) ([]domain.DestinationImportRow, error) {
	out := make([]domain.DestinationImportRow, len(m.rows))
	copy(out, m.rows)
	return out, nil
}
