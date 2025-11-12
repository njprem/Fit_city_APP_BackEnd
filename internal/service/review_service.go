package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/media"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/ports"
)

var (
	ErrReviewValidation   = errors.New("review validation failed")
	ErrReviewAlreadyExist = errors.New("review already exists for this destination")
	ErrReviewNotFound     = errors.New("review not found")
	ErrReviewForbidden    = errors.New("not allowed to manage this review")
)

type ReviewServiceConfig struct {
	Bucket            string
	MaxImages         int
	MaxImageBytes     int64
	AllowedMIMETypes  []string
	ImageProcessor    media.Processor
	ImageMaxDimension int
	PublicBaseURL     string
}

type ReviewImageUpload struct {
	Reader      io.Reader
	Size        int64
	FileName    string
	ContentType string
	Ordering    int
}

type ReviewCreateInput struct {
	Rating  int
	Title   *string
	Content *string
	Images  []ReviewImageUpload
}

type ReviewService struct {
	reviews      ports.ReviewRepository
	media        ports.ReviewMediaRepository
	destinations ports.DestinationRepository
	storage      ports.ObjectStorage

	bucket            string
	publicBase        string
	maxImages         int
	maxImageBytes     int64
	allowedMIMEs      map[string]struct{}
	now               func() time.Time
	imageProcessor    media.Processor
	imageMaxDimension int
}

const (
	defaultMaxReviewImages = 5
	defaultMaxImageBytes   = int64(5 * 1024 * 1024)
)

var defaultAllowedMIMEs = []string{
	"image/jpeg",
	"image/png",
	"image/webp",
}

func NewReviewService(
	reviews ports.ReviewRepository,
	mediaRepo ports.ReviewMediaRepository,
	destinations ports.DestinationRepository,
	storage ports.ObjectStorage,
	cfg ReviewServiceConfig,
) *ReviewService {
	maxImages := cfg.MaxImages
	if maxImages <= 0 {
		maxImages = defaultMaxReviewImages
	}
	maxBytes := cfg.MaxImageBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxImageBytes
	}
	allowedMIMEs := cfg.AllowedMIMETypes
	if len(allowedMIMEs) == 0 {
		allowedMIMEs = defaultAllowedMIMEs
	}
	mimeSet := make(map[string]struct{}, len(allowedMIMEs))
	for _, mt := range allowedMIMEs {
		mimeSet[strings.ToLower(strings.TrimSpace(mt))] = struct{}{}
	}

	maxDimension := cfg.ImageMaxDimension
	if maxDimension <= 0 {
		maxDimension = media.DefaultMaxDimension
	}
	publicBase := strings.TrimRight(cfg.PublicBaseURL, "/")

	return &ReviewService{
		reviews:           reviews,
		media:             mediaRepo,
		destinations:      destinations,
		storage:           storage,
		bucket:            strings.TrimSpace(cfg.Bucket),
		publicBase:        publicBase,
		maxImages:         maxImages,
		maxImageBytes:     maxBytes,
		allowedMIMEs:      mimeSet,
		now:               time.Now,
		imageProcessor:    cfg.ImageProcessor,
		imageMaxDimension: maxDimension,
	}
}

