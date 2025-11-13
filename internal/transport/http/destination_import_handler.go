package http

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/service"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/util"
)

type DestinationImportHandler struct {
	service       *service.DestinationImportService
	maxUploadSize int64
}

func RegisterDestinationImports(e *echo.Echo, auth *service.AuthService, svc *service.DestinationImportService, enabled bool, maxUpload int64) {
	if !enabled || svc == nil {
		return
	}
	handler := &DestinationImportHandler{
		service:       svc,
		maxUploadSize: maxUpload,
	}

	group := e.Group("/api/v1/admin/destination-imports", RequireAuth(auth), RequireAdmin(auth))
	group.GET("/template", handler.template)
	group.POST("", handler.create)
	group.GET("/:id", handler.getJob)
	group.GET("/:id/errors", handler.downloadErrors)
}

func (h *DestinationImportHandler) template(c echo.Context) error {
	headers := []string{
		"slug", "name", "status", "category", "city", "country", "description",
		"latitude", "longitude", "contact", "opening_time", "closing_time",
		"hero_image_url", "gallery_1_url", "gallery_1_caption",
		"gallery_2_url", "gallery_2_caption", "gallery_3_url", "gallery_3_caption",
		"hero_image_upload_id", "published_hero_image",
	}
	sampleRow := []string{
		"central-park", "Central Park", "published", "Nature", "New York", "USA",
		"Iconic urban park with year-round programming.", "40.785091", "-73.968285",
		"+1 212-310-6600", "06:00", "22:00",
		"https://cdn.fitcity/destinations/central-park/hero.jpg",
		"https://cdn.fitcity/destinations/central-park/gallery-1.jpg", "Bethesda Fountain",
		"https://cdn.fitcity/destinations/central-park/gallery-2.jpg", "Bow Bridge",
		"", "", "", "", "",
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	_ = writer.Write(headers)
	_ = writer.Write(sampleRow)
	writer.Flush()

	if err := writer.Error(); err != nil {
		return c.JSON(http.StatusInternalServerError, util.Error("could not generate template"))
	}

	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename=\"destination-import-template.csv\"`)
	return c.Blob(http.StatusOK, "text/csv", buf.Bytes())
}

func (h *DestinationImportHandler) create(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, util.Error("authentication required"))
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("csv file is required"))
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("unable to read upload"))
	}
	defer src.Close()

	limit := h.maxUploadSize
	if limit <= 0 {
		limit = 8 * 1024 * 1024
	}

	data, err := io.ReadAll(io.LimitReader(src, limit+1))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("failed reading upload"))
	}
	if int64(len(data)) > limit {
		return c.JSON(http.StatusRequestEntityTooLarge, util.Error("upload exceeds size limit"))
	}

	notes := strings.TrimSpace(c.FormValue("notes"))
	var notesPtr *string
	if notes != "" {
		notesPtr = &notes
	}

	dryRun := strings.EqualFold(c.QueryParam("dry_run"), "true")

	job, rows, err := h.service.Import(c.Request().Context(), user.ID, file.Filename, data, dryRun, notesPtr)
	if err != nil {
		return h.writeError(c, err)
	}
	return c.JSON(http.StatusAccepted, util.Envelope{
		"job":  buildImportJob(job),
		"rows": buildImportRows(rows),
	})
}

func (h *DestinationImportHandler) getJob(c echo.Context) error {
	jobID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid job id"))
	}
	job, rows, err := h.service.GetJob(c.Request().Context(), jobID)
	if err != nil {
		return c.JSON(http.StatusNotFound, util.Error("import job not found"))
	}
	return c.JSON(http.StatusOK, util.Envelope{
		"job":  buildImportJob(job),
		"rows": buildImportRows(rows),
	})
}

func (h *DestinationImportHandler) downloadErrors(c echo.Context) error {
	jobID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid job id"))
	}
	_, rows, err := h.service.GetJob(c.Request().Context(), jobID)
	if err != nil {
		return c.JSON(http.StatusNotFound, util.Error("import job not found"))
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	_ = writer.Write([]string{"row_number", "error"})
	for _, row := range rows {
		if row.Status != domain.DestinationImportRowStatusFailed {
			continue
		}
		errMsg := ""
		if row.ErrorMessage != nil {
			errMsg = *row.ErrorMessage
		}
		_ = writer.Write([]string{strconv.Itoa(row.RowNumber), errMsg})
	}
	writer.Flush()

	if err := writer.Error(); err != nil {
		return c.JSON(http.StatusInternalServerError, util.Error("could not generate csv"))
	}

	return c.Blob(http.StatusOK, "text/csv", buf.Bytes())
}

func (h *DestinationImportHandler) writeError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrImportEmptyFile):
		return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
	case errors.Is(err, service.ErrImportInvalidHeaders):
		return c.JSON(http.StatusUnprocessableEntity, util.Error(err.Error()))
	case errors.Is(err, service.ErrImportTooLarge), errors.Is(err, service.ErrImportRowLimitExceeded):
		return c.JSON(http.StatusRequestEntityTooLarge, util.Error(err.Error()))
	default:
		return c.JSON(http.StatusInternalServerError, util.Error("internal error"))
	}
}

func buildImportJob(job *domain.DestinationImportJob) util.Envelope {
	resp := util.Envelope{
		"id":              job.ID,
		"uploaded_by":     job.UploadedBy,
		"status":          job.Status,
		"dry_run":         job.DryRun,
		"file_key":        job.FileKey,
		"total_rows":      job.TotalRows,
		"processed_rows":  job.ProcessedRows,
		"rows_failed":     job.RowsFailed,
		"changes_created": job.ChangesCreated,
		"submitted_at":    job.SubmittedAt,
		"created_at":      job.CreatedAt,
		"updated_at":      job.UpdatedAt,
	}
	if job.CompletedAt != nil {
		resp["completed_at"] = *job.CompletedAt
	}
	if job.Notes != nil {
		resp["notes"] = *job.Notes
	}
	if job.ErrorCSVKey != nil {
		resp["error_csv_key"] = *job.ErrorCSVKey
	}
	if len(job.PendingIDs) > 0 {
		resp["pending_change_ids"] = job.PendingIDs
	}
	return resp
}

func buildImportRows(rows []domain.DestinationImportRow) []util.Envelope {
	resp := make([]util.Envelope, 0, len(rows))
	for _, row := range rows {
		item := util.Envelope{
			"id":         row.ID,
			"row_number": row.RowNumber,
			"status":     row.Status,
			"action":     row.Action,
			"payload":    row.Payload,
			"created_at": row.CreatedAt,
		}
		if row.ChangeID != nil {
			item["change_id"] = *row.ChangeID
		}
		if row.ErrorMessage != nil {
			item["error"] = *row.ErrorMessage
		}
		resp = append(resp, item)
	}
	return resp
}
