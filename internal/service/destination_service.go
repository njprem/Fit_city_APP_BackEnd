package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/ports"
)

type DestinationService struct {
	destinations ports.DestinationRepository
}

func NewDestinationService(destRepo ports.DestinationRepository) *DestinationService {
	return &DestinationService{destinations: destRepo}
}

func (s *DestinationService) ListPublished(ctx context.Context, limit, offset int) ([]domain.Destination, error) {
	return s.destinations.ListPublished(ctx, limit, offset)
}

func (s *DestinationService) GetPublishedByID(ctx context.Context, id uuid.UUID) (*domain.Destination, error) {
	dest, err := s.destinations.FindPublishedByID(ctx, id)
	if err != nil {
		return nil, ErrDestinationNotFound
	}
	return dest, nil
}

func (s *DestinationService) GetPublishedBySlug(ctx context.Context, slug string) (*domain.Destination, error) {
	dest, err := s.destinations.FindBySlug(ctx, slug)
	if err != nil {
		return nil, ErrDestinationNotFound
	}
	if dest == nil || !dest.IsPublished() {
		return nil, ErrDestinationNotFound
	}
	return dest, nil
}
