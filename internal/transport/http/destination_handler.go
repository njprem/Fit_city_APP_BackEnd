package http

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/service"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/util"
)

type DestinationFeatures struct {
	View   bool
	Create bool
	Update bool
	Delete bool
}

type DestinationHandler struct {
	workflow     *service.DestinationWorkflowService
	destinations *service.DestinationService
	auth         *service.AuthService
	features     DestinationFeatures
}

func RegisterDestinations(e *echo.Echo, auth *service.AuthService, destService *service.DestinationService, workflow *service.DestinationWorkflowService, features DestinationFeatures) {
	handler := &DestinationHandler{
		workflow:     workflow,
		destinations: destService,
		auth:         auth,
		features:     features,
	}

	if features.View {
		public := e.Group("/api/v1/destinations")
		public.GET("", handler.listPublished)
		public.GET("/:id", handler.getDestination)
	}

	if features.Create || features.Update || features.Delete {
		admin := e.Group("/api/v1/admin/destination-changes", RequireAuth(auth), RequireAdmin(auth))
		admin.POST("", handler.createChange)
		admin.PUT("/:id", handler.updateChange)
		admin.POST("/:id/submit", handler.submitChange)
		admin.POST("/:id/approve", handler.approveChange)
		admin.POST("/:id/reject", handler.rejectChange)
		admin.GET("", handler.listChanges)
		admin.GET("/:id", handler.getChange)
		admin.POST("/:id/hero-image", handler.uploadHeroImage)
		admin.POST("/:id/gallery", handler.uploadGalleryImages)
	}
}

func (h *DestinationHandler) createChange(c echo.Context) error {
	if !h.features.Create {
		return c.JSON(http.StatusForbidden, util.Error("destination draft creation disabled"))
	}

	user, ok := CurrentUser(c)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, util.Error("authentication required"))
	}

	var req struct {
		Action        string                         `json:"action"`
		DestinationID string                         `json:"destination_id"`
		Fields        domain.DestinationChangeFields `json:"fields"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid request body"))
	}

	action, err := parseChangeAction(req.Action)
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
	}

	var destinationID *uuid.UUID
	if strings.TrimSpace(req.DestinationID) != "" {
		id, err := uuid.Parse(req.DestinationID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, util.Error("destination_id must be a valid UUID"))
		}
		destinationID = &id
	}

	change, err := h.workflow.CreateDraft(c.Request().Context(), user.ID, service.DestinationDraftInput{
		Action:        action,
		DestinationID: destinationID,
		Fields:        req.Fields,
	})
	if err != nil {
		return h.writeChangeError(c, err)
	}

	return c.JSON(http.StatusCreated, util.Envelope{
		"change_request": buildChangeResponse(change),
	})
}

func (h *DestinationHandler) updateChange(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, util.Error("authentication required"))
	}

	changeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid change id"))
	}

	var req struct {
		DraftVersion int                            `json:"draft_version"`
		Fields       domain.DestinationChangeFields `json:"fields"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid request body"))
	}

	change, err := h.workflow.UpdateDraft(c.Request().Context(), changeID, user.ID, req.DraftVersion, req.Fields)
	if err != nil {
		return h.writeChangeError(c, err)
	}

	return c.JSON(http.StatusOK, util.Envelope{
		"change_request": buildChangeResponse(change),
	})
}

func (h *DestinationHandler) submitChange(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, util.Error("authentication required"))
	}

	changeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid change id"))
	}

	change, err := h.workflow.GetChange(c.Request().Context(), changeID)
	if err != nil {
		return h.writeChangeError(c, err)
	}

	if !h.isActionEnabled(change.Action) {
		return c.JSON(http.StatusForbidden, util.Error("feature disabled for this action"))
	}

	change, err = h.workflow.SubmitDraft(c.Request().Context(), change.ID, user.ID)
	if err != nil {
		return h.writeChangeError(c, err)
	}

	return c.JSON(http.StatusAccepted, util.Envelope{
		"change_request": buildChangeResponse(change),
		"message":        "Destination submitted for review",
	})
}

