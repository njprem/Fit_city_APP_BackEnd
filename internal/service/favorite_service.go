package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/ports"
)

var (
	ErrFavoriteAlreadyExists = errors.New("destination already saved to favorites")
	ErrFavoriteNotFound      = errors.New("favorite not found")
)

type FavoriteService struct {
	favorites    ports.FavoriteRepository
	destinations ports.DestinationRepository
}

type FavoriteListResult struct {
	Items  []domain.FavoriteListItem
	Total  int64
	Limit  int
	Offset int
}

func NewFavoriteService(favoriteRepo ports.FavoriteRepository, destinationRepo ports.DestinationRepository) *FavoriteService {
	return &FavoriteService{
		favorites:    favoriteRepo,
		destinations: destinationRepo,
	}
}

func (s *FavoriteService) Save(ctx context.Context, userID, destinationID uuid.UUID) (*domain.Favorite, error) {
	if _, err := s.destinations.FindPublishedByID(ctx, destinationID); err != nil {
		if isNotFound(err) {
			return nil, ErrDestinationNotFound
		}
		return nil, err
	}

	favorite, err := s.favorites.Add(ctx, userID, destinationID)
	if err != nil {
		switch {
		case isNotFound(err), isUniqueViolation(err):
			return nil, ErrFavoriteAlreadyExists
		default:
			return nil, err
		}
	}
	return favorite, nil
}

func (s *FavoriteService) Remove(ctx context.Context, userID, destinationID uuid.UUID) error {
	if err := s.favorites.Remove(ctx, userID, destinationID); err != nil {
		if isNotFound(err) {
			return ErrFavoriteNotFound
		}
		return err
	}
	return nil
}

func (s *FavoriteService) List(ctx context.Context, userID uuid.UUID, limit, offset int) (*FavoriteListResult, error) {
	nLimit, nOffset := normalizeFavoritesPagination(limit, offset)

	items, err := s.favorites.ListByUser(ctx, userID, nLimit, nOffset)
	if err != nil {
		return nil, err
	}

	total, err := s.favorites.CountByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &FavoriteListResult{
		Items:  items,
		Total:  total,
		Limit:  nLimit,
		Offset: nOffset,
	}, nil
}

func (s *FavoriteService) Count(ctx context.Context, destinationID uuid.UUID) (int64, error) {
	if _, err := s.destinations.FindPublishedByID(ctx, destinationID); err != nil {
		if isNotFound(err) {
			return 0, ErrDestinationNotFound
		}
		return 0, err
	}
	return s.favorites.CountByDestination(ctx, destinationID)
}

func normalizeFavoritesPagination(limit, offset int) (int, int) {
	const (
		defaultLimit = 20
		maxLimit     = 100
	)

	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
