package service

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

func TestDestinationWorkflowService_Flows(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 7, 10, 12, 0, 0, 0, time.UTC)

	destRepo := newMemoryDestinationRepo(now)
	changeRepo := newMemoryChangeRepo()
	versionRepo := newMemoryVersionRepo()
	storage := &memoryStorage{}

	service := NewDestinationWorkflowService(destRepo, changeRepo, versionRepo, storage, DestinationWorkflowConfig{
		Bucket:            "fitcity-destinations",
		PublicBaseURL:     "https://cdn.fitcity.local/fitcity-destinations",
		ImageMaxBytes:     5 * 1024 * 1024,
		AllowedCategories: []string{"Nature", "Adventure", "City", "Food"},
		ApprovalRequired:  true,
		HardDeleteAllowed: false,
	})
	service.SetClock(func() time.Time { return now })

	admin := uuid.New()
	reviewer := uuid.New()

	t.Run("create destination approve success", func(t *testing.T) {
		change, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action: domain.DestinationChangeActionCreate,
			Fields: domain.DestinationChangeFields{
				Name:        strPtr("Central Park"),
				Category:    strPtr("Nature"),
				Description: strPtr("Iconic park"),
			},
		})
		if err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		if _, err = service.SubmitDraft(ctx, change.ID, admin); err != nil {
			t.Fatalf("SubmitDraft: %v", err)
		}
		approved, dest, err := service.Approve(ctx, change.ID, reviewer)
		if err != nil {
			t.Fatalf("Approve: %v", err)
		}
		if approved.Status != domain.DestinationChangeStatusApproved {
			t.Fatalf("expected approved status, got %s", approved.Status)
		}
		if dest == nil || dest.Status != domain.DestinationStatusPublished {
			t.Fatalf("expected published destination")
		}
	})

	t.Run("create destination reject", func(t *testing.T) {
		change, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action: domain.DestinationChangeActionCreate,
			Fields: domain.DestinationChangeFields{
				Name:        strPtr("Orphan Draft"),
				Description: strPtr("Should be rejected"),
				Category:    strPtr("City"),
			},
		})
		if err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		if _, err = service.SubmitDraft(ctx, change.ID, admin); err != nil {
			t.Fatalf("SubmitDraft: %v", err)
		}
		rejected, err := service.Reject(ctx, change.ID, reviewer, "Not needed")
		if err != nil {
			t.Fatalf("Reject: %v", err)
		}
		if rejected.Status != domain.DestinationChangeStatusRejected {
			t.Fatalf("expected rejected status, got %s", rejected.Status)
		}
	})

	t.Run("update published destination approve success", func(t *testing.T) {
		existing := destRepo.mustCreate(ctx, domain.DestinationChangeFields{
			Name:        strPtr("City Museum"),
			Description: strPtr("Original"),
			Category:    strPtr("City"),
		}, admin, domain.DestinationStatusPublished, nil)

		change, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action:        domain.DestinationChangeActionUpdate,
			DestinationID: &existing.ID,
			Fields: domain.DestinationChangeFields{
				Description: strPtr("Updated description"),
			},
		})
		if err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		if _, err = service.SubmitDraft(ctx, change.ID, admin); err != nil {
			t.Fatalf("SubmitDraft: %v", err)
		}
		approved, dest, err := service.Approve(ctx, change.ID, reviewer)
		if err != nil {
			t.Fatalf("Approve: %v", err)
		}
		if approved.PublishedVersion == nil || *approved.PublishedVersion != dest.Version {
			t.Fatalf("expected published version to match destination version")
		}
		if dest.Description == nil || *dest.Description != "Updated description" {
			t.Fatalf("destination description not updated")
		}
	})

	t.Run("update published destination reject", func(t *testing.T) {
		existing := destRepo.mustCreate(ctx, domain.DestinationChangeFields{
			Name:        strPtr("Harbor Pier"),
			Description: strPtr("Existing content"),
			Category:    strPtr("Adventure"),
		}, admin, domain.DestinationStatusPublished, nil)

		change, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action:        domain.DestinationChangeActionUpdate,
			DestinationID: &existing.ID,
			Fields: domain.DestinationChangeFields{
				Description: strPtr("Rejected update"),
			},
		})
		if err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		if _, err = service.SubmitDraft(ctx, change.ID, admin); err != nil {
			t.Fatalf("SubmitDraft: %v", err)
		}
		rejected, err := service.Reject(ctx, change.ID, reviewer, "Prefer original")
		if err != nil {
			t.Fatalf("Reject: %v", err)
		}
		if rejected.Status != domain.DestinationChangeStatusRejected {
			t.Fatalf("expected rejected status")
		}
		persisted, _ := destRepo.FindByID(ctx, existing.ID)
		if persisted.Description == nil || *persisted.Description != "Existing content" {
			t.Fatalf("destination should remain unchanged on rejection")
		}
	})

	t.Run("update drafted destination approve success", func(t *testing.T) {
		existing := destRepo.mustCreate(ctx, domain.DestinationChangeFields{
			Name:        strPtr("New River Walk"),
			Description: strPtr("Draft content"),
			Category:    strPtr("Nature"),
		}, admin, domain.DestinationStatusDraft, nil)

		targetStatus := domain.DestinationStatusPublished
		change, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action:        domain.DestinationChangeActionUpdate,
			DestinationID: &existing.ID,
			Fields: domain.DestinationChangeFields{
				Description: strPtr("Ready to publish"),
				Status:      &targetStatus,
			},
		})
		if err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		if _, err = service.SubmitDraft(ctx, change.ID, admin); err != nil {
			t.Fatalf("SubmitDraft: %v", err)
		}
		_, dest, err := service.Approve(ctx, change.ID, reviewer)
		if err != nil {
			t.Fatalf("Approve: %v", err)
		}
		if dest.Status != domain.DestinationStatusPublished {
			t.Fatalf("expected destination to be published, got %s", dest.Status)
		}
	})

	t.Run("update drafted destination reject", func(t *testing.T) {
		existing := destRepo.mustCreate(ctx, domain.DestinationChangeFields{
			Name:        strPtr("Future Gallery"),
			Description: strPtr("Draft info"),
			Category:    strPtr("City"),
		}, admin, domain.DestinationStatusDraft, nil)

		change, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action:        domain.DestinationChangeActionUpdate,
			DestinationID: &existing.ID,
			Fields: domain.DestinationChangeFields{
				Description: strPtr("Needs correction"),
			},
		})
		if err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		if _, err = service.SubmitDraft(ctx, change.ID, admin); err != nil {
			t.Fatalf("SubmitDraft: %v", err)
		}
		rejected, err := service.Reject(ctx, change.ID, reviewer, "Fix details")
		if err != nil {
			t.Fatalf("Reject: %v", err)
		}
		if rejected.Status != domain.DestinationChangeStatusRejected {
			t.Fatalf("expected rejection")
		}
	})

	t.Run("delete published destination approve success", func(t *testing.T) {
		existing := destRepo.mustCreate(ctx, domain.DestinationChangeFields{
			Name:        strPtr("Old Stadium"),
			Description: strPtr("Closing down"),
			Category:    strPtr("City"),
		}, admin, domain.DestinationStatusPublished, nil)

		change, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action:        domain.DestinationChangeActionDelete,
			DestinationID: &existing.ID,
			Fields:        domain.DestinationChangeFields{},
		})
		if err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		if _, err = service.SubmitDraft(ctx, change.ID, admin); err != nil {
			t.Fatalf("SubmitDraft: %v", err)
		}
		_, dest, err := service.Approve(ctx, change.ID, reviewer)
		if err != nil {
			t.Fatalf("Approve: %v", err)
		}
		if dest == nil || dest.Status != domain.DestinationStatusArchived {
			t.Fatalf("expected archived destination after delete approval")
		}
	})

	t.Run("delete published destination reject", func(t *testing.T) {
		existing := destRepo.mustCreate(ctx, domain.DestinationChangeFields{
			Name:        strPtr("Historic Library"),
			Description: strPtr("Important landmark"),
			Category:    strPtr("City"),
		}, admin, domain.DestinationStatusPublished, nil)

		change, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action:        domain.DestinationChangeActionDelete,
			DestinationID: &existing.ID,
			Fields:        domain.DestinationChangeFields{},
		})
		if err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		if _, err = service.SubmitDraft(ctx, change.ID, admin); err != nil {
			t.Fatalf("SubmitDraft: %v", err)
		}
		if _, err = service.Reject(ctx, change.ID, reviewer, "Keep open"); err != nil {
			t.Fatalf("Reject: %v", err)
		}
		persisted, _ := destRepo.FindByID(ctx, existing.ID)
		if persisted.Status != domain.DestinationStatusPublished {
			t.Fatalf("destination should remain published after rejection")
		}
	})

	t.Run("delete drafted destination approve success", func(t *testing.T) {
		existing := destRepo.mustCreate(ctx, domain.DestinationChangeFields{
			Name:        strPtr("Prototype Trail"),
			Description: strPtr("Draft location"),
			Category:    strPtr("Adventure"),
		}, admin, domain.DestinationStatusDraft, nil)

		change, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action:        domain.DestinationChangeActionDelete,
			DestinationID: &existing.ID,
			Fields:        domain.DestinationChangeFields{},
		})
		if err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		if _, err = service.SubmitDraft(ctx, change.ID, admin); err != nil {
			t.Fatalf("SubmitDraft: %v", err)
		}
		_, dest, err := service.Approve(ctx, change.ID, reviewer)
		if err != nil {
			t.Fatalf("Approve: %v", err)
		}
		if dest == nil || dest.Status != domain.DestinationStatusArchived {
			t.Fatalf("drafted destination should be archived on delete approval")
		}
	})

	t.Run("delete drafted destination reject", func(t *testing.T) {
		existing := destRepo.mustCreate(ctx, domain.DestinationChangeFields{
			Name:        strPtr("Seasonal Market"),
			Description: strPtr("Draft info"),
			Category:    strPtr("Food"),
		}, admin, domain.DestinationStatusDraft, nil)

		change, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action:        domain.DestinationChangeActionDelete,
			DestinationID: &existing.ID,
			Fields:        domain.DestinationChangeFields{},
		})
		if err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		if _, err = service.SubmitDraft(ctx, change.ID, admin); err != nil {
			t.Fatalf("SubmitDraft: %v", err)
		}
		if _, err = service.Reject(ctx, change.ID, reviewer, "Keep as draft"); err != nil {
			t.Fatalf("Reject: %v", err)
		}
		persisted, _ := destRepo.FindByID(ctx, existing.ID)
		if persisted.Status != domain.DestinationStatusDraft {
			t.Fatalf("draft destination should remain draft after rejection")
		}
	})

	t.Run("create destination persists contact hours and gallery", func(t *testing.T) {
		gallery := domain.DestinationGallery{
			{
				URL:      "https://cdn.fitcity.local/destinations/gallery-1.jpg",
				Ordering: 1,
			},
			{
				URL:      "https://cdn.fitcity.local/destinations/gallery-2.jpg",
				Ordering: 2,
				Caption:  strPtr("Evening skyline"),
			},
		}
		contact := strPtr("+1-555-123-4567")
		opening := strPtr("08:00")
		closing := strPtr("21:30")

		change, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action: domain.DestinationChangeActionCreate,
			Fields: domain.DestinationChangeFields{
				Name:        strPtr("Gallery Gardens"),
				Category:    strPtr("Nature"),
				Description: strPtr("Features vibrant exhibits"),
				Contact:     contact,
				OpeningTime: opening,
				ClosingTime: closing,
				Gallery:     &gallery,
			},
		})
		if err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		if _, err = service.SubmitDraft(ctx, change.ID, admin); err != nil {
			t.Fatalf("SubmitDraft: %v", err)
		}
		_, dest, err := service.Approve(ctx, change.ID, reviewer)
		if err != nil {
			t.Fatalf("Approve: %v", err)
		}
		if dest.Contact == nil || *dest.Contact != *contact {
			t.Fatalf("expected contact to persist")
		}
		if dest.OpeningTime == nil || *dest.OpeningTime != *opening {
			t.Fatalf("expected opening time to persist")
		}
		if dest.ClosingTime == nil || *dest.ClosingTime != *closing {
			t.Fatalf("expected closing time to persist")
		}
		if len(dest.Gallery) != len(gallery) {
			t.Fatalf("expected %d gallery items, got %d", len(gallery), len(dest.Gallery))
		}
		for i := range gallery {
			if dest.Gallery[i].URL != gallery[i].URL {
				t.Fatalf("gallery url mismatch at index %d", i)
			}
			if gallery[i].Caption == nil {
				if dest.Gallery[i].Caption != nil {
					t.Fatalf("expected nil caption at index %d", i)
				}
			} else if dest.Gallery[i].Caption == nil || *dest.Gallery[i].Caption != *gallery[i].Caption {
				t.Fatalf("gallery caption mismatch at index %d", i)
			}
			if dest.Gallery[i].Ordering != gallery[i].Ordering {
				t.Fatalf("gallery ordering mismatch at index %d", i)
			}
		}
	})

	t.Run("create draft rejects invalid opening hours", func(t *testing.T) {
		_, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action: domain.DestinationChangeActionCreate,
			Fields: domain.DestinationChangeFields{
				Name:        strPtr("Late Night Market"),
				Category:    strPtr("Food"),
				Description: strPtr("Nightly specials"),
				OpeningTime: strPtr("25:00"),
			},
		})
		if err == nil {
			t.Fatalf("expected validation error for invalid opening time")
		}
		if !errors.Is(err, ErrDestinationChangeValidation) {
			t.Fatalf("expected ErrDestinationChangeValidation, got %v", err)
		}
	})

	t.Run("create draft rejects gallery item without url", func(t *testing.T) {
		gallery := domain.DestinationGallery{
			{
				URL:      " ",
				Ordering: 1,
			},
		}
		_, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action: domain.DestinationChangeActionCreate,
			Fields: domain.DestinationChangeFields{
				Name:     strPtr("Gallery Missing URL"),
				Category: strPtr("Nature"),
				Gallery:  &gallery,
			},
		})
		if err == nil {
			t.Fatalf("expected validation error for gallery without url")
		}
		if !errors.Is(err, ErrDestinationChangeValidation) {
			t.Fatalf("expected ErrDestinationChangeValidation, got %v", err)
		}
	})

	t.Run("upload gallery images appends media", func(t *testing.T) {
		change, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action: domain.DestinationChangeActionCreate,
			Fields: domain.DestinationChangeFields{
				Name:     strPtr("Gallery Draft"),
				Category: strPtr("Nature"),
				Gallery: &domain.DestinationGallery{
					{URL: "https://cdn.fitcity.local/existing.jpg", Ordering: 0},
				},
			},
		})
		if err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		uploads := []GalleryImageUpload{
			{
				Reader:      bytes.NewReader([]byte("image-one")),
				Size:        int64(len([]byte("image-one"))),
				FileName:    "one.jpg",
				ContentType: "image/jpeg",
			},
			{
				Reader:      bytes.NewReader([]byte("image-two")),
				Size:        int64(len([]byte("image-two"))),
				FileName:    "two.png",
				ContentType: "image/png",
			},
		}
		updated, results, err := service.UploadGalleryImages(ctx, change.ID, admin, uploads)
		if err != nil {
			t.Fatalf("UploadGalleryImages: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 upload results, got %d", len(results))
		}
		if updated.Payload.Gallery == nil || len(*updated.Payload.Gallery) != 3 {
			t.Fatalf("expected gallery length 3, got %d", len(*updated.Payload.Gallery))
		}
		if (*updated.Payload.Gallery)[2].Ordering != 2 {
			t.Fatalf("expected last ordering 2, got %d", (*updated.Payload.Gallery)[2].Ordering)
		}
	})

	t.Run("upload gallery images rejects unauthorized user", func(t *testing.T) {
		change, err := service.CreateDraft(ctx, admin, DestinationDraftInput{
			Action: domain.DestinationChangeActionCreate,
			Fields: domain.DestinationChangeFields{
				Name:     strPtr("Unauthorized Gallery"),
				Category: strPtr("Nature"),
			},
		})
		if err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		_, _, err = service.UploadGalleryImages(ctx, change.ID, uuid.New(), []GalleryImageUpload{
			{
				Reader:      bytes.NewReader([]byte("image-one")),
				Size:        int64(len([]byte("image-one"))),
				FileName:    "one.jpg",
				ContentType: "image/jpeg",
			},
		})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})
}

