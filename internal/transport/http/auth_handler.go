package http

import (
	"net/http"
	"FitCity-API/internal/service"
	"github.com/labstack/echo/v4"
)

func RegisterAuth(e *echo.Echo, s *service.AuthService) {
	e.POST("/v1/auth/google", func(c echo.Context) error {
		var req struct{ IDToken string `json:"id_token"` }
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error":"bad body"})
		}
		jwt, err := s.LoginWithGoogle(c.Request().Context(), req.IDToken)
		if err != nil { return c.JSON(http.StatusUnauthorized, echo.Map{"error": err.Error()}) }
		return c.JSON(http.StatusOK, echo.Map{"token": jwt})
	})
}