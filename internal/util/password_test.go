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

func TestValidatePassword(t *testing.T) {
	valid := "StrongPass1!"
	if err := ValidatePassword(valid); err != nil {
		t.Fatalf("expected %q to be valid, got %v", valid, err)
	}

	cases := []struct {
		name string
		pass string
	}{
		{name: "too short", pass: "Aa1!short"},
		{name: "no uppercase", pass: "lowercase1!"},
		{name: "no lowercase", pass: "UPPERCASE1!"},
		{name: "no digit", pass: "NoDigits!!!!"},
		{name: "no special", pass: "NoSpecial1234"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidatePassword(tc.pass); err == nil {
				t.Fatalf("expected password %q to be invalid", tc.pass)
			}
		})
	}
}