func (h *DestinationHandler) approveChange(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, util.Error("authentication required"))
	}

	changeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid change id"))
	}

	change, err := h.workflow.GetChange(c.Request().Context(), changeID)
	if err != nil {
		return h.writeChangeError(c, err)
	}

	if !h.isActionEnabled(change.Action) {
		return c.JSON(http.StatusForbidden, util.Error("feature disabled for this action"))
	}

	change, destination, err := h.workflow.Approve(c.Request().Context(), changeID, user.ID)
	if err != nil {
		return h.writeChangeError(c, err)
	}

	resp := util.Envelope{
		"change_request": buildChangeResponse(change),
		"message":        "Destination updated successfully",
	}
	if destination != nil {
		resp["destination"] = buildDestinationResponse(destination)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *DestinationHandler) rejectChange(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, util.Error("authentication required"))
	}

	changeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid change id"))
	}

	change, err := h.workflow.GetChange(c.Request().Context(), changeID)
	if err != nil {
		return h.writeChangeError(c, err)
	}

	if !h.isActionEnabled(change.Action) {
		return c.JSON(http.StatusForbidden, util.Error("feature disabled for this action"))
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid request body"))
	}

	change, err = h.workflow.Reject(c.Request().Context(), changeID, user.ID, req.Message)
	if err != nil {
		return h.writeChangeError(c, err)
	}

	return c.JSON(http.StatusOK, util.Envelope{
		"change_request": buildChangeResponse(change),
		"message":        "Destination change rejected",
	})
}

func (h *DestinationHandler) getChange(c echo.Context) error {
	changeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid change id"))
	}
	change, err := h.workflow.GetChange(c.Request().Context(), changeID)
	if err != nil {
		return h.writeChangeError(c, err)
	}
	return c.JSON(http.StatusOK, util.Envelope{
		"change_request": buildChangeResponse(change),
	})
}

func (h *DestinationHandler) listChanges(c echo.Context) error {
	statusFilters := strings.Split(strings.TrimSpace(c.QueryParam("status")), ",")
	statuses := make([]domain.DestinationChangeStatus, 0)
	for _, raw := range statusFilters {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		status, err := parseChangeStatus(raw)
		if err != nil {
			return c.JSON(http.StatusBadRequest, util.Error("invalid status filter"))
		}
		statuses = append(statuses, status)
	}

	var destinationID *uuid.UUID
	if id := strings.TrimSpace(c.QueryParam("destination_id")); id != "" {
		parsed, err := uuid.Parse(id)
		if err != nil {
			return c.JSON(http.StatusBadRequest, util.Error("destination_id must be a valid UUID"))
		}
		destinationID = &parsed
	}

	limit, offset := parsePagination(c, 50, 0)

	changes, err := h.workflow.ListChanges(c.Request().Context(), domain.DestinationChangeFilter{
		DestinationID: destinationID,
		Statuses:      statuses,
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		return h.writeChangeError(c, err)
	}

	payload := make([]util.Envelope, 0, len(changes))
	for i := range changes {
		payload = append(payload, buildChangeResponse(&changes[i]))
	}

	return c.JSON(http.StatusOK, util.Envelope{
		"changes": payload,
		"meta": util.Envelope{
			"limit":  limit,
			"offset": offset,
			"count":  len(payload),
		},
	})
}

func (h *DestinationHandler) uploadHeroImage(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, util.Error("authentication required"))
	}

	changeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid change id"))
	}

	change, err := h.workflow.GetChange(c.Request().Context(), changeID)
	if err != nil {
		return h.writeChangeError(c, err)
	}
	if !h.isActionEnabled(change.Action) {
		return c.JSON(http.StatusForbidden, util.Error("feature disabled for this action"))
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("file upload required"))
	}

	src, err := fileHeader.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("unable to read upload"))
	}
	defer src.Close()

	updated, err := h.workflow.UploadHeroImage(c.Request().Context(), change.ID, user.ID, service.HeroImageUpload{
		Reader:      src,
		Size:        fileHeader.Size,
		FileName:    fileHeader.Filename,
		ContentType: fileHeader.Header.Get("Content-Type"),
	})
	if err != nil {
		return h.writeChangeError(c, err)
	}

	return c.JSON(http.StatusOK, util.Envelope{
		"change_request": buildChangeResponse(updated),
	})
}

func (h *DestinationHandler) uploadGalleryImages(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, util.Error("authentication required"))
	}

	changeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid change id"))
	}

	change, err := h.workflow.GetChange(c.Request().Context(), changeID)
	if err != nil {
		return h.writeChangeError(c, err)
	}
	if !h.isActionEnabled(change.Action) {
		return c.JSON(http.StatusForbidden, util.Error("feature disabled for this action"))
	}

	if err := c.Request().ParseMultipartForm(32 << 20); err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid multipart payload"))
	}
	form := c.Request().MultipartForm
	if form == nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid multipart payload"))
	}
	var headers []*multipart.FileHeader
	if files := form.File["files"]; files != nil {
		headers = append(headers, files...)
	}
	if files := form.File["files[]"]; files != nil {
		headers = append(headers, files...)
	}
	if files := form.File["gallery"]; files != nil {
		headers = append(headers, files...)
	}
	if files := form.File["gallery[]"]; files != nil {
		headers = append(headers, files...)
	}
	if len(headers) == 0 {
		return c.JSON(http.StatusBadRequest, util.Error("at least one image file is required"))
	}

	uploads := make([]service.GalleryImageUpload, 0, len(headers))
	closers := make([]io.Closer, 0, len(headers))
	for _, header := range headers {
		file, err := header.Open()
		if err != nil {
			for _, closer := range closers {
				_ = closer.Close()
			}
			return c.JSON(http.StatusBadRequest, util.Error("unable to read upload"))
		}
		closers = append(closers, file)
		uploads = append(uploads, service.GalleryImageUpload{
			Reader:      file,
			Size:        header.Size,
			FileName:    header.Filename,
			ContentType: header.Header.Get("Content-Type"),
		})
	}
	defer func() {
		for _, closer := range closers {
			_ = closer.Close()
		}
	}()

	updated, results, err := h.workflow.UploadGalleryImages(c.Request().Context(), change.ID, user.ID, uploads)
	if err != nil {
		return h.writeChangeError(c, err)
	}

	uploadsResp := make([]util.Envelope, 0, len(results))
	for _, item := range results {
		uploadsResp = append(uploadsResp, util.Envelope{
			"upload_id": item.UploadID,
			"url":       item.URL,
			"ordering":  item.Ordering,
		})
	}

	return c.JSON(http.StatusOK, util.Envelope{
		"change_request":  buildChangeResponse(updated),
		"gallery_uploads": uploadsResp,
	})
}

func (h *DestinationHandler) listPublished(c echo.Context) error {
	if !h.features.View {
		return c.JSON(http.StatusNotFound, util.Error("resource not found"))
	}
	limit, offset := parsePagination(c, 20, 0)
	filter, err := parseDestinationListFilter(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
	}
	destinations, err := h.destinations.ListPublished(c.Request().Context(), limit, offset, filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, util.Error("unable to list destinations"))
	}
	payload := make([]util.Envelope, 0, len(destinations))
	for i := range destinations {
		payload = append(payload, buildDestinationResponse(&destinations[i]))
	}
	return c.JSON(http.StatusOK, util.Envelope{
		"destinations": payload,
		"meta": util.Envelope{
			"limit":  limit,
			"offset": offset,
			"count":  len(payload),
		},
	})
}

func (h *DestinationHandler) getDestination(c echo.Context) error {
	if !h.features.View {
		return c.JSON(http.StatusNotFound, util.Error("resource not found"))
	}
	key := strings.TrimSpace(c.Param("id"))
	if key == "" {
		return c.JSON(http.StatusBadRequest, util.Error("identifier required"))
	}

	var (
		dest *domain.Destination
		err  error
	)
	if id, parseErr := uuid.Parse(key); parseErr == nil {
		dest, err = h.destinations.GetPublishedByID(c.Request().Context(), id)
	} else {
		dest, err = h.destinations.GetPublishedBySlug(c.Request().Context(), key)
	}
	if err != nil {
		if errors.Is(err, service.ErrDestinationNotFound) {
			return c.JSON(http.StatusNotFound, util.Error("destination not found"))
		}
		return c.JSON(http.StatusInternalServerError, util.Error("unable to load destination"))
	}
	return c.JSON(http.StatusOK, util.Envelope{
		"destination": buildDestinationResponse(dest),
	})
}

func (h *DestinationHandler) writeChangeError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrDestinationChangeNotFound):
		return c.JSON(http.StatusNotFound, util.Error("change request not found"))
	case errors.Is(err, service.ErrDestinationNotFound):
		return c.JSON(http.StatusNotFound, util.Error("destination not found"))
	case errors.Is(err, service.ErrForbidden):
		return c.JSON(http.StatusForbidden, util.Error("forbidden"))
	case errors.Is(err, service.ErrInvalidChangeState):
		return c.JSON(http.StatusConflict, util.Error(err.Error()))
	case errors.Is(err, service.ErrReviewerConflict):
		return c.JSON(http.StatusForbidden, util.Error(err.Error()))
	case errors.Is(err, service.ErrHardDeleteNotAllowed):
		return c.JSON(http.StatusForbidden, util.Error(err.Error()))
	case errors.Is(err, service.ErrDestinationChangeValidation):
		return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
	case errors.Is(err, service.ErrHeroImageTooLarge), errors.Is(err, service.ErrHeroImageUnsupportedType), errors.Is(err, service.ErrHeroImageRequired):
		return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
	case errors.Is(err, service.ErrGalleryImageTooLarge), errors.Is(err, service.ErrGalleryImageUnsupportedType), errors.Is(err, service.ErrGalleryImageRequired):
		return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
	default:
		return c.JSON(http.StatusInternalServerError, util.Error("internal error"))
	}
}

