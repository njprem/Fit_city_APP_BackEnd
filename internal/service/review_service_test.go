package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/ports"
)

func TestReviewService_CreateReview_AllowsNewAfterSoftDelete(t *testing.T) {
	ctx := context.Background()

	destID := uuid.New()
	userID := uuid.New()

	repo := newMemoryReviewRepository()
	mediaRepo := newMemoryMediaRepository()
	destRepo := &reviewDestinationRepo{
		items: map[uuid.UUID]*domain.Destination{
			destID: {ID: destID, Status: domain.DestinationStatusPublished},
		},
	}
	storage := &reviewStorage{}

	svc := NewReviewService(repo, mediaRepo, destRepo, storage, ReviewServiceConfig{
		Bucket: "fitcity-reviews",
	})

	// First review succeeds.
	review, agg, err := svc.CreateReview(ctx, userID, destID, ReviewCreateInput{Rating: 4})
	if err != nil {
		t.Fatalf("CreateReview first call returned error: %v", err)
	}
	if review.Rating != 4 {
		t.Fatalf("expected rating 4, got %d", review.Rating)
	}
	if agg.TotalReviews != 1 {
		t.Fatalf("expected total reviews 1, got %d", agg.TotalReviews)
	}

	// Delete the review (soft delete).
	if err := svc.DeleteReview(ctx, review.ID, userID, false); err != nil {
		t.Fatalf("DeleteReview returned error: %v", err)
	}

	// New review after soft delete should succeed.
	review2, agg2, err := svc.CreateReview(ctx, userID, destID, ReviewCreateInput{Rating: 5})
	if err != nil {
		t.Fatalf("CreateReview second call returned error: %v", err)
	}
	if review2.Rating != 5 {
		t.Fatalf("expected rating 5, got %d", review2.Rating)
	}
	if agg2.TotalReviews != 1 {
		t.Fatalf("expected total reviews 1 after recreation, got %d", agg2.TotalReviews)
	}

	// Ensure the soft-deleted review remains marked as deleted.
	if !repo.isDeleted(review.ID) {
		t.Fatalf("expected first review to be soft deleted")
	}
}

func TestReviewService_CreateReview_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	destID := uuid.New()
	userID := uuid.New()

	repo := newMemoryReviewRepository()
	destRepo := &reviewDestinationRepo{
		items: map[uuid.UUID]*domain.Destination{
			destID: {ID: destID, Status: domain.DestinationStatusPublished},
		},
	}
	svc := NewReviewService(repo, newMemoryMediaRepository(), destRepo, &reviewStorage{}, ReviewServiceConfig{})

	_, _, err := svc.CreateReview(ctx, userID, destID, ReviewCreateInput{Rating: -1})
	if !errors.Is(err, ErrReviewValidation) {
		t.Fatalf("expected ErrReviewValidation for invalid rating, got %v", err)
	}

	content := "Nice place"
	_, _, err = svc.CreateReview(ctx, userID, destID, ReviewCreateInput{Rating: 3, Content: &content})
	if !errors.Is(err, ErrReviewValidation) {
		t.Fatalf("expected ErrReviewValidation when content provided without title, got %v", err)
	}

	title := "Trip"
	badImage := ReviewImageUpload{
		Reader:      bytes.NewReader([]byte("gifdata")),
		Size:        int64(len("gifdata")),
		FileName:    "bad.gif",
		ContentType: "image/gif",
	}
	_, _, err = svc.CreateReview(ctx, userID, destID, ReviewCreateInput{
		Rating: 4,
		Title:  &title,
		Images: []ReviewImageUpload{badImage},
	})
	if !errors.Is(err, ErrReviewValidation) {
		t.Fatalf("expected ErrReviewValidation for unsupported image type, got %v", err)
	}
}

