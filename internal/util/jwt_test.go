package util

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTManagerGenerateAndParse(t *testing.T) {
	ttl := time.Minute
	manager := NewJWTManager("top-secret", ttl)

	userID := uuid.New()
	username := "tester"
	token, expiresAt, err := manager.Generate(userID, "user@example.com", &username, true)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if token == "" {
		t.Fatalf("expected token to be non-empty")
	}
	if expiresAt.Before(time.Now()) {
		t.Fatalf("expected expiry in the future")
	}

	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if claims.UserID != userID {
		t.Fatalf("expected user id %s, got %s", userID, claims.UserID)
	}
	if claims.Username == nil || *claims.Username != username {
		t.Fatalf("expected username claim to be set")
	}
	if !claims.ProfileCompleted {
		t.Fatalf("expected profile completed claim to be true")
	}
}

func TestJWTManagerParseExpiredToken(t *testing.T) {
	manager := NewJWTManager("secret", time.Millisecond)
	userID := uuid.New()
	token, _, err := manager.Generate(userID, "user@example.com", nil, false)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	time.Sleep(2 * time.Millisecond)

	if _, err := manager.Parse(token); err == nil {
		t.Fatalf("expected parse error for expired token")
	}
}