func (h *DestinationHandler) isActionEnabled(action domain.DestinationChangeAction) bool {
	switch action {
	case domain.DestinationChangeActionCreate:
		return h.features.Create
	case domain.DestinationChangeActionUpdate:
		return h.features.Update
	case domain.DestinationChangeActionDelete:
		return h.features.Delete
	default:
		return false
	}
}

func parsePagination(c echo.Context, defaultLimit, defaultOffset int) (int, int) {
	limit := defaultLimit
	offset := defaultOffset
	if v := strings.TrimSpace(c.QueryParam("limit")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if v := strings.TrimSpace(c.QueryParam("offset")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	return limit, offset
}

func parseChangeAction(raw string) (domain.DestinationChangeAction, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "create":
		return domain.DestinationChangeActionCreate, nil
	case "update":
		return domain.DestinationChangeActionUpdate, nil
	case "delete":
		return domain.DestinationChangeActionDelete, nil
	default:
		return "", errors.New("invalid action")
	}
}

func parseChangeStatus(raw string) (domain.DestinationChangeStatus, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "draft":
		return domain.DestinationChangeStatusDraft, nil
	case "pending_review":
		return domain.DestinationChangeStatusPendingReview, nil
	case "approved":
		return domain.DestinationChangeStatusApproved, nil
	case "rejected":
		return domain.DestinationChangeStatusRejected, nil
	default:
		return "", errors.New("invalid status")
	}
}

func buildDestinationResponse(dest *domain.Destination) util.Envelope {
	if dest == nil {
		return util.Envelope{}
	}
	resp := util.Envelope{
		"id":         dest.ID,
		"name":       dest.Name,
		"status":     dest.Status,
		"version":    dest.Version,
		"created_at": dest.CreatedAt,
		"updated_at": dest.UpdatedAt,
	}
	if dest.Slug != nil {
		resp["slug"] = *dest.Slug
	}
	if dest.City != nil {
		resp["city"] = *dest.City
	}
	if dest.Country != nil {
		resp["country"] = *dest.Country
	}
	if dest.Category != nil {
		resp["category"] = *dest.Category
	}
	if dest.Description != nil {
		resp["description"] = *dest.Description
	}
	if dest.Contact != nil {
		resp["contact"] = *dest.Contact
	}
	if dest.OpeningTime != nil {
		resp["opening_time"] = *dest.OpeningTime
	}
	if dest.ClosingTime != nil {
		resp["closing_time"] = *dest.ClosingTime
	}
	if dest.Latitude != nil {
		resp["latitude"] = *dest.Latitude
	}
	if dest.Longitude != nil {
		resp["longitude"] = *dest.Longitude
	}
	if len(dest.Gallery) > 0 {
		resp["gallery"] = dest.Gallery
	}
	if dest.HeroImage != nil {
		resp["hero_image_url"] = *dest.HeroImage
	}
	if dest.UpdatedBy != nil {
		resp["updated_by"] = *dest.UpdatedBy
	}
	if dest.DeletedAt != nil {
		resp["deleted_at"] = *dest.DeletedAt
	}
	resp["average_rating"] = dest.AverageRating
	resp["review_count"] = dest.ReviewCount
	return resp
}

