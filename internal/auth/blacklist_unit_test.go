package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestBlacklist_Add_Success(t *testing.T) {
	_, rdb := newTestRedis(t)
	s := NewBlacklistService(rdb)

	err := s.Add(context.Background(), "jti-1", time.Now().Add(time.Hour).Unix())
	assert.NoError(t, err)

	ok, err := s.IsBlacklisted(context.Background(), "jti-1")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestBlacklist_Add_Duplicate(t *testing.T) {
	_, rdb := newTestRedis(t)
	s := NewBlacklistService(rdb)

	exp := time.Now().Add(time.Hour).Unix()
	assert.NoError(t, s.Add(context.Background(), "jti-dup", exp))
	assert.NoError(t, s.Add(context.Background(), "jti-dup", exp))

	ok, err := s.IsBlacklisted(context.Background(), "jti-dup")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestBlacklist_Add_ExpiredToken(t *testing.T) {
	_, rdb := newTestRedis(t)
	s := NewBlacklistService(rdb)

	err := s.Add(context.Background(), "jti-expired", time.Now().Add(-time.Hour).Unix())
	assert.NoError(t, err)

	// Karena TTL negatif, jti tidak disimpan.
	ok, err := s.IsBlacklisted(context.Background(), "jti-expired")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestBlacklist_IsBlacklisted_NotFound(t *testing.T) {
	_, rdb := newTestRedis(t)
	s := NewBlacklistService(rdb)

	ok, err := s.IsBlacklisted(context.Background(), "never-added-jti")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestBlacklist_IsBlacklisted_RedisDown(t *testing.T) {
	mr := miniredis.NewMiniRedis()
	assert.NoError(t, mr.Start())
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mr.Close()
	defer func() { _ = rdb.Close() }()

	s := NewBlacklistService(rdb)
	_, err := s.IsBlacklisted(context.Background(), "jti-down")
	assert.Error(t, err)

	err = s.Add(context.Background(), "jti-down", time.Now().Add(time.Hour).Unix())
	assert.Error(t, err)
}

func TestBlacklist_NilRedis(t *testing.T) {
	s := NewBlacklistService(nil)

	assert.NoError(t, s.Add(context.Background(), "jti", time.Now().Add(time.Hour).Unix()))

	ok, err := s.IsBlacklisted(context.Background(), "jti")
	assert.NoError(t, err)
	assert.False(t, ok)
}
