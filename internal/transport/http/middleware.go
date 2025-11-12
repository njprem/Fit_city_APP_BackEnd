package http

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/service"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/util"
)

func RequireAuth(auth *service.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if strings.TrimSpace(authHeader) == "" {
				return c.JSON(http.StatusUnauthorized, util.Error("missing authorization header"))
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				return c.JSON(http.StatusUnauthorized, util.Error("invalid authorization header"))
			}
			token := strings.TrimSpace(parts[1])
			user, err := auth.Authenticate(c.Request().Context(), token)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, util.Error(err.Error()))
			}
			c.Set(contextUserKey, user)
			c.Set(contextTokenKey, token)
			return next(c)
		}
	}
}

func RequireAdmin(auth *service.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := c.Get(contextUserKey).(*domain.User)
			if !ok || user == nil {
				return c.JSON(http.StatusUnauthorized, util.Error("authentication required"))
			}
			isAdmin, err := auth.IsAdmin(c.Request().Context(), user)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, util.Error("unable to verify role"))
			}
			if !isAdmin {
				return c.JSON(http.StatusForbidden, util.Error("admin privileges required"))
			}
			return next(c)
		}
	}
}

func CurrentUser(c echo.Context) (*domain.User, bool) {
	user, ok := c.Get(contextUserKey).(*domain.User)
	return user, ok
}