// --- memory repositories for testing ---

type memoryDestinationRepo struct {
	mu    sync.Mutex
	store map[uuid.UUID]*domain.Destination
	now   time.Time
}

func newMemoryDestinationRepo(now time.Time) *memoryDestinationRepo {
	return newMemoryDestinationRepoWithClock(now)
}

func newMemoryDestinationRepoWithClock(now time.Time) *memoryDestinationRepo {
	return &memoryDestinationRepo{
		store: make(map[uuid.UUID]*domain.Destination),
		now:   now,
	}
}

func (m *memoryDestinationRepo) Create(ctx context.Context, fields domain.DestinationChangeFields, createdBy uuid.UUID, status domain.DestinationStatus, heroImageURL *string) (*domain.Destination, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New()
	now := m.now
	dest := &domain.Destination{
		ID:          id,
		Name:        deref(fields.Name),
		Slug:        copyStringPtr(fields.Slug),
		Status:      status,
		Version:     1,
		City:        copyStringPtr(fields.City),
		Country:     copyStringPtr(fields.Country),
		Category:    copyStringPtr(fields.Category),
		Description: copyStringPtr(fields.Description),
		Latitude:    copyFloatPtr(fields.Latitude),
		Longitude:   copyFloatPtr(fields.Longitude),
		Contact:     copyStringPtr(fields.Contact),
		OpeningTime: copyStringPtr(fields.OpeningTime),
		ClosingTime: copyStringPtr(fields.ClosingTime),
		Gallery:     copyGalleryValue(fields.Gallery),
		HeroImage:   copyStringPtr(heroImageURL),
		CreatedAt:   now,
		UpdatedAt:   now,
		UpdatedBy:   copyUUIDPtr(&createdBy),
	}
	m.store[id] = cloneDestination(dest)
	return cloneDestination(dest), nil
}

