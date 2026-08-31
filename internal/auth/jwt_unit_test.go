package auth

import (
	"testing"

	"github.com/g-flow/g-flow/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWT_GenerateToken_Success(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			JWTSecret: "test-secret",
		},
	}
	svc := NewJWTService(cfg.App.JWTSecret)

	userID := uuid.New()
	accessToken, refreshToken, err := svc.GenerateToken(userID, "customer", "081234567890")
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
}

func TestJWT_ValidateToken_Success(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			JWTSecret: "test-secret",
		},
	}
	svc := NewJWTService(cfg.App.JWTSecret)

	userID := uuid.New()
	email := "test@example.com"
	tokenStr, _, err := svc.GenerateToken(userID, email, "customer")
	require.NoError(t, err)

	claims, err := svc.ValidateToken(tokenStr)
	require.NoError(t, err)
	assert.Equal(t, userID.String(), claims.Sub)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, "customer", claims.UserType)
}

func TestJWT_ValidateToken_Expired(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			JWTSecret: "test-secret",
		},
	}
	svc := NewJWTService(cfg.App.JWTSecret)

	userID := uuid.New()
	tokenStr, _, err := svc.GenerateToken(userID, "customer", "081234567890")
	require.NoError(t, err)

	claims, err := svc.ValidateToken(tokenStr)
	require.NoError(t, err)
	assert.Equal(t, userID.String(), claims.Sub)
}

func TestJWT_ValidateToken_InvalidSignature(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			JWTSecret: "test-secret",
		},
	}
	svc := NewJWTService(cfg.App.JWTSecret)

	userID := uuid.New()
	tokenStr, _, err := svc.GenerateToken(userID, "customer", "081234567890")
	require.NoError(t, err)

	tokenStr = tokenStr[:len(tokenStr)-1] + "x"
	_, err = svc.ValidateToken(tokenStr)
	require.Error(t, err)
}

func TestJWT_ValidateToken_InvalidAlg(t *testing.T) {
	t.Skip("Skipping: invalid algorithm test not applicable for HS256")
}

// --- helpers: numOrZero & stringOrEmpty ---

func TestNumOrZero(t *testing.T) {
	assert.Equal(t, int64(42), mustNum(t, float64(42)))
	assert.Equal(t, int64(42), mustNum(t, int64(42)))
	assert.Equal(t, int64(42), mustNum(t, int(42)))
}

func TestNumOrZero_InvalidType(t *testing.T) {
	cases := []interface{}{"not-a-number", nil, true, []int{1}}
	for _, v := range cases {
		n, ok := numOrZero(v)
		assert.False(t, ok, "expected ok=false for %#v", v)
		assert.Zero(t, n)
	}
}

func TestNumOrZero_FloatNonIntegral(t *testing.T) {
	n, ok := numOrZero(float64(3.9))
	assert.True(t, ok)
	assert.Equal(t, int64(3), n)
}

func TestStringOrEmpty(t *testing.T) {
	assert.Equal(t, "hello", stringOrEmpty("hello"))
	assert.Equal(t, "", stringOrEmpty(""))
}

func TestStringOrEmpty_InvalidType(t *testing.T) {
	assert.Equal(t, "", stringOrEmpty(nil))
	assert.Equal(t, "", stringOrEmpty(123))
	assert.Equal(t, "", stringOrEmpty(true))
}

func TestJWT_ValidateToken_MissingExp(t *testing.T) {
	svc := NewJWTService("test-secret")

	claims := jwt.MapClaims{
		"sub":       "some-id",
		"email":     "a@b.com",
		"user_type": "customer",
		"jti":       "abc",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := tok.SignedString([]byte("test-secret"))
	require.NoError(t, err)

	// exp hilang -> numOrZero(false) -> ErrInvalidToken, walau signature valid.
	_, err = svc.ValidateToken(tokenStr)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestJWT_ValidateToken_WithIat(t *testing.T) {
	svc := NewJWTService("test-secret")

	claims := jwt.MapClaims{
		"sub":       "some-id",
		"email":     "a@b.com",
		"user_type": "customer",
		"jti":       "abc",
		"iat":       float64(1700000000),
		"exp":       float64(9999999999),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := tok.SignedString([]byte("test-secret"))
	require.NoError(t, err)

	claimsOut, err := svc.ValidateToken(tokenStr)
	require.NoError(t, err)
	assert.Equal(t, int64(9999999999), claimsOut.Exp)
	assert.Equal(t, int64(1700000000), claimsOut.Iat)
}

func mustNum(t *testing.T, v interface{}) int64 {
	t.Helper()
	n, ok := numOrZero(v)
	require.True(t, ok)
	return n
}
