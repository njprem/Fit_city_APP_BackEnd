package ports

import "FitCity-API/internal/domain"

type UserRepository interface {
	UpsertByEmail(email, name string) (*domain.User, error)
	FindByID(id string) (*domain.User, error)
	FindByEmail(email string) (*domain.User, error)
}