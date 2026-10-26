package auth

import (
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	pw := "CorrectHorseBatteryStaple123!"
	h, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}

	ok, err := VerifyPassword(pw, h)
	if err != nil {
		t.Fatalf("VerifyPassword error: %v", err)
	}
	if !ok {
		t.Errorf("expected password to verify")
	}

	bad, err := VerifyPassword("WrongPassword", h)
	if err != nil {
		t.Fatalf("VerifyPassword error: %v", err)
	}
	if bad {
		t.Errorf("expected wrong password to fail")
	}
}

func TestDummyVerify(t *testing.T) {
	DummyVerify("anyPasswordAttempt")
}
