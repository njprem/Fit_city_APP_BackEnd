package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/service"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/util"
)

type FavoriteHandler struct {
	auth      *service.AuthService
	favorites *service.FavoriteService
}

type FavoriteItemResponse struct {
	ID            uuid.UUID                   `json:"id"`
	DestinationID uuid.UUID                   `json:"destination_id"`
	SavedAt       string                      `json:"saved_at"`
	Destination   FavoriteDestinationResponse `json:"destination"`
}

type FavoriteDestinationResponse struct {
	Name         string  `json:"name"`
	Slug         *string `json:"slug,omitempty"`
	City         *string `json:"city,omitempty"`
	Country      *string `json:"country,omitempty"`
	Category     *string `json:"category,omitempty"`
	HeroImageURL *string `json:"hero_image_url,omitempty"`
}

func RegisterFavorites(e *echo.Echo, auth *service.AuthService, favorites *service.FavoriteService) {
	handler := &FavoriteHandler{
		auth:      auth,
		favorites: favorites,
	}

	protected := e.Group("/api/v1/users/me/favorites", RequireAuth(auth))
	protected.POST("", handler.saveFavorite)
	protected.DELETE("/:destination_id", handler.removeFavorite)
	protected.GET("", handler.listFavorites)

	public := e.Group("/api/v1/destinations/:destination_id/favorites")
	public.GET("/count", handler.countFavorites)
}

func (h *FavoriteHandler) saveFavorite(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, util.Error("authentication required"))
	}

	var req struct {
		DestinationID string `json:"destination_id"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid request body"))
	}

	if strings.TrimSpace(req.DestinationID) == "" {
		return c.JSON(http.StatusBadRequest, util.Error("destination_id is required"))
	}

	destinationID, err := uuid.Parse(strings.TrimSpace(req.DestinationID))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("destination_id must be a valid UUID"))
	}

	favorite, err := h.favorites.Save(c.Request().Context(), user.ID, destinationID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDestinationNotFound):
			return c.JSON(http.StatusNotFound, util.Error("destination not found"))
		case errors.Is(err, service.ErrFavoriteAlreadyExists):
			return c.JSON(http.StatusConflict, util.Error("destination already saved"))
		default:
			return c.JSON(http.StatusInternalServerError, util.Error("could not update favorites"))
		}
	}

	return c.JSON(http.StatusCreated, util.Envelope{
		"favorite": util.Envelope{
			"id":             favorite.ID,
			"destination_id": favorite.DestinationID,
			"saved_at":       favorite.CreatedAt.UTC().Format(time.RFC3339),
		},
		"message": "Destination saved to Favorites",
	})
}

func (h *FavoriteHandler) removeFavorite(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, util.Error("authentication required"))
	}

	destinationID, err := uuid.Parse(strings.TrimSpace(c.Param("destination_id")))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("destination_id must be a valid UUID"))
	}

	if err := h.favorites.Remove(c.Request().Context(), user.ID, destinationID); err != nil {
		switch {
		case errors.Is(err, service.ErrFavoriteNotFound):
			return c.JSON(http.StatusNotFound, util.Error("destination is not in your favorites"))
		default:
			return c.JSON(http.StatusInternalServerError, util.Error("could not update favorites"))
		}
	}

	return c.JSON(http.StatusOK, util.Envelope{
		"destination_id": destinationID,
		"message":        "Destination removed from Favorites",
	})
}

func (h *FavoriteHandler) listFavorites(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, util.Error("authentication required"))
	}

	limit, offset := parsePagination(c, 20, 0)
	result, err := h.favorites.List(c.Request().Context(), user.ID, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, util.Error("unable to load favorites"))
	}

	items := make([]FavoriteItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toFavoriteItemResponse(item))
	}

	return c.JSON(http.StatusOK, util.Envelope{
		"items": items,
		"pagination": util.Envelope{
			"limit":  result.Limit,
			"offset": result.Offset,
			"total":  result.Total,
			"count":  len(items),
		},
	})
}

func (h *FavoriteHandler) countFavorites(c echo.Context) error {
	destinationID, err := uuid.Parse(strings.TrimSpace(c.Param("destination_id")))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("destination_id must be a valid UUID"))
	}

	count, err := h.favorites.Count(c.Request().Context(), destinationID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDestinationNotFound):
			return c.JSON(http.StatusNotFound, util.Error("destination not found"))
		default:
			return c.JSON(http.StatusInternalServerError, util.Error("unable to fetch favorites count"))
		}
	}

	return c.JSON(http.StatusOK, util.Envelope{
		"destination_id":   destinationID,
		"favorites_count":  count,
		"last_updated_utc": time.Now().UTC().Format(time.RFC3339),
	})
}

func toFavoriteItemResponse(item domain.FavoriteListItem) FavoriteItemResponse {
	return FavoriteItemResponse{
		ID:            item.ID,
		DestinationID: item.DestinationID,
		SavedAt:       item.CreatedAt.UTC().Format(time.RFC3339),
		Destination: FavoriteDestinationResponse{
			Name:         item.DestinationName,
			Slug:         item.DestinationSlug,
			City:         item.City,
			Country:      item.Country,
			Category:     item.Category,
			HeroImageURL: item.HeroImageURL,
		},
	}
}
