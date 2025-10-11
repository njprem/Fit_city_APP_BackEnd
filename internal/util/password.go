package util

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"unicode"

	"golang.org/x/crypto/argon2"
)

const (
	saltLength   = 16
	hashLength   = 32
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
)

func GenerateSalt() ([]byte, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return salt, nil
}

func ValidatePassword(password string) error {
	if len(password) < 12 {
		return errors.New("password must be at least 12 characters long")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r), unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return errors.New("password must include uppercase, lowercase, number, and special character")
	}

	return nil
}

func HashPassword(password string, salt []byte) ([]byte, error) {
	if len(password) == 0 {
		return nil, errors.New("password cannot be empty")
	}
	if len(salt) == 0 {
		return nil, errors.New("salt cannot be empty")
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, hashLength)
	return hash, nil
}

func DerivePassword(password string) (hash, salt []byte, err error) {
	salt, err = GenerateSalt()
	if err != nil {
		return nil, nil, err
	}
	hash, err = HashPassword(password, salt)
	if err != nil {
		return nil, nil, err
	}
	return hash, salt, nil
}

func VerifyPassword(password string, salt, expectedHash []byte) bool {
	if len(password) == 0 || len(salt) == 0 || len(expectedHash) == 0 {
		return false
	}
	candidate, err := HashPassword(password, salt)
	if err != nil {
		return false
	}
	if len(candidate) != len(expectedHash) {
		return false
	}
	return subtle.ConstantTimeCompare(candidate, expectedHash) == 1
}
