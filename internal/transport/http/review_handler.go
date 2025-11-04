package http

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/service"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/util"
)

type ReviewHandler struct {
	auth    *service.AuthService
	reviews *service.ReviewService
}

type ReviewMediaResponse struct {
	ID       uuid.UUID `json:"id"`
	URL      string    `json:"url"`
	Ordering int       `json:"ordering"`
}

type ReviewAuthorResponse struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Username    *string   `json:"username,omitempty"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
}

type ReviewResponse struct {
	ID            uuid.UUID             `json:"id"`
	DestinationID uuid.UUID             `json:"destination_id"`
	Rating        int                   `json:"rating"`
	Title         *string               `json:"title,omitempty"`
	Content       *string               `json:"content,omitempty"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
	Reviewer      ReviewAuthorResponse  `json:"reviewer"`
	Media         []ReviewMediaResponse `json:"media,omitempty"`
}

type ReviewAggregateResponse struct {
	AverageRating float64        `json:"average_rating"`
	TotalReviews  int            `json:"total_reviews"`
	RatingCounts  map[string]int `json:"rating_counts"`
}

type ReviewCreateResponse struct {
	Review    ReviewResponse          `json:"review"`
	Aggregate ReviewAggregateResponse `json:"aggregate"`
}

type ReviewListResponse struct {
	DestinationID uuid.UUID        `json:"destination_id"`
	AverageRating float64          `json:"average_rating"`
	TotalReviews  int              `json:"total_reviews"`
	RatingCounts  map[string]int   `json:"rating_counts"`
	Reviews       []ReviewResponse `json:"reviews"`
	Limit         int              `json:"limit"`
	Offset        int              `json:"offset"`
}

func RegisterReviews(e *echo.Echo, auth *service.AuthService, reviews *service.ReviewService) {
	handler := &ReviewHandler{
		auth:    auth,
		reviews: reviews,
	}

	public := e.Group("/api/v1/destinations/:destination_id/reviews")
	public.GET("", handler.listReviews)

	protected := e.Group("/api/v1/destinations/:destination_id/reviews", RequireAuth(auth))
	protected.POST("", handler.createReview)

	deleter := e.Group("/api/v1/reviews", RequireAuth(auth))
	deleter.DELETE("/:id", handler.deleteReview)
}

// createReview handles POST /api/v1/destinations/{destination_id}/reviews
func (h *ReviewHandler) createReview(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, util.Error("authentication required"))
	}

	destID, err := uuid.Parse(c.Param("destination_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid destination id"))
	}

	if err := c.Request().ParseMultipartForm(32 << 20); err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid multipart payload"))
	}

	ratingStr := strings.TrimSpace(c.FormValue("rating"))
	if ratingStr == "" {
		return c.JSON(http.StatusBadRequest, util.Error("rating required"))
	}
	rating, err := strconv.Atoi(ratingStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("rating must be an integer"))
	}

	title := optionalString(c.FormValue("title"))
	content := optionalString(c.FormValue("content"))

	form := c.Request().MultipartForm
	uploads, closeFns, err := buildReviewUploads(form)
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
	}
	defer func() {
		for _, closer := range closeFns {
			_ = closer.Close()
		}
	}()

	review, aggregate, err := h.reviews.CreateReview(c.Request().Context(), user.ID, destID, service.ReviewCreateInput{
		Rating:  rating,
		Title:   title,
		Content: content,
		Images:  uploads,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrReviewValidation):
			return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
		case errors.Is(err, service.ErrReviewAlreadyExist):
			return c.JSON(http.StatusConflict, util.Error(err.Error()))
		case errors.Is(err, service.ErrDestinationNotFound):
			return c.JSON(http.StatusNotFound, util.Error(err.Error()))
		default:
			return c.JSON(http.StatusInternalServerError, util.Error("unable to create review"))
		}
	}

	resp := ReviewCreateResponse{
		Review:    toReviewResponse(*review),
		Aggregate: toAggregateResponse(aggregate),
	}
	return c.JSON(http.StatusCreated, resp)
}

