package http

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/util"
)

// RegisterSwagger registers the Swagger UI handler under /swagger.
func RegisterSwagger(e *echo.Echo) {
	e.GET("/swagger/doc.json", func(c echo.Context) error {
		specPath := filepath.Join("docs", "swagger.yaml")
		data, err := os.ReadFile(specPath)
		if err != nil {
			c.Logger().Errorf("load swagger spec: %v", err)
			return c.JSON(http.StatusInternalServerError, util.Error("unable to load swagger spec"))
		}
		jsonSpec, err := yaml.YAMLToJSON(data)
		if err != nil {
			c.Logger().Errorf("convert swagger spec: %v", err)
			return c.JSON(http.StatusInternalServerError, util.Error("unable to parse swagger spec"))
		}
		return c.Blob(http.StatusOK, echo.MIMEApplicationJSONCharsetUTF8, jsonSpec)
	})
	e.GET("/swagger/*", echoSwagger.WrapHandler)
}
