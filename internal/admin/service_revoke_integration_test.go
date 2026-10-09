//go:build integration

// Integration test TD-174 STEP D-FIX — race iat == cutoff.
//
// Test memakai time.Now() NYATA (bukan fixed untilUnix): login menghasilkan
// token ber-iat T, lalu suspend di detik YANG SAMA memicu
// RevokeUserBefore(uid, T+1).
//   - Sebelum fix (cutoff = T): IsUserRevoked(iat=T) = (T < T) = false -> token lolos (bug).
//   - Sesudah fix (cutoff = T+1): (T < T+1) = true -> token ter-revoke.
//
// Setup: miniredis + BlacklistService nyata + JWTService nyata + admin.Service
// nyata (repo memakai pgxmock untuk transaksi). Jalankan dengan:
//
//	go test -tags integration ./internal/admin/ -run TestIntegrationAdmin -v -count=1
package admin

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/g-flow/g-flow/internal/auth"
)

// TestIntegrationAdmin_RevokeUserBefore_ImmediateRevoke: login customer dan
// suspend admin terjadi di detik yang sama (T). Setelah UpdateUserStatus
// (suspend), token ber-iat T WAJIB dianggap revoked. Ini test yang GAGAL
// sebelum fix (cutoff = T -> T < T false) dan PASS setelah fix (cutoff = T+1).
func TestIntegrationAdmin_RevokeUserBefore_ImmediateRevoke(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	blacklist := auth.NewBlacklistService(rdb)
	jwtSvc := auth.NewJWTService("td174-step-d-fix-test-secret")

	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	svc := NewService(
		NewRepository(mDB),
		mDB,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		blacklist,
		nil,
	)

	ctx := context.Background()
	userID := uuid.New()
	key := "user_revoke_before:" + userID.String()

	// Reproduksi race sesungguhnya: jajar ke awal detik supaya GenerateToken
	// (iat = T) dan UpdateUserStatus (cutoff = T+1) jatuh di detik yang sama.
	// Kalau detik berganti di tengah, ulangi (max 10 percobaan).
	var iat int64
	aligned := false
	for attempt := 0; attempt < 10 && !aligned; attempt++ {
		now := time.Now()
		sleep := time.Until(time.Unix(now.Unix()+1, 0)) + 20*time.Millisecond
		if sleep > 0 && sleep < 2*time.Second {
			time.Sleep(sleep)
		}

		accessToken, _, err := jwtSvc.GenerateToken(userID, "itest@example.com", "customer")
		require.NoError(t, err)
		claims, err := jwtSvc.ValidateToken(accessToken)
		require.NoError(t, err)
		iat = claims.Iat

		before := time.Now().Unix()
		expectUpdateUserStatusSetup(mDB)
		res, err := svc.UpdateUserStatus(ctx, UserStatusRequest{
			UserID:  userID,
			AdminID: testAdminID,
			Action:  "suspend",
		})
		require.NoError(t, err)
		require.Equal(t, "SUSPENDED", res.Status)
		after := time.Now().Unix()

		if before == iat && after == iat {
			aligned = true
		}
	}
	require.True(t, aligned,
		"login & suspend harus berada di detik yang sama (T) untuk mereproduksi race iat == cutoff")

	// Bukti cutoff: key Redis ada dan bernilai T+1 (grace 1 detik).
	raw, err := mr.Get(key)
	require.NoError(t, err, "key revoke harus dibuat di Redis")
	cutoff, err := strconv.ParseInt(raw, 10, 64)
	require.NoError(t, err)
	require.Equal(t, iat+1, cutoff, "cutoff harus T+1 (grace 1 detik), bukan T")
	require.Equal(t, iat, cutoff-1)

	// ASSERT INTI: token iat = T harus ter-revoke setelah suspend.
	// Sebelum fix ini FALSE (T < T); sesudah fix TRUE (T < T+1).
	revoked, err := blacklist.IsUserRevoked(ctx, userID.String(), iat)
	require.NoError(t, err)
	assert.True(t, revoked,
		"token iat=T harus revoked setelah suspend di detik yang sama (cutoff=T+1)")

	require.NoError(t, mDB.ExpectationsWereMet())
}
