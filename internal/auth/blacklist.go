package auth

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// BlacklistService menyimpan jti token yang di-blacklist di Redis (logout /
// suspend) dengan TTL sampai token expired. Selain itu juga menyediakan
// user-level revocation untuk memaksa semua token user menjadi invalid sejak
// waktu tertentu (berdasarkan iat token).
type BlacklistService struct {
	db *redis.Client
}

// NewBlacklistService membuat BlacklistService dari *redis.Client.
// Client boleh nil; saat itu semua operasi batal secara graceful.
func NewBlacklistService(db *redis.Client) *BlacklistService {
	return &BlacklistService{db: db}
}

// Add menandai jti sebagai blacklisted dengan TTL hingga token expires
// (exp adalah unix timestamp). Return nil jika sukses atau Redis unavailable.
func (s *BlacklistService) Add(ctx context.Context, jti string, exp int64) error {
	if s.db == nil {
		return nil
	}

	ttl := time.Until(time.Unix(exp, 0))
	if ttl < 0 {
		return nil
	}

	if err := s.db.Set(ctx, "blacklist:"+jti, "true", ttl).Err(); err != nil {
		return err
	}
	return nil
}

// IsBlacklisted mengembalikan true jika jti ada di blacklist. Jika Redis
// error, kembalikan false (graceful degradation: token dianggap valid).
func (s *BlacklistService) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	if s.db == nil {
		return false, nil
	}

	val, err := s.db.Exists(ctx, "blacklist:"+jti).Result()
	if err != nil {
		return false, err
	}
	return val > 0, nil
}

// RevokeUserBefore menetapkan waktu cutoff (untilUnix, unix timestamp) untuk
// user tertentu. Semua token dengan iat < untilUnix akan dianggap invalid
// (user-level revocation). TTL diset sama dengan RefreshTokenTTL karena refresh
// token umumnya bertahan paling lama; pendekatan ini menjaga key tetap ada
// selama perlu untuk mengevaluasi token aktif. Return error Redis jika gagal.
func (s *BlacklistService) RevokeUserBefore(ctx context.Context, userID string, untilUnix int64) error {
	if s.db == nil {
		return nil
	}

	key := "user_revoke_before:" + userID
	ttl := RefreshTokenTTL
	if err := s.db.Set(ctx, key, strconv.FormatInt(untilUnix, 10), ttl).Err(); err != nil {
		return err
	}
	return nil
}

// IsUserRevoked mengecek apakah user telah direvoke. Token dianggap revoked
// jika iat token < stored cutoff (token dikeluarkan sebelum waktu revoke).
// Jika key tidak ada, dianggap tidak direvoke. Jika Redis error, return error.
// Jika db nil (graceful), return false, nil.
func (s *BlacklistService) IsUserRevoked(ctx context.Context, userID string, iat int64) (bool, error) {
	if s.db == nil {
		return false, nil
	}

	key := "user_revoke_before:" + userID
	val, err := s.db.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	stored, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return false, err
	}
	return iat < stored, nil
}