func TestReviewService_CreateReviewProcessesImages(t *testing.T) {
	ctx := context.Background()
	destID := uuid.New()
	userID := uuid.New()

	repo := newMemoryReviewRepository()
	mediaRepo := newMemoryMediaRepository()
	destRepo := &reviewDestinationRepo{
		items: map[uuid.UUID]*domain.Destination{
			destID: {ID: destID, Status: domain.DestinationStatusPublished},
		},
	}
	storage := &reviewStorage{}
	processor := &stubImageProcessor{output: []byte("processed-review-image")}

	svc := NewReviewService(repo, mediaRepo, destRepo, storage, ReviewServiceConfig{
		Bucket:         "fitcity-reviews",
		ImageProcessor: processor,
	})

	title := "Trip"
	image := ReviewImageUpload{
		Reader:      bytes.NewReader([]byte("original-review-image")),
		Size:        int64(len("original-review-image")),
		FileName:    "review.jpg",
		ContentType: "image/jpeg",
	}

	if _, _, err := svc.CreateReview(ctx, userID, destID, ReviewCreateInput{Rating: 5, Title: &title, Images: []ReviewImageUpload{image}}); err != nil {
		t.Fatalf("CreateReview returned error: %v", err)
	}
	if processor.calls != 1 {
		t.Fatalf("expected processor to be invoked once, got %d", processor.calls)
	}
	if storage.uploads != 1 {
		t.Fatalf("expected one upload, got %d", storage.uploads)
	}
	if string(storage.lastData) != "processed-review-image" {
		t.Fatalf("expected processed bytes to be uploaded, got %q", storage.lastData)
	}
}

func TestReviewService_CreateReview_UniqueViolation(t *testing.T) {
	ctx := context.Background()
	destID := uuid.New()
	userID := uuid.New()

	repo := newMemoryReviewRepository()
	destRepo := &reviewDestinationRepo{
		items: map[uuid.UUID]*domain.Destination{
			destID: {ID: destID, Status: domain.DestinationStatusPublished},
		},
	}
	svc := NewReviewService(repo, newMemoryMediaRepository(), destRepo, &reviewStorage{}, ReviewServiceConfig{})

	if _, _, err := svc.CreateReview(ctx, userID, destID, ReviewCreateInput{Rating: 4}); err != nil {
		t.Fatalf("unexpected error in first CreateReview: %v", err)
	}

	if _, _, err := svc.CreateReview(ctx, userID, destID, ReviewCreateInput{Rating: 5}); !errors.Is(err, ErrReviewAlreadyExist) {
		t.Fatalf("expected ErrReviewAlreadyExist for duplicate active review, got %v", err)
	}
}

func TestReviewService_ListDestinationReviews(t *testing.T) {
	ctx := context.Background()
	destID := uuid.New()
	otherDestID := uuid.New()
	userID := uuid.New()

	repo := newMemoryReviewRepository()
	destRepo := &reviewDestinationRepo{
		items: map[uuid.UUID]*domain.Destination{
			destID:      {ID: destID, Status: domain.DestinationStatusPublished},
			otherDestID: {ID: otherDestID, Status: domain.DestinationStatusPublished},
		},
	}
	svc := NewReviewService(repo, newMemoryMediaRepository(), destRepo, &reviewStorage{}, ReviewServiceConfig{})

	if _, _, err := svc.CreateReview(ctx, userID, destID, ReviewCreateInput{Rating: 5}); err != nil {
		t.Fatalf("unexpected error creating review: %v", err)
	}
	if _, _, err := svc.CreateReview(ctx, userID, otherDestID, ReviewCreateInput{Rating: 3}); err != nil {
		t.Fatalf("unexpected error creating other review: %v", err)
	}

	result, err := svc.ListDestinationReviews(ctx, destID, domain.ReviewListFilter{})
	if err != nil {
		t.Fatalf("ListDestinationReviews returned error: %v", err)
	}
	if len(result.Reviews) != 1 {
		t.Fatalf("expected 1 review, got %d", len(result.Reviews))
	}
	if result.Aggregate.TotalReviews != 1 || result.Aggregate.AverageRating != 5 {
		t.Fatalf("unexpected aggregate: %+v", result.Aggregate)
	}
}

