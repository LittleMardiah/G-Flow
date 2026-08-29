package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// BlacklistService menyimpan jti token yang di-blacklist di Redis (logout /
// suspend) dengan TTL sampai token expired.
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
