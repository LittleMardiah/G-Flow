//go:build integration

// Package auth — unit test murni (tanpa mock/minmock) untuk helper JWT.
// Menutup branch yang tidak bisa dijangkau lewat HTTP integration test
// (error path parsing token, parsing klaim numerik/string). Digabungkan
// saat `go test -tags integration`.
package auth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// TestValidateToken_InvalidSignature menutup branch error parsing/signature JWT.
func TestValidateToken_InvalidSignature(t *testing.T) {
	svc := NewJWTService("correct-secret")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":       "00000000-0000-0000-0000-000000000001",
		"email":     "a@test.com",
		"user_type": "customer",
		"jti":       "abc",
		"exp":       4102444800,
	})
	signed, err := token.SignedString([]byte("wrong-secret"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := svc.ValidateToken(signed); err == nil {
		t.Fatalf("expected error untuk signature yang salah")
	}
}

// TestValidateToken_MissingRequiredClaims menutup branch klaim hilang
// (exp tidak ada -> numOrZero ok=false -> ErrInvalidToken).
func TestValidateToken_MissingRequiredClaims(t *testing.T) {
	svc := NewJWTService("correct-secret")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "00000000-0000-0000-0000-000000000001",
	})
	signed, err := token.SignedString([]byte("correct-secret"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := svc.ValidateToken(signed); err == nil {
		t.Fatalf("expected error karena klaim exp hilang")
	}
}

// TestNumOrZero_Branches menguji parsing klaim numerik dengan tipe berbeda.
func TestNumOrZero_Branches(t *testing.T) {
	if v, ok := numOrZero(float64(123)); !ok || v != 123 {
		t.Fatalf("float64: got %d %v", v, ok)
	}
	if v, ok := numOrZero(int64(456)); !ok || v != 456 {
		t.Fatalf("int64: got %d %v", v, ok)
	}
	if v, ok := numOrZero(int(789)); !ok || v != 789 {
		t.Fatalf("int: got %d %v", v, ok)
	}
	if v, ok := numOrZero("bukan-angka"); ok {
		t.Fatalf("string harusnya false, got %d ok=%v", v, ok)
	}
}

// TestStringOrEmpty_Branches menguji konversi klaim string.
func TestStringOrEmpty_Branches(t *testing.T) {
	if s := stringOrEmpty("hello"); s != "hello" {
		t.Fatalf("string: got %q", s)
	}
	if s := stringOrEmpty(42); s != "" {
		t.Fatalf("non-string: got %q, want empty", s)
	}
}