func (m *memoryDestinationRepo) Update(ctx context.Context, id uuid.UUID, fields domain.DestinationChangeFields, updatedBy uuid.UUID, statusOverride *domain.DestinationStatus, heroImageURL *string) (*domain.Destination, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	dest, ok := m.store[id]
	if !ok {
		return nil, sql.ErrNoRows
	}

	if fields.Name != nil {
		dest.Name = *fields.Name
	}
	if fields.Slug != nil {
		dest.Slug = copyStringPtr(fields.Slug)
	}
	if fields.City != nil {
		dest.City = copyStringPtr(fields.City)
	}
	if fields.Country != nil {
		dest.Country = copyStringPtr(fields.Country)
	}
	if fields.Category != nil {
		dest.Category = copyStringPtr(fields.Category)
	}
	if fields.Description != nil {
		dest.Description = copyStringPtr(fields.Description)
	}
	if fields.Contact != nil {
		dest.Contact = copyStringPtr(fields.Contact)
	}
	if fields.OpeningTime != nil {
		dest.OpeningTime = copyStringPtr(fields.OpeningTime)
	}
	if fields.ClosingTime != nil {
		dest.ClosingTime = copyStringPtr(fields.ClosingTime)
	}
	if fields.Latitude != nil {
		dest.Latitude = copyFloatPtr(fields.Latitude)
	}
	if fields.Longitude != nil {
		dest.Longitude = copyFloatPtr(fields.Longitude)
	}
	if heroImageURL != nil {
		dest.HeroImage = copyStringPtr(heroImageURL)
	}
	if fields.Gallery != nil {
		dest.Gallery = copyGalleryValue(fields.Gallery)
	}
	if statusOverride != nil {
		dest.Status = *statusOverride
	}
	dest.Version++
	now := m.now
	dest.UpdatedAt = now
	dest.UpdatedBy = copyUUIDPtr(&updatedBy)

	m.store[id] = cloneDestination(dest)
	return cloneDestination(dest), nil
}

