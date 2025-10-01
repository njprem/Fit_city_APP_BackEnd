package service

import (
	"time"
	"errors"
	"FitCity-API/internal/repository/ports"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/api/idtoken"
	"context"
)

type AuthService struct {
	users  ports.UserRepository
	jwtKey []byte
	aud    string
}

func NewAuthService(u ports.UserRepository, jwtSecret, googleAud string) *AuthService {
	return &AuthService{users: u, jwtKey: []byte(jwtSecret), aud: googleAud}
}

func (s *AuthService) LoginWithGoogle(ctx context.Context, idTok string) (string, error) {
	pl, err := idtoken.Validate(ctx, idTok, s.aud)
	if err != nil { return "", errors.New("invalid google token") }
	email, _ := pl.Claims["email"].(string)
	name, _  := pl.Claims["name"].(string)
	u, err := s.users.UpsertByEmail(email, name)
	if err != nil { return "", err }

	claims := jwt.MapClaims{"sub": u.ID, "email": u.Email, "exp": time.Now().Add(24*time.Hour).Unix()}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.jwtKey)
}