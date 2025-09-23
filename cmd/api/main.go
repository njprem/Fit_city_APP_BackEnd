package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()

	// Example route
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "FitCity API is running 🚀")
	})

	// Start server on port 8080
	e.Logger.Fatal(e.Start(":8080"))
}