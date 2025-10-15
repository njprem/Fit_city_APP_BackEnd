package http

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
)

func NewRouter(allowOrigins []string) *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	allowCredentials := true
	for _, origin := range allowOrigins {
		if origin == "*" {
			allowCredentials = false
			break
		}
	}

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.Secure())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: allowOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{
			echo.HeaderAuthorization,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderOrigin,
			echo.HeaderXRequestedWith,
		},
		AllowCredentials: allowCredentials,
	}))

	e.GET("/health", func(c echo.Context) error { return c.JSON(http.StatusOK, echo.Map{"ok": true}) })
	return e
}