func TestReviewService_DeleteReview_Permissions(t *testing.T) {
	ctx := context.Background()
	destID := uuid.New()
	userID := uuid.New()
	otherUser := uuid.New()

	repo := newMemoryReviewRepository()
	destRepo := &reviewDestinationRepo{
		items: map[uuid.UUID]*domain.Destination{
			destID: {ID: destID, Status: domain.DestinationStatusPublished},
		},
	}
	svc := NewReviewService(repo, newMemoryMediaRepository(), destRepo, &reviewStorage{}, ReviewServiceConfig{})

	review, _, err := svc.CreateReview(ctx, userID, destID, ReviewCreateInput{Rating: 2})
	if err != nil {
		t.Fatalf("unexpected error creating review: %v", err)
	}

	if err := svc.DeleteReview(ctx, review.ID, otherUser, false); !errors.Is(err, ErrReviewForbidden) {
		t.Fatalf("expected ErrReviewForbidden, got %v", err)
	}

	if err := svc.DeleteReview(ctx, review.ID, userID, false); err != nil {
		t.Fatalf("DeleteReview by owner returned error: %v", err)
	}
	if !repo.isDeleted(review.ID) {
		t.Fatalf("review should be soft deleted")
	}
}

// --- Test doubles ---

type memoryReviewRepository struct {
	reviews map[uuid.UUID]*domain.Review
}

func newMemoryReviewRepository() *memoryReviewRepository {
	return &memoryReviewRepository{
		reviews: make(map[uuid.UUID]*domain.Review),
	}
}

func (m *memoryReviewRepository) Create(_ context.Context, review *domain.Review) (*domain.Review, error) {
	for _, existing := range m.reviews {
		if existing.UserID == review.UserID && existing.DestinationID == review.DestinationID && existing.DeletedAt == nil {
			return nil, &pgconn.PgError{Code: "23505"}
		}
	}

	now := time.Now().UTC()
	cloned := *review
	cloned.ID = uuid.New()
	cloned.CreatedAt = now
	cloned.UpdatedAt = now
	m.reviews[cloned.ID] = &cloned
	return &cloned, nil
}

func (m *memoryReviewRepository) GetByID(_ context.Context, id uuid.UUID) (*domain.Review, error) {
	review, ok := m.reviews[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cloned := *review
	return &cloned, nil
}

func (m *memoryReviewRepository) ListByDestination(_ context.Context, destID uuid.UUID, filter domain.ReviewListFilter) ([]domain.Review, error) {
	var items []domain.Review
	for _, review := range m.reviews {
		if review.DestinationID != destID || review.DeletedAt != nil {
			continue
		}
		if filter.Rating != nil && review.Rating != *filter.Rating {
			continue
		}
		if filter.MinRating != nil && review.Rating < *filter.MinRating {
			continue
		}
		if filter.MaxRating != nil && review.Rating > *filter.MaxRating {
			continue
		}
		items = append(items, *review)
	}

	sort.Slice(items, func(i, j int) bool {
		switch filter.SortField {
		case domain.ReviewSortRating:
			if filter.SortOrder == domain.SortOrderAsc {
				return items[i].Rating < items[j].Rating
			}
			return items[i].Rating > items[j].Rating
		default:
			if filter.SortOrder == domain.SortOrderAsc {
				return items[i].CreatedAt.Before(items[j].CreatedAt)
			}
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
	})

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return append([]domain.Review(nil), items[offset:end]...), nil
}

func (m *memoryReviewRepository) AggregateByDestination(_ context.Context, destID uuid.UUID, filter domain.ReviewAggregateFilter) (*domain.ReviewAggregate, error) {
	counts := map[int]int{0: 0, 1: 0, 2: 0, 3: 0, 4: 0, 5: 0}
	total := 0
	sum := 0

	for _, review := range m.reviews {
		if review.DestinationID != destID || review.DeletedAt != nil {
			continue
		}
		if filter.Rating != nil && review.Rating != *filter.Rating {
			continue
		}
		if filter.MinRating != nil && review.Rating < *filter.MinRating {
			continue
		}
		if filter.MaxRating != nil && review.Rating > *filter.MaxRating {
			continue
		}
		total++
		counts[review.Rating]++
		sum += review.Rating
	}

	average := 0.0
	if total > 0 {
		average = float64(sum) / float64(total)
	}

	return &domain.ReviewAggregate{
		DestinationID: destID,
		AverageRating: average,
		TotalReviews:  total,
		RatingCounts:  counts,
	}, nil
}

func (m *memoryReviewRepository) SoftDelete(_ context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	review, ok := m.reviews[id]
	if !ok {
		return errors.New("not found")
	}
	now := time.Now().UTC()
	review.DeletedAt = &now
	review.DeletedBy = &deletedBy
	review.UpdatedAt = now
	return nil
}

func (m *memoryReviewRepository) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.reviews, id)
	return nil
}

