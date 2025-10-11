package util

import (
	"crypto/rand"
	"math/big"
	"strings"
)

func GenerateNumericOTP(digits int) (string, error) {
	if digits <= 0 {
		digits = 6
	}
	var builder strings.Builder
	builder.Grow(digits)
	for i := 0; i < digits; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		builder.WriteByte(byte('0' + n.Int64()))
	}
	return builder.String(), nil
}