func (s *ReviewService) CreateReview(ctx context.Context, userID, destinationID uuid.UUID, input ReviewCreateInput) (*domain.Review, *domain.ReviewAggregate, error) {
	title := normalizeString(input.Title)
	content := normalizeString(input.Content)

	if err := validateRating(input.Rating); err != nil {
		return nil, nil, err
	}
	if content != nil && title == nil {
		return nil, nil, fmt.Errorf("%w: content requires title", ErrReviewValidation)
	}
	if err := s.validateImages(input.Images); err != nil {
		return nil, nil, err
	}
	if err := s.ensureDestinationExists(ctx, destinationID); err != nil {
		return nil, nil, err
	}

	reviewToCreate := &domain.Review{
		DestinationID: destinationID,
		UserID:        userID,
		Rating:        input.Rating,
		Title:         title,
		Content:       content,
	}

	stored, err := s.reviews.Create(ctx, reviewToCreate)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, nil, ErrReviewAlreadyExist
		}
		return nil, nil, err
	}

	if len(input.Images) > 0 {
		records, uploadErr := s.uploadMedia(ctx, destinationID, stored.ID, input.Images)
		if uploadErr != nil {
			_ = s.reviews.SoftDelete(ctx, stored.ID, userID)
			return nil, nil, uploadErr
		}
		if err := s.media.CreateMany(ctx, records); err != nil {
			_ = s.reviews.SoftDelete(ctx, stored.ID, userID)
			return nil, nil, err
		}
	}

	review, err := s.reviews.GetByID(ctx, stored.ID)
	if err != nil {
		return nil, nil, err
	}

	mediaMap, err := s.media.ListByReviewIDs(ctx, []uuid.UUID{review.ID})
	if err != nil {
		return nil, nil, err
	}
	review.Media = mediaMap[review.ID]

	aggregate, err := s.reviews.AggregateByDestination(ctx, destinationID, domain.ReviewAggregateFilter{})
	if err != nil {
		return nil, nil, err
	}

	return review, aggregate, nil
}

func (s *ReviewService) ListDestinationReviews(ctx context.Context, destinationID uuid.UUID, filter domain.ReviewListFilter) (*domain.ReviewListResult, error) {
	if err := s.ensureDestinationExists(ctx, destinationID); err != nil {
		return nil, err
	}

	normalized, err := s.normalizeListFilter(filter)
	if err != nil {
		return nil, err
	}

	reviews, err := s.reviews.ListByDestination(ctx, destinationID, normalized)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(reviews))
	for _, review := range reviews {
		ids = append(ids, review.ID)
	}
	if len(ids) > 0 {
		mediaMap, mediaErr := s.media.ListByReviewIDs(ctx, ids)
		if mediaErr != nil {
			return nil, mediaErr
		}
		for i := range reviews {
			reviews[i].Media = mediaMap[reviews[i].ID]
		}
	}

	aggregate, err := s.reviews.AggregateByDestination(ctx, destinationID, toAggregateFilter(normalized))
	if err != nil {
		return nil, err
	}

	return &domain.ReviewListResult{
		DestinationID: destinationID,
		Reviews:       reviews,
		Aggregate:     *aggregate,
		Limit:         normalized.Limit,
		Offset:        normalized.Offset,
	}, nil
}

func (s *ReviewService) DeleteReview(ctx context.Context, reviewID, requesterID uuid.UUID, isAdmin bool) error {
	review, err := s.reviews.GetByID(ctx, reviewID)
	if err != nil {
		if isNotFound(err) {
			return ErrReviewNotFound
		}
		return err
	}
	if review.DeletedAt != nil {
		return ErrReviewNotFound
	}
	if review.UserID != requesterID && !isAdmin {
		return ErrReviewForbidden
	}
	if err := s.reviews.SoftDelete(ctx, reviewID, requesterID); err != nil {
		if isNotFound(err) {
			return ErrReviewNotFound
		}
		return err
	}
	return nil
}

func (s *ReviewService) validateImages(images []ReviewImageUpload) error {
	if len(images) == 0 {
		return nil
	}
	if len(images) > s.maxImages {
		return fmt.Errorf("%w: maximum %d images allowed", ErrReviewValidation, s.maxImages)
	}
	for idx, image := range images {
		if image.Size <= 0 {
			return fmt.Errorf("%w: image %d is empty", ErrReviewValidation, idx+1)
		}
		if s.maxImageBytes > 0 && image.Size > s.maxImageBytes {
			return fmt.Errorf("%w: image %d exceeds size limit (%d bytes)", ErrReviewValidation, idx+1, s.maxImageBytes)
		}
		contentType := strings.ToLower(strings.TrimSpace(image.ContentType))
		if _, ok := s.allowedMIMEs[contentType]; !ok {
			return fmt.Errorf("%w: image %d has unsupported content type %s", ErrReviewValidation, idx+1, image.ContentType)
		}
	}
	return nil
}

func (s *ReviewService) ensureDestinationExists(ctx context.Context, destinationID uuid.UUID) error {
	if destinationID == uuid.Nil {
		return ErrDestinationNotFound
	}
	if _, err := s.destinations.FindPublishedByID(ctx, destinationID); err != nil {
		if isNotFound(err) {
			return ErrDestinationNotFound
		}
		return err
	}
	return nil
}

func (s *ReviewService) normalizeListFilter(filter domain.ReviewListFilter) (domain.ReviewListFilter, error) {
	result := filter
	if result.Limit <= 0 {
		result.Limit = 20
	}
	if result.Limit > 100 {
		result.Limit = 100
	}
	if result.Offset < 0 {
		result.Offset = 0
	}

	if result.SortField != domain.ReviewSortRating && result.SortField != domain.ReviewSortCreatedAt {
		result.SortField = domain.ReviewSortCreatedAt
	}
	if result.SortOrder != domain.SortOrderAsc {
		result.SortOrder = domain.SortOrderDesc
	}

	if result.Rating != nil {
		if err := validateRating(*result.Rating); err != nil {
			return domain.ReviewListFilter{}, err
		}
	}
	if result.MinRating != nil {
		if err := validateRating(*result.MinRating); err != nil {
			return domain.ReviewListFilter{}, err
		}
	}
	if result.MaxRating != nil {
		if err := validateRating(*result.MaxRating); err != nil {
			return domain.ReviewListFilter{}, err
		}
	}
	if result.MinRating != nil && result.MaxRating != nil && *result.MinRating > *result.MaxRating {
		return domain.ReviewListFilter{}, fmt.Errorf("%w: min_rating cannot be greater than max_rating", ErrReviewValidation)
	}
	return result, nil
}

func (s *ReviewService) uploadMedia(ctx context.Context, destinationID, reviewID uuid.UUID, images []ReviewImageUpload) ([]domain.ReviewMedia, error) {
	now := s.now()
	records := make([]domain.ReviewMedia, 0, len(images))

	for idx, image := range images {
		ordering := image.Ordering
		if ordering < 0 {
			ordering = 0
		}
		if ordering == 0 {
			ordering = idx
		}
		ext := safeImageExtension(image.ContentType, image.FileName)
		objectKey := fmt.Sprintf("reviews/%s/%s/%s_%d%s", destinationID.String(), reviewID.String(), now.UTC().Format("20060102T150405Z0700"), idx, ext)

		reader, size, contentType, err := prepareImageForUpload(ctx, s.imageProcessor, media.Upload{
			Reader:      image.Reader,
			Size:        image.Size,
			FileName:    image.FileName,
			ContentType: image.ContentType,
		}, s.imageMaxDimension)
		if err != nil {
			return nil, err
		}

		url, err := s.storage.Upload(ctx, s.bucket, objectKey, contentType, reader, size)
		if err != nil {
			return nil, err
		}
		if s.publicBase != "" {
			url = strings.TrimRight(s.publicBase, "/") + "/" + strings.TrimLeft(objectKey, "/")
		}

		records = append(records, domain.ReviewMedia{
			ReviewID:  reviewID,
			ObjectKey: objectKey,
			URL:       url,
			Ordering:  ordering,
		})
	}
	return records, nil
}

func validateRating(rating int) error {
	if rating < 0 || rating > 5 {
		return fmt.Errorf("%w: rating must be between 0 and 5", ErrReviewValidation)
	}
	return nil
}

func normalizeString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func safeImageExtension(contentType, fileName string) string {
	ext := extensionFromContentType(strings.ToLower(strings.TrimSpace(contentType)))
	if ext != "" {
		return ext
	}
	if fileName != "" {
		if nameExt := strings.ToLower(strings.TrimSpace(filepath.Ext(fileName))); nameExt != "" {
			return nameExt
		}
	}
	return ".bin"
}

func toAggregateFilter(filter domain.ReviewListFilter) domain.ReviewAggregateFilter {
	return domain.ReviewAggregateFilter{
		Rating:       filter.Rating,
		MinRating:    filter.MinRating,
		MaxRating:    filter.MaxRating,
		PostedAfter:  filter.PostedAfter,
		PostedBefore: filter.PostedBefore,
	}
}