func (m *memoryReviewRepository) isDeleted(id uuid.UUID) bool {
	review, ok := m.reviews[id]
	if !ok {
		return false
	}
	return review.DeletedAt != nil
}

type memoryMediaRepository struct {
	items map[uuid.UUID][]domain.ReviewMedia
}

func newMemoryMediaRepository() *memoryMediaRepository {
	return &memoryMediaRepository{
		items: make(map[uuid.UUID][]domain.ReviewMedia),
	}
}

func (m *memoryMediaRepository) CreateMany(_ context.Context, media []domain.ReviewMedia) error {
	for _, item := range media {
		m.items[item.ReviewID] = append(m.items[item.ReviewID], item)
	}
	return nil
}

func (m *memoryMediaRepository) ListByReviewIDs(_ context.Context, reviewIDs []uuid.UUID) (map[uuid.UUID][]domain.ReviewMedia, error) {
	result := make(map[uuid.UUID][]domain.ReviewMedia)
	for _, id := range reviewIDs {
		if media, ok := m.items[id]; ok {
			result[id] = append([]domain.ReviewMedia(nil), media...)
		}
	}
	return result, nil
}

type reviewStorage struct {
	lastObject string
	lastData   []byte
	uploads    int
}

func (s *reviewStorage) Upload(_ context.Context, _ string, objectName, _ string, reader io.Reader, _ int64) (string, error) {
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, reader); err != nil {
		return "", err
	}
	s.lastObject = objectName
	s.lastData = buf.Bytes()
	s.uploads++
	return "https://example.com/" + objectName, nil
}

type reviewDestinationRepo struct {
	items map[uuid.UUID]*domain.Destination
}

func (m *reviewDestinationRepo) Create(context.Context, domain.DestinationChangeFields, uuid.UUID, domain.DestinationStatus, *string) (*domain.Destination, error) {
	return nil, errors.New("not implemented")
}

func (m *reviewDestinationRepo) Update(context.Context, uuid.UUID, domain.DestinationChangeFields, uuid.UUID, *domain.DestinationStatus, *string) (*domain.Destination, error) {
	return nil, errors.New("not implemented")
}

func (m *reviewDestinationRepo) Archive(context.Context, uuid.UUID, uuid.UUID) (*domain.Destination, error) {
	return nil, errors.New("not implemented")
}

func (m *reviewDestinationRepo) HardDelete(context.Context, uuid.UUID) error {
	return errors.New("not implemented")
}

func (m *reviewDestinationRepo) FindByID(context.Context, uuid.UUID) (*domain.Destination, error) {
	return nil, errors.New("not implemented")
}

func (m *reviewDestinationRepo) FindPublishedByID(_ context.Context, id uuid.UUID) (*domain.Destination, error) {
	dest, ok := m.items[id]
	if !ok {
		return nil, errors.New("not found")
	}
	if dest.Status != domain.DestinationStatusPublished {
		return nil, errors.New("not published")
	}
	cloned := *dest
	return &cloned, nil
}

func (m *reviewDestinationRepo) FindBySlug(context.Context, string) (*domain.Destination, error) {
	return nil, errors.New("not implemented")
}

func (m *reviewDestinationRepo) ListPublished(context.Context, int, int, domain.DestinationListFilter) ([]domain.Destination, error) {
	return nil, errors.New("not implemented")
}

var _ ports.ReviewRepository = (*memoryReviewRepository)(nil)
var _ ports.ReviewMediaRepository = (*memoryMediaRepository)(nil)
var _ ports.DestinationRepository = (*reviewDestinationRepo)(nil)
var _ ports.ObjectStorage = (*reviewStorage)(nil)