func parseDestinationListFilter(c echo.Context) (domain.DestinationListFilter, error) {
	filter := domain.DestinationListFilter{
		Search: strings.TrimSpace(c.QueryParam("query")),
		Sort:   domain.DestinationSortUpdatedAtDesc,
	}

	categories := make([]string, 0)
	if raw := c.QueryParam("categories"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				categories = append(categories, trimmed)
			}
		}
	}
	if rawValues, ok := c.QueryParams()["category"]; ok {
		for _, part := range rawValues {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				categories = append(categories, trimmed)
			}
		}
	}
	if len(categories) > 0 {
		filter.Categories = categories
	}

	if v := strings.TrimSpace(c.QueryParam("min_rating")); v != "" {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return domain.DestinationListFilter{}, errors.New("min_rating must be a number")
		}
		if parsed < 0 || parsed > 5 {
			return domain.DestinationListFilter{}, errors.New("min_rating must be between 0 and 5")
		}
		filter.MinRating = &parsed
	}
	if v := strings.TrimSpace(c.QueryParam("max_rating")); v != "" {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return domain.DestinationListFilter{}, errors.New("max_rating must be a number")
		}
		if parsed < 0 || parsed > 5 {
			return domain.DestinationListFilter{}, errors.New("max_rating must be between 0 and 5")
		}
		filter.MaxRating = &parsed
	}
	if filter.MinRating != nil && filter.MaxRating != nil && *filter.MinRating > *filter.MaxRating {
		return domain.DestinationListFilter{}, errors.New("min_rating cannot be greater than max_rating")
	}

	if raw := strings.TrimSpace(c.QueryParam("sort")); raw != "" {
		value := strings.ToLower(raw)
		switch value {
		case string(domain.DestinationSortRatingDesc), "rating":
			filter.Sort = domain.DestinationSortRatingDesc
		case string(domain.DestinationSortRatingAsc):
			filter.Sort = domain.DestinationSortRatingAsc
		case string(domain.DestinationSortNameAsc), "alphabetical", "alpha":
			filter.Sort = domain.DestinationSortNameAsc
		case string(domain.DestinationSortNameDesc), "alpha_desc":
			filter.Sort = domain.DestinationSortNameDesc
		case string(domain.DestinationSortUpdatedAtDesc), "updated", "recent":
			filter.Sort = domain.DestinationSortUpdatedAtDesc
		default:
			return domain.DestinationListFilter{}, fmt.Errorf("invalid sort value %q", raw)
		}
	}

	return filter, nil
}

func buildChangeResponse(change *domain.DestinationChangeRequest) util.Envelope {
	if change == nil {
		return util.Envelope{}
	}
	resp := util.Envelope{
		"id":            change.ID,
		"action":        change.Action,
		"status":        change.Status,
		"draft_version": change.DraftVersion,
		"submitted_by":  change.SubmittedBy,
		"created_at":    change.CreatedAt,
		"updated_at":    change.UpdatedAt,
		"fields":        buildChangeFields(change.Payload),
	}
	if change.DestinationID != nil {
		resp["destination_id"] = *change.DestinationID
	}
	if change.HeroImageTempKey != nil {
		resp["hero_image_temp_key"] = *change.HeroImageTempKey
	}
	if change.SubmittedAt != nil {
		resp["submitted_at"] = *change.SubmittedAt
	}
	if change.ReviewedAt != nil {
		resp["reviewed_at"] = *change.ReviewedAt
	}
	if change.ReviewedBy != nil {
		resp["reviewed_by"] = *change.ReviewedBy
	}
	if change.ReviewMessage != nil {
		resp["review_message"] = *change.ReviewMessage
	}
	if change.PublishedVersion != nil {
		resp["published_version"] = *change.PublishedVersion
	}
	return resp
}

func buildChangeFields(fields domain.DestinationChangeFields) util.Envelope {
	resp := util.Envelope{}
	if fields.Name != nil {
		resp["name"] = *fields.Name
	}
	if fields.Slug != nil {
		resp["slug"] = *fields.Slug
	}
	if fields.City != nil {
		resp["city"] = *fields.City
	}
	if fields.Country != nil {
		resp["country"] = *fields.Country
	}
	if fields.Category != nil {
		resp["category"] = *fields.Category
	}
	if fields.Description != nil {
		resp["description"] = *fields.Description
	}
	if fields.Contact != nil {
		resp["contact"] = *fields.Contact
	}
	if fields.OpeningTime != nil {
		resp["opening_time"] = *fields.OpeningTime
	}
	if fields.ClosingTime != nil {
		resp["closing_time"] = *fields.ClosingTime
	}
	if fields.Latitude != nil {
		resp["latitude"] = *fields.Latitude
	}
	if fields.Longitude != nil {
		resp["longitude"] = *fields.Longitude
	}
	if fields.Gallery != nil {
		resp["gallery"] = *fields.Gallery
	}
	if fields.Status != nil {
		resp["status"] = *fields.Status
	}
	if fields.HeroImageUploadID != nil {
		resp["hero_image_upload_id"] = *fields.HeroImageUploadID
	}
	if fields.HeroImageURL != nil {
		resp["hero_image_url"] = *fields.HeroImageURL
	}
	if fields.PublishedHeroImage != nil {
		resp["published_hero_image"] = *fields.PublishedHeroImage
	}
	if fields.HardDelete != nil {
		resp["hard_delete"] = *fields.HardDelete
	}
	return resp
}
