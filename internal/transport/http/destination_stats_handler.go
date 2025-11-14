package http

import (
	"encoding/csv"
	"errors"
	"fmt"
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

type DestinationStatsHandler struct {
	auth         *service.AuthService
	destinations *service.DestinationService
	stats        *service.DestinationViewStatsService
}

func RegisterDestinationStats(e *echo.Echo, auth *service.AuthService, destSvc *service.DestinationService, statsSvc *service.DestinationViewStatsService) {
	handler := &DestinationStatsHandler{
		auth:         auth,
		destinations: destSvc,
		stats:        statsSvc,
	}

	public := e.Group("/api/v1/destinations")
	public.GET("/:identifier/views", handler.getDestinationViews)
	public.GET("/trending", handler.trendingDestinations)

	admin := e.Group("/api/v1/admin/destination-stats", RequireAuth(auth), RequireAdmin(auth))
	admin.GET("/views", handler.adminDestinationStats)
	admin.POST("/export", handler.exportDestinationPopularity)
}

func (h *DestinationStatsHandler) getDestinationViews(c echo.Context) error {
	dest, err := h.resolveDestination(c, strings.TrimSpace(c.Param("identifier")))
	if err != nil {
		return err
	}

	stats, err := h.stats.GetViewStats(c.Request().Context(), dest, false)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, util.Error("unable to fetch destination stats"))
	}

	ranges := parseRangeFilter(c.QueryParam("range"))
	return c.JSON(http.StatusOK, buildViewResponse(stats, ranges))
}

func (h *DestinationStatsHandler) trendingDestinations(c echo.Context) error {
	rangeKey := parseRangeKey(c.QueryParam("range"))
	limit, _ := strconvAtoiDefault(c.QueryParam("limit"), 10)
	if limit <= 0 {
		limit = 10
	}
	stats, err := h.stats.Trending(c.Request().Context(), rangeKey, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, util.Error("unable to load trending destinations"))
	}

	destinations := make([]util.Envelope, 0, len(stats))
	for _, record := range stats {
		destinations = append(destinations, util.Envelope{
			"destination_id": record.DestinationID,
			"name":           record.Name,
			"city":           record.City,
			"country":        record.Country,
			"views":          buildRangeMap(record.Stats, []domain.DestinationViewRange{rangeKey}),
		})
	}

	return c.JSON(http.StatusOK, util.Envelope{
		"range":        string(rangeKey),
		"limit":        limit,
		"destinations": destinations,
	})
}

func (h *DestinationStatsHandler) adminDestinationStats(c echo.Context) error {
	rawID := strings.TrimSpace(c.QueryParam("destination_id"))
	if rawID == "" {
		return c.JSON(http.StatusBadRequest, util.Error("destination_id is required"))
	}
	destID, err := uuid.Parse(rawID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("destination_id must be a valid UUID"))
	}

	dest, err := h.destinations.GetPublishedByID(c.Request().Context(), destID)
	if err != nil {
		if errors.Is(err, service.ErrDestinationNotFound) {
			return c.JSON(http.StatusNotFound, util.Error("destination not found"))
		}
		return c.JSON(http.StatusInternalServerError, util.Error("unable to load destination"))
	}

	stats, err := h.stats.GetViewStats(c.Request().Context(), dest, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, util.Error("unable to fetch destination stats"))
	}

	ranges := parseRangeFilter(c.QueryParam("range"))
	return c.JSON(http.StatusOK, buildViewResponse(stats, ranges))
}

