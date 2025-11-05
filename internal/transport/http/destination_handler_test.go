package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

func TestParseDestinationListFilter(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/destinations", nil)
	q := req.URL.Query()
	q.Set("query", "  park  ")
	q.Set("categories", "Nature, City ")
	q.Add("category", "Outdoors")
	q.Set("min_rating", "2.5")
	q.Set("max_rating", "4.5")
	q.Set("sort", "alpha_desc")
	req.URL.RawQuery = q.Encode()
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	filter, err := parseDestinationListFilter(c)
	if err != nil {
		t.Fatalf("parseDestinationListFilter returned error: %v", err)
	}

	if filter.Search != "park" {
		t.Fatalf("expected search 'park', got %q", filter.Search)
	}

	expectedCategories := []string{"Nature", "City", "Outdoors"}
	if len(filter.Categories) != len(expectedCategories) {
		t.Fatalf("expected %d categories, got %d", len(expectedCategories), len(filter.Categories))
	}
	for i, expected := range expectedCategories {
		if filter.Categories[i] != expected {
			t.Fatalf("expected category %q at position %d, got %q", expected, i, filter.Categories[i])
		}
	}

	if filter.MinRating == nil || *filter.MinRating != 2.5 {
		t.Fatalf("expected min rating 2.5, got %v", filter.MinRating)
	}
	if filter.MaxRating == nil || *filter.MaxRating != 4.5 {
		t.Fatalf("expected max rating 4.5, got %v", filter.MaxRating)
	}
	if filter.Sort != domain.DestinationSortNameDesc {
		t.Fatalf("expected sort %q, got %q", domain.DestinationSortNameDesc, filter.Sort)
	}
}

func TestParseDestinationListFilterInvalidRating(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/destinations?min_rating=5&max_rating=2", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_, err := parseDestinationListFilter(c)
	if err == nil {
		t.Fatal("expected error for invalid rating range, got nil")
	}
}