// listReviews handles GET /api/v1/destinations/{destination_id}/reviews
func (h *ReviewHandler) listReviews(c echo.Context) error {
	destID, err := uuid.Parse(c.Param("destination_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid destination id"))
	}

	filter, err := parseReviewFilter(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
	}

	result, err := h.reviews.ListDestinationReviews(c.Request().Context(), destID, filter)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDestinationNotFound):
			return c.JSON(http.StatusNotFound, util.Error(err.Error()))
		case errors.Is(err, service.ErrReviewValidation):
			return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
		default:
			return c.JSON(http.StatusInternalServerError, util.Error("unable to list reviews"))
		}
	}

	reviews := make([]ReviewResponse, 0, len(result.Reviews))
	for _, review := range result.Reviews {
		reviews = append(reviews, toReviewResponse(review))
	}

	resp := ReviewListResponse{
		DestinationID: result.DestinationID,
		AverageRating: result.Aggregate.AverageRating,
		TotalReviews:  result.Aggregate.TotalReviews,
		RatingCounts:  stringKeyedCounts(result.Aggregate.RatingCounts),
		Reviews:       reviews,
		Limit:         result.Limit,
		Offset:        result.Offset,
	}
	return c.JSON(http.StatusOK, resp)
}

// deleteReview handles DELETE /api/v1/reviews/{id}
func (h *ReviewHandler) deleteReview(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, util.Error("authentication required"))
	}

	reviewID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid review id"))
	}

	isAdmin, err := h.auth.IsAdmin(c.Request().Context(), user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, util.Error("unable to verify role"))
	}

	if err := h.reviews.DeleteReview(c.Request().Context(), reviewID, user.ID, isAdmin); err != nil {
		switch {
		case errors.Is(err, service.ErrReviewNotFound):
			return c.JSON(http.StatusNotFound, util.Error(err.Error()))
		case errors.Is(err, service.ErrReviewForbidden):
			return c.JSON(http.StatusForbidden, util.Error(err.Error()))
		default:
			return c.JSON(http.StatusInternalServerError, util.Error("unable to delete review"))
		}
	}

	return c.JSON(http.StatusOK, util.Envelope{"success": true})
}

func parseReviewFilter(c echo.Context) (domain.ReviewListFilter, error) {
	filter := domain.ReviewListFilter{}

	if limitStr := strings.TrimSpace(c.QueryParam("limit")); limitStr != "" {
		val, err := strconv.Atoi(limitStr)
		if err != nil {
			return domain.ReviewListFilter{}, errors.New("limit must be an integer")
		}
		filter.Limit = val
	}
	if offsetStr := strings.TrimSpace(c.QueryParam("offset")); offsetStr != "" {
		val, err := strconv.Atoi(offsetStr)
		if err != nil {
			return domain.ReviewListFilter{}, errors.New("offset must be an integer")
		}
		filter.Offset = val
	}
	if ratingStr := strings.TrimSpace(c.QueryParam("rating")); ratingStr != "" {
		val, err := strconv.Atoi(ratingStr)
		if err != nil {
			return domain.ReviewListFilter{}, errors.New("rating must be an integer")
		}
		filter.Rating = &val
	}
	if minRatingStr := strings.TrimSpace(c.QueryParam("min_rating")); minRatingStr != "" {
		val, err := strconv.Atoi(minRatingStr)
		if err != nil {
			return domain.ReviewListFilter{}, errors.New("min_rating must be an integer")
		}
		filter.MinRating = &val
	}
	if maxRatingStr := strings.TrimSpace(c.QueryParam("max_rating")); maxRatingStr != "" {
		val, err := strconv.Atoi(maxRatingStr)
		if err != nil {
			return domain.ReviewListFilter{}, errors.New("max_rating must be an integer")
		}
		filter.MaxRating = &val
	}
	if afterStr := strings.TrimSpace(c.QueryParam("posted_after")); afterStr != "" {
		t, err := time.Parse(time.RFC3339, afterStr)
		if err != nil {
			return domain.ReviewListFilter{}, errors.New("posted_after must be an RFC3339 timestamp")
		}
		filter.PostedAfter = &t
	}
	if beforeStr := strings.TrimSpace(c.QueryParam("posted_before")); beforeStr != "" {
		t, err := time.Parse(time.RFC3339, beforeStr)
		if err != nil {
			return domain.ReviewListFilter{}, errors.New("posted_before must be an RFC3339 timestamp")
		}
		filter.PostedBefore = &t
	}

	switch strings.ToLower(strings.TrimSpace(c.QueryParam("sort"))) {
	case "rating":
		filter.SortField = domain.ReviewSortRating
	default:
		filter.SortField = domain.ReviewSortCreatedAt
	}

	switch strings.ToLower(strings.TrimSpace(c.QueryParam("order"))) {
	case "asc":
		filter.SortOrder = domain.SortOrderAsc
	default:
		filter.SortOrder = domain.SortOrderDesc
	}

	return filter, nil
}

