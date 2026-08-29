package auth

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// deadClient membuat redis.Client yang menunjuk ke alamat mati supaya operasi
// return error (menguji branch error handling BlacklistService) dengan cepat.
func deadClient(t *testing.T) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 200 * time.Millisecond,
	})
}

// TestBlacklistAdd_RedisError: Redis tidak dapat dijangkau -> Add return error.
func TestBlacklistAdd_RedisError(t *testing.T) {
	s := NewBlacklistService(deadClient(t))
	err := s.Add(context.Background(), "some-jti", time.Now().Add(time.Hour).Unix())
	assert.Error(t, err)
}

// TestIsBlacklisted_RedisError: Redis tidak dapat dijangkau -> IsBlacklisted
// return error (bukan false, karena kegagalan infrastruktur berbeda dengan
// "tidak ada di blacklist").
func TestIsBlacklisted_RedisError(t *testing.T) {
	s := NewBlacklistService(deadClient(t))
	_, err := s.IsBlacklisted(context.Background(), "some-jti")
	assert.Error(t, err)
}
