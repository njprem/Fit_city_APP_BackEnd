package util

import "testing"

func TestDeriveAndVerifyPassword(t *testing.T) {
	hash, salt, err := DerivePassword("s3cret-pass")
	if err != nil {
		t.Fatalf("DerivePassword returned error: %v", err)
	}
	if len(hash) == 0 || len(salt) == 0 {
		t.Fatalf("expected hash and salt to be populated")
	}
	if !VerifyPassword("s3cret-pass", salt, hash) {
		t.Fatalf("expected password verification to succeed")
	}
	if VerifyPassword("wrong-pass", salt, hash) {
		t.Fatalf("expected password verification to fail for wrong password")
	}
}

func TestHashPasswordEmptyInput(t *testing.T) {
	if _, err := HashPassword("", []byte{1, 2, 3}); err == nil {
		t.Fatalf("expected error when password empty")
	}
	if _, err := HashPassword("secret", nil); err == nil {
		t.Fatalf("expected error when salt empty")
	}
}