func (h *DestinationStatsHandler) exportDestinationPopularity(c echo.Context) error {
	var req struct {
		DestinationIDs []string `json:"destination_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid request body"))
	}

	destIDs := make([]uuid.UUID, 0, len(req.DestinationIDs))
	for _, raw := range req.DestinationIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return c.JSON(http.StatusBadRequest, util.Error("destination_ids must be valid UUIDs"))
		}
		destIDs = append(destIDs, id)
	}

	records, err := h.stats.ExportPopularity(c.Request().Context(), destIDs, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, util.Error("unable to export destination stats"))
	}

	fileName := fmt.Sprintf("destination-popularity-%d.csv", time.Now().Unix())
	res := c.Response()
	res.Header().Set(echo.HeaderContentType, "text/csv")
	res.Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", fileName))

	writer := csv.NewWriter(res.Writer)
	header := []string{"destination_name", "city", "country", "views_1h", "views_6h", "views_12h", "views_24h", "views_7d", "views_30d", "views_all"}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, record := range records {
		row := []string{
			record.Name,
			valueOrEmpty(record.City),
			valueOrEmpty(record.Country),
		}
		for _, key := range domain.DestinationViewRangesOrdered {
			val := record.Stats[key]
			row = append(row, fmt.Sprintf("%d", val.TotalViews))
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func (h *DestinationStatsHandler) resolveDestination(c echo.Context, identifier string) (*domain.Destination, error) {
	if identifier == "" {
		return nil, c.JSON(http.StatusBadRequest, util.Error("destination identifier required"))
	}
	if id, err := uuid.Parse(identifier); err == nil {
		dest, err := h.destinations.GetPublishedByID(c.Request().Context(), id)
		if err != nil {
			if errors.Is(err, service.ErrDestinationNotFound) {
				return nil, c.JSON(http.StatusNotFound, util.Error("destination not found"))
			}
			return nil, c.JSON(http.StatusInternalServerError, util.Error("unable to load destination"))
		}
		return dest, nil
	}

	dest, err := h.destinations.GetPublishedBySlug(c.Request().Context(), identifier)
	if err != nil {
		if errors.Is(err, service.ErrDestinationNotFound) {
			return nil, c.JSON(http.StatusNotFound, util.Error("destination not found"))
		}
		return nil, c.JSON(http.StatusInternalServerError, util.Error("unable to load destination"))
	}
	return dest, nil
}

func buildViewResponse(stats domain.DestinationViewStats, ranges []domain.DestinationViewRange) util.Envelope {
	responseRanges := buildRangeMap(stats.Ranges, ranges)
	return util.Envelope{
		"destination_id": stats.DestinationID,
		"name":           stats.Name,
		"city":           stats.City,
		"country":        stats.Country,
		"views":          responseRanges,
	}
}

func buildRangeMap(values map[domain.DestinationViewRange]domain.DestinationViewStatValue, ranges []domain.DestinationViewRange) map[string]util.Envelope {
	if len(ranges) == 0 {
		ranges = domain.DestinationViewRangesOrdered
	}
	result := make(map[string]util.Envelope, len(ranges))
	for _, key := range ranges {
		val := values[key]
		result[string(key)] = util.Envelope{
			"total_views":     val.TotalViews,
			"unique_users":    val.UniqueUsers,
			"unique_ips":      val.UniqueIPs,
			"last_updated_at": val.BucketEnd.UTC().Format(time.RFC3339),
		}
	}
	return result
}

func parseRangeFilter(raw string) []domain.DestinationViewRange {
	if strings.TrimSpace(raw) == "" {
		return domain.DestinationViewRangesOrdered
	}
	parts := strings.Split(raw, ",")
	seen := make(map[domain.DestinationViewRange]bool, len(parts))
	result := make([]domain.DestinationViewRange, 0, len(parts))
	for _, part := range parts {
		key := parseRangeKey(part)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, key)
	}
	return result
}

func parseRangeKey(raw string) domain.DestinationViewRange {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "1h":
		return domain.DestinationViewRange1h
	case "6h":
		return domain.DestinationViewRange6h
	case "12h":
		return domain.DestinationViewRange12h
	case "24h":
		return domain.DestinationViewRange24h
	case "7d":
		return domain.DestinationViewRange7d
	case "30d":
		return domain.DestinationViewRange30d
	case "all", "alltime", "":
		return domain.DestinationViewRangeAll
	default:
		return domain.DestinationViewRangeAll
	}
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func strconvAtoiDefault(raw string, fallback int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return fallback, err
	}
	return val, nil
}