func (m *memoryDestinationRepo) Archive(ctx context.Context, id uuid.UUID, updatedBy uuid.UUID) (*domain.Destination, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	dest, ok := m.store[id]
	if !ok {
		return nil, sql.ErrNoRows
	}

	now := m.now
	dest.Status = domain.DestinationStatusArchived
	dest.DeletedAt = &now
	dest.Version++
	dest.UpdatedAt = now
	dest.UpdatedBy = copyUUIDPtr(&updatedBy)

	m.store[id] = cloneDestination(dest)
	return cloneDestination(dest), nil
}

func (m *memoryDestinationRepo) HardDelete(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store[id]; !ok {
		return sql.ErrNoRows
	}
	delete(m.store, id)
	return nil
}

func (m *memoryDestinationRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Destination, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	dest, ok := m.store[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return cloneDestination(dest), nil
}

func (m *memoryDestinationRepo) FindPublishedByID(ctx context.Context, id uuid.UUID) (*domain.Destination, error) {
	dest, err := m.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if dest.Status != domain.DestinationStatusPublished || dest.DeletedAt != nil {
		return nil, sql.ErrNoRows
	}
	return dest, nil
}

func (m *memoryDestinationRepo) FindBySlug(ctx context.Context, slug string) (*domain.Destination, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, dest := range m.store {
		if dest.Slug != nil && *dest.Slug == slug && dest.DeletedAt == nil {
			return cloneDestination(dest), nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *memoryDestinationRepo) ListPublished(ctx context.Context, limit, offset int, filter domain.DestinationListFilter) ([]domain.Destination, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	published := make([]domain.Destination, 0)
	needle := strings.ToLower(strings.TrimSpace(filter.Search))

	categorySet := map[string]struct{}{}
	for _, category := range filter.Categories {
		if trimmed := strings.ToLower(strings.TrimSpace(category)); trimmed != "" {
			categorySet[trimmed] = struct{}{}
		}
	}

	for _, dest := range m.store {
		if dest.Status != domain.DestinationStatusPublished || dest.DeletedAt != nil {
			continue
		}
		if needle != "" && !destinationMatchesQuery(dest, needle) {
			continue
		}
		if len(categorySet) > 0 {
			if dest.Category == nil {
				continue
			}
			if _, ok := categorySet[strings.ToLower(strings.TrimSpace(*dest.Category))]; !ok {
				continue
			}
		}
		if filter.MinRating != nil && dest.AverageRating < *filter.MinRating {
			continue
		}
		if filter.MaxRating != nil && dest.AverageRating > *filter.MaxRating {
			continue
		}
		published = append(published, *cloneDestination(dest))
	}

	sort.SliceStable(published, func(i, j int) bool {
		switch filter.Sort {
		case domain.DestinationSortRatingAsc:
			if published[i].AverageRating == published[j].AverageRating {
				return strings.ToLower(published[i].Name) < strings.ToLower(published[j].Name)
			}
			return published[i].AverageRating < published[j].AverageRating
		case domain.DestinationSortRatingDesc:
			if published[i].AverageRating == published[j].AverageRating {
				return strings.ToLower(published[i].Name) < strings.ToLower(published[j].Name)
			}
			return published[i].AverageRating > published[j].AverageRating
		case domain.DestinationSortNameAsc:
			return strings.ToLower(published[i].Name) < strings.ToLower(published[j].Name)
		case domain.DestinationSortNameDesc:
			return strings.ToLower(published[i].Name) > strings.ToLower(published[j].Name)
		default:
			if published[i].UpdatedAt.Equal(published[j].UpdatedAt) {
				return strings.ToLower(published[i].Name) < strings.ToLower(published[j].Name)
			}
			return published[i].UpdatedAt.After(published[j].UpdatedAt)
		}
	})

	if offset >= len(published) {
		return []domain.Destination{}, nil
	}
	end := len(published)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return published[offset:end], nil
}

func destinationMatchesQuery(dest *domain.Destination, needle string) bool {
	if dest == nil {
		return false
	}
	fields := []string{
		dest.Name,
	}
	if dest.Slug != nil {
		fields = append(fields, *dest.Slug)
	}
	if dest.City != nil {
		fields = append(fields, *dest.City)
	}
	if dest.Country != nil {
		fields = append(fields, *dest.Country)
	}
	if dest.Category != nil {
		fields = append(fields, *dest.Category)
	}
	if dest.Description != nil {
		fields = append(fields, *dest.Description)
	}
	if dest.Contact != nil {
		fields = append(fields, *dest.Contact)
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

func (m *memoryDestinationRepo) mustCreate(ctx context.Context, fields domain.DestinationChangeFields, createdBy uuid.UUID, status domain.DestinationStatus, heroImageURL *string) *domain.Destination {
	dest, err := m.Create(ctx, fields, createdBy, status, heroImageURL)
	if err != nil {
		panic(err)
	}
	return dest
}

type memoryChangeRepo struct {
	mu    sync.Mutex
	store map[uuid.UUID]*domain.DestinationChangeRequest
}

func newMemoryChangeRepo() *memoryChangeRepo {
	return &memoryChangeRepo{store: make(map[uuid.UUID]*domain.DestinationChangeRequest)}
}

func (m *memoryChangeRepo) Create(ctx context.Context, change *domain.DestinationChangeRequest) (*domain.DestinationChangeRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	change.ID = id
	m.store[id] = cloneChange(change)
	return cloneChange(change), nil
}

func (m *memoryChangeRepo) Update(ctx context.Context, change *domain.DestinationChangeRequest) (*domain.DestinationChangeRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.store[change.ID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	prev := change.DraftVersion
	if prev > 0 {
		prev--
	}
	if prev > 0 && existing.DraftVersion != prev {
		return nil, sql.ErrNoRows
	}
	m.store[change.ID] = cloneChange(change)
	return cloneChange(change), nil
}

func (m *memoryChangeRepo) MarkSubmitted(ctx context.Context, id uuid.UUID, submittedAt time.Time) (*domain.DestinationChangeRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	change, ok := m.store[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	if change.Status != domain.DestinationChangeStatusDraft && change.Status != domain.DestinationChangeStatusRejected {
		return nil, sql.ErrNoRows
	}
	change.Status = domain.DestinationChangeStatusPendingReview
	change.SubmittedAt = &submittedAt
	change.UpdatedAt = submittedAt
	m.store[id] = cloneChange(change)
	return cloneChange(change), nil
}

func (m *memoryChangeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.DestinationChangeRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	change, ok := m.store[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return cloneChange(change), nil
}

func (m *memoryChangeRepo) List(ctx context.Context, filter domain.DestinationChangeFilter) ([]domain.DestinationChangeRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.DestinationChangeRequest, 0)
	for _, change := range m.store {
		if filter.DestinationID != nil && (change.DestinationID == nil || *change.DestinationID != *filter.DestinationID) {
			continue
		}
		if filter.SubmittedBy != nil && change.SubmittedBy != *filter.SubmittedBy {
			continue
		}
		if len(filter.Statuses) > 0 {
			match := false
			for _, status := range filter.Statuses {
				if change.Status == status {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		out = append(out, *cloneChange(change))
	}
	return out, nil
}

func (m *memoryChangeRepo) SetStatus(ctx context.Context, id uuid.UUID, status domain.DestinationChangeStatus, reviewerID *uuid.UUID, reviewMessage *string, publishedVersion *int64) (*domain.DestinationChangeRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	change, ok := m.store[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	change.Status = status
	change.ReviewedBy = copyUUIDPtr(reviewerID)
	change.ReviewMessage = copyStringPtr(reviewMessage)
	now := time.Now().UTC()
	change.ReviewedAt = &now
	change.PublishedVersion = publishedVersion
	change.UpdatedAt = now
	m.store[id] = cloneChange(change)
	return cloneChange(change), nil
}

func (m *memoryChangeRepo) DeleteByID(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store[id]; !ok {
		return sql.ErrNoRows
	}
	delete(m.store, id)
	return nil
}

type memoryVersionRepo struct {
	mu      sync.Mutex
	records map[uuid.UUID][]domain.DestinationVersion
}

func newMemoryVersionRepo() *memoryVersionRepo {
	return &memoryVersionRepo{records: make(map[uuid.UUID][]domain.DestinationVersion)}
}

func (m *memoryVersionRepo) Create(ctx context.Context, version *domain.DestinationVersion) (*domain.DestinationVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := *version
	rec.ID = uuid.New()
	m.records[version.DestinationID] = append(m.records[version.DestinationID], rec)
	return &rec, nil
}

func (m *memoryVersionRepo) ListByDestination(ctx context.Context, destinationID uuid.UUID, limit int) ([]domain.DestinationVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.records[destinationID]
	if len(out) > limit && limit > 0 {
		return out[:limit], nil
	}
	return append([]domain.DestinationVersion(nil), out...), nil
}

type memoryStorage struct {
	objects sync.Map
}

func (m *memoryStorage) Upload(ctx context.Context, bucket, objectName, contentType string, reader io.Reader, size int64) (string, error) {
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, reader); err != nil {
		return "", err
	}
	m.objects.Store(objectName, buf.Bytes())
	return "https://cdn.local/" + objectName, nil
}

// helpers

func cloneDestination(src *domain.Destination) *domain.Destination {
	if src == nil {
		return nil
	}
	dest := *src
	dest.Slug = copyStringPtr(src.Slug)
	dest.City = copyStringPtr(src.City)
	dest.Country = copyStringPtr(src.Country)
	dest.Category = copyStringPtr(src.Category)
	dest.Description = copyStringPtr(src.Description)
	dest.Latitude = copyFloatPtr(src.Latitude)
	dest.Longitude = copyFloatPtr(src.Longitude)
	dest.Contact = copyStringPtr(src.Contact)
	dest.OpeningTime = copyStringPtr(src.OpeningTime)
	dest.ClosingTime = copyStringPtr(src.ClosingTime)
	dest.Gallery = cloneGalleryTest(src.Gallery)
	dest.HeroImage = copyStringPtr(src.HeroImage)
	dest.UpdatedBy = copyUUIDPtr(src.UpdatedBy)
	dest.DeletedAt = copyTimePtr(src.DeletedAt)
	return &dest
}

func cloneGalleryTest(src domain.DestinationGallery) domain.DestinationGallery {
	if src == nil {
		return nil
	}
	if len(src) == 0 {
		return domain.DestinationGallery{}
	}
	out := make(domain.DestinationGallery, len(src))
	for i, media := range src {
		out[i] = domain.DestinationMedia{
			URL:      media.URL,
			Ordering: media.Ordering,
		}
		if media.Caption != nil {
			caption := *media.Caption
			out[i].Caption = &caption
		}
	}
	return out
}

func copyGalleryValue(src *domain.DestinationGallery) domain.DestinationGallery {
	if src == nil {
		return nil
	}
	return cloneGalleryTest(*src)
}

func copyGalleryPtr(src *domain.DestinationGallery) *domain.DestinationGallery {
	if src == nil {
		return nil
	}
	cloned := cloneGalleryTest(*src)
	return &cloned
}

func cloneChange(src *domain.DestinationChangeRequest) *domain.DestinationChangeRequest {
	if src == nil {
		return nil
	}
	change := *src
	change.Payload = copyChangeFields(src.Payload)
	change.DestinationID = copyUUIDPtr(src.DestinationID)
	change.HeroImageTempKey = copyStringPtr(src.HeroImageTempKey)
	change.ReviewedBy = copyUUIDPtr(src.ReviewedBy)
	change.ReviewMessage = copyStringPtr(src.ReviewMessage)
	change.SubmittedAt = copyTimePtr(src.SubmittedAt)
	change.ReviewedAt = copyTimePtr(src.ReviewedAt)
	return &change
}

func strPtr(v string) *string {
	return &v
}

func copyStringPtr(src *string) *string {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func copyUUIDPtr(src *uuid.UUID) *uuid.UUID {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func copyFloatPtr(src *float64) *float64 {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func copyTimePtr(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func deref(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func copyChangeFields(src domain.DestinationChangeFields) domain.DestinationChangeFields {
	return domain.DestinationChangeFields{
		Name:               copyStringPtr(src.Name),
		Slug:               copyStringPtr(src.Slug),
		City:               copyStringPtr(src.City),
		Country:            copyStringPtr(src.Country),
		Category:           copyStringPtr(src.Category),
		Description:        copyStringPtr(src.Description),
		Contact:            copyStringPtr(src.Contact),
		OpeningTime:        copyStringPtr(src.OpeningTime),
		ClosingTime:        copyStringPtr(src.ClosingTime),
		Gallery:            copyGalleryPtr(src.Gallery),
		Latitude:           copyFloatPtr(src.Latitude),
		Longitude:          copyFloatPtr(src.Longitude),
		Status:             copyStatusPtr(src.Status),
		HeroImageUploadID:  copyStringPtr(src.HeroImageUploadID),
		HeroImageURL:       copyStringPtr(src.HeroImageURL),
		PublishedHeroImage: copyStringPtr(src.PublishedHeroImage),
		HardDelete:         copyBoolPtr(src.HardDelete),
	}
}

func copyStatusPtr(src *domain.DestinationStatus) *domain.DestinationStatus {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func copyBoolPtr(src *bool) *bool {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}
