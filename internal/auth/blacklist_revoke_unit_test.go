package auth

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRevokeUserBefore_Success: set cutoff, key harus ada di Redis dengan
// value format int64 untilUnix dan TTL > 0.
func TestRevokeUserBefore_Success(t *testing.T) {
	_, rdb := newTestRedis(t)
	s := NewBlacklistService(rdb)

	const until = int64(1000)
	assert.NoError(t, s.RevokeUserBefore(context.Background(), "user-1", until))

	key := "user_revoke_before:user-1"
	val, err := rdb.Get(context.Background(), key).Result()
	assert.NoError(t, err)
	assert.Equal(t, strconv.FormatInt(until, 10), val)

	ttl, err := rdb.TTL(context.Background(), key).Result()
	assert.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))
}

// TestRevokeUserBefore_NilRedis: db=nil -> return nil, tidak panic.
func TestRevokeUserBefore_NilRedis(t *testing.T) {
	s := NewBlacklistService(nil)
	assert.NoError(t, s.RevokeUserBefore(context.Background(), "user-1", 1000))
}

// TestRevokeUserBefore_RedisDown: Redis mati -> return error.
func TestRevokeUserBefore_RedisDown(t *testing.T) {
	s := NewBlacklistService(deadClient(t))
	err := s.RevokeUserBefore(context.Background(), "user-1", 1000)
	assert.Error(t, err)
}

// TestIsUserRevoked_NilRedis: db=nil -> return (false, nil), tidak panic.
func TestIsUserRevoked_NilRedis(t *testing.T) {
	s := NewBlacklistService(nil)
	ok, err := s.IsUserRevoked(context.Background(), "user-1", 1000)
	assert.NoError(t, err)
	assert.False(t, ok)
}

// TestIsUserRevoked_KeyNotFound: key tidak ada -> (false, nil) tanpa error.
func TestIsUserRevoked_KeyNotFound(t *testing.T) {
	_, rdb := newTestRedis(t)
	s := NewBlacklistService(rdb)

	ok, err := s.IsUserRevoked(context.Background(), "user-1", 1000)
	assert.NoError(t, err)
	assert.False(t, ok)
}

// TestIsUserRevoked_BeforeCutoff: iat = T-1 < stored T -> revoked (true).
func TestIsUserRevoked_BeforeCutoff(t *testing.T) {
	_, rdb := newTestRedis(t)
	s := NewBlacklistService(rdb)

	assert.NoError(t, s.RevokeUserBefore(context.Background(), "user-1", 1000))

	ok, err := s.IsUserRevoked(context.Background(), "user-1", 999)
	assert.NoError(t, err)
	assert.True(t, ok)
}

// TestIsUserRevoked_AtCutoff: boundary iat == T -> masih valid (false),
// karena hanya iat < stored yang dianggap revoked.
func TestIsUserRevoked_AtCutoff(t *testing.T) {
	_, rdb := newTestRedis(t)
	s := NewBlacklistService(rdb)

	assert.NoError(t, s.RevokeUserBefore(context.Background(), "user-1", 1000))

	ok, err := s.IsUserRevoked(context.Background(), "user-1", 1000)
	assert.NoError(t, err)
	assert.False(t, ok)
}

// TestIsUserRevoked_AfterCutoff: iat = T+1 > stored T -> valid (false).
func TestIsUserRevoked_AfterCutoff(t *testing.T) {
	_, rdb := newTestRedis(t)
	s := NewBlacklistService(rdb)

	assert.NoError(t, s.RevokeUserBefore(context.Background(), "user-1", 1000))

	ok, err := s.IsUserRevoked(context.Background(), "user-1", 1001)
	assert.NoError(t, err)
	assert.False(t, ok)
}

// TestIsUserRevoked_RedisDown: Redis mati -> (false, err).
func TestIsUserRevoked_RedisDown(t *testing.T) {
	s := NewBlacklistService(deadClient(t))
	ok, err := s.IsUserRevoked(context.Background(), "user-1", 1000)
	assert.Error(t, err)
	assert.False(t, ok)
}

// TestIsUserRevoked_InvalidStoredValue: key ada tapi value bukan int64
// -> (false, err) dari strconv.ParseInt.
func TestIsUserRevoked_InvalidStoredValue(t *testing.T) {
	_, rdb := newTestRedis(t)
	s := NewBlacklistService(rdb)

	key := "user_revoke_before:user-1"
	assert.NoError(t, rdb.Set(context.Background(), key, "not-a-number", time.Hour).Err())

	ok, err := s.IsUserRevoked(context.Background(), "user-1", 1000)
	assert.Error(t, err)
	assert.False(t, ok)
}

// TestRevokeUserBefore_IsUserRevoked_Roundtrip: revoke di T=1000 lalu cek
// iat=999 -> true, iat=1000 -> false (boundary), iat=1001 -> false.
func TestRevokeUserBefore_IsUserRevoked_Roundtrip(t *testing.T) {
	_, rdb := newTestRedis(t)
	s := NewBlacklistService(rdb)

	assert.NoError(t, s.RevokeUserBefore(context.Background(), "user-1", 1000))

	ok, err := s.IsUserRevoked(context.Background(), "user-1", 999)
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = s.IsUserRevoked(context.Background(), "user-1", 1000)
	assert.NoError(t, err)
	assert.False(t, ok)

	ok, err = s.IsUserRevoked(context.Background(), "user-1", 1001)
	assert.NoError(t, err)
	assert.False(t, ok)
}