func buildReviewUploads(form *multipart.Form) ([]service.ReviewImageUpload, []io.ReadCloser, error) {
	if form == nil {
		return nil, nil, nil
	}

	var headers []*multipart.FileHeader
	if files := form.File["images"]; files != nil {
		headers = append(headers, files...)
	}
	if files := form.File["images[]"]; files != nil {
		headers = append(headers, files...)
	}

	uploads := make([]service.ReviewImageUpload, 0, len(headers))
	closers := make([]io.ReadCloser, 0, len(headers))
	for idx, header := range headers {
		file, err := header.Open()
		if err != nil {
			for _, closer := range closers {
				_ = closer.Close()
			}
			return nil, nil, err
		}
		closers = append(closers, file)
		uploads = append(uploads, service.ReviewImageUpload{
			Reader:      file,
			Size:        header.Size,
			FileName:    header.Filename,
			ContentType: header.Header.Get(echo.HeaderContentType),
			Ordering:    idx,
		})
	}
	return uploads, closers, nil
}

func toReviewResponse(review domain.Review) ReviewResponse {
	resp := ReviewResponse{
		ID:            review.ID,
		DestinationID: review.DestinationID,
		Rating:        review.Rating,
		Title:         review.Title,
		Content:       review.Content,
		CreatedAt:     review.CreatedAt,
		UpdatedAt:     review.UpdatedAt,
		Reviewer: ReviewAuthorResponse{
			ID:          review.UserID,
			DisplayName: reviewerDisplayName(review),
			Username:    review.ReviewerUsername,
			AvatarURL:   review.ReviewerAvatar,
		},
	}

	if len(review.Media) > 0 {
		resp.Media = make([]ReviewMediaResponse, 0, len(review.Media))
		for _, media := range review.Media {
			resp.Media = append(resp.Media, ReviewMediaResponse{
				ID:       media.ID,
				URL:      media.URL,
				Ordering: media.Ordering,
			})
		}
	}
	return resp
}

func toAggregateResponse(aggregate *domain.ReviewAggregate) ReviewAggregateResponse {
	if aggregate == nil {
		return ReviewAggregateResponse{
			AverageRating: 0,
			TotalReviews:  0,
			RatingCounts:  stringKeyedCounts(nil),
		}
	}
	return ReviewAggregateResponse{
		AverageRating: aggregate.AverageRating,
		TotalReviews:  aggregate.TotalReviews,
		RatingCounts:  stringKeyedCounts(aggregate.RatingCounts),
	}
}

func stringKeyedCounts(counts map[int]int) map[string]int {
	result := map[string]int{
		"0": 0,
		"1": 0,
		"2": 0,
		"3": 0,
		"4": 0,
		"5": 0,
	}
	for rating, count := range counts {
		key := strconv.Itoa(rating)
		result[key] = count
	}
	return result
}

func reviewerDisplayName(review domain.Review) string {
	if review.ReviewerName != nil {
		if trimmed := strings.TrimSpace(*review.ReviewerName); trimmed != "" {
			return trimmed
		}
	}
	if review.ReviewerUsername != nil {
		if trimmed := strings.TrimSpace(*review.ReviewerUsername); trimmed != "" {
			return trimmed
		}
	}
	if review.ReviewerEmail != nil {
		email := strings.TrimSpace(*review.ReviewerEmail)
		if email != "" {
			if idx := strings.Index(email, "@"); idx > 0 {
				return email[:idx]
			}
		}
	}
	return "Anonymous"
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	v := strings.TrimSpace(value)
	return &v
}
