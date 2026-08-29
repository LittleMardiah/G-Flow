// Package auth menyediakan layanan JWT (generate & validate) dan
// blacklist token via Redis untuk logout/suspend (jti storage).
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenTTL menggambarkan durasi berlaku access & refresh token.
const (
	AccessTokenTTL  = time.Hour
	RefreshTokenTTL = 7 * 24 * time.Hour
)

// ErrInvalidToken adalah sentinel error untuk token yang tidak valid /
// signature salah / sudah kedaluwarsa.
var ErrInvalidToken = errors.New("auth: token tidak valid atau sudah kedaluwarsa")

// Claims adalah payload JWT yang digunakan G-Flow.
type Claims struct {
	Sub      string `json:"sub"`       // user ID (uuid)
	Email    string `json:"email"`     // email user
	UserType string `json:"user_type"` // customer | driver | merchant | admin
	Jti      string `json:"jti"`       // unique per token
	Exp      int64  `json:"exp"`       // expiry (unix seconds)
	Iat      int64  `json:"iat"`       // issued at (unix seconds)
}

// JWTService menghasilkan dan memvalidasi JWT bertanda tangan HS256.
type JWTService struct {
	secret []byte
}

// NewJWTService membuat JWTService dari secret (env JWT_SECRET).
func NewJWTService(secret string) *JWTService {
	return &JWTService{secret: []byte(secret)}
}

// GenerateToken membuat pasangan access token (1 jam) dan refresh token
// (7 hari) untuk user. Setiap token diberi jti (UUID unik).
func (s *JWTService) GenerateToken(userID uuid.UUID, email string, userType string) (accessToken string, refreshToken string, err error) {
	now := time.Now()

	accessToken, err = s.sign(userID, email, userType, now, AccessTokenTTL)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = s.sign(userID, email, userType, now, RefreshTokenTTL)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// sign membuat JWT HS256 dengan claims dari data user dan TTL tertentu.
func (s *JWTService) sign(userID uuid.UUID, email string, userType string, now time.Time, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":       userID.String(),
		"email":     email,
		"user_type": userType,
		"jti":       uuid.New().String(),
		"iat":       now.Unix(),
		"exp":       now.Add(ttl).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ValidateToken mem-parsing, memverifikasi signature, dan memeriksa expiry
// token. Mengembalikan Claims jika valid, atau error jika tidak.
func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	parsed, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(t *jwt.Token) (interface{}, error) {
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, ErrInvalidToken
	}

	mapClaims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, ErrInvalidToken
	}

	claims.Sub = stringOrEmpty(mapClaims["sub"])
	claims.Email = stringOrEmpty(mapClaims["email"])
	claims.UserType = stringOrEmpty(mapClaims["user_type"])
	claims.Jti = stringOrEmpty(mapClaims["jti"])

	if exp, ok := numOrZero(mapClaims["exp"]); ok {
		claims.Exp = exp
	} else {
		return nil, ErrInvalidToken
	}

	if iat, ok := numOrZero(mapClaims["iat"]); ok {
		claims.Iat = iat
	}

	return claims, nil
}

func stringOrEmpty(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func numOrZero(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	}
	return 0, false
}
