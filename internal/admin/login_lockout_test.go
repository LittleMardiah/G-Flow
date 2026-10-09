package admin

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pola query pgxmock untuk flow lockout login (whitespace di-normalisasi
// sebelum regex dicocokkan, jadi query multi-line tetap match).
const (
	regexpLoginUserQuery      = `FROM users WHERE email`
	regexpLockoutFlagQuery    = `locked_until > NOW\(\) FROM admin_lockouts`
	regexpLockoutTTLQuery     = `SELECT locked_until FROM admin_lockouts`
	regexpFailedAttemptUPSERT = `INSERT INTO admin_lockouts`
	regexpApplyLockoutQuery   = `UPDATE admin_lockouts SET locked_until`
	regexpResetAttemptsQuery  = `UPDATE admin_lockouts SET failed_attempts`

	adminEmailPlain    = "admin@g-flow.local"
	nonAdminEmailPlain = "customer@g-flow.local"
	wrongPasswordPlain = "brute-force-guess"
)

// lockedErrorBody adalah subset response error 429 yang dipakai test.
type lockedErrorBody struct {
	Success bool `json:"success"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details struct {
			RetryAfter int `json:"retry_after"`
		} `json:"details"`
	} `json:"error"`
}

func decodeErrorBody(t *testing.T, body string) lockedErrorBody {
	t.Helper()
	var parsed lockedErrorBody
	require.NoError(t, json.Unmarshal([]byte(body), &parsed))
	return parsed
}

// pgmockRowsWithLockedUntil membuat baris admin_lockouts.locked_until untuk
// query retry_after.
func pgmockRowsWithLockedUntil(ts time.Time) *pgxmock.Rows {
	return pgxmock.NewRows([]string{"locked_until"}).AddRow(ts)
}

// mrGet membaca key miniredis; ok=false bila key tidak ada.
func mrGet(mr *miniredis.Miniredis, key string) (value string, ok bool) {
	v, err := mr.Get(key)
	if err != nil {
		return "", false
	}
	return v, true
}

func loginBody(email, password string) string {
	return `{"email":"` + email + `","password":"` + password + `"}`
}

// expectNotLocked menyiapkan ekspektasi "cek lockout -> tidak terkunci".
// Flag lock tidak ada di Redis sehingga checkLockout jatuh ke PostgreSQL.
func expectNotLocked(mDB pgxmock.PgxPoolIface) {
	mDB.ExpectQuery(regexpLockoutFlagQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"locked"}).AddRow(false))
}

// expectFailedLoginAttempt menyiapkan ekspektasi satu percobaan login dengan
// password salah: lookup user -> cek lockout (tidak terkunci) -> catat failed
// attempt (+ terapkan lockout pada attempt ke maxFailedAttempts).
func expectFailedLoginAttempt(t *testing.T, mDB pgxmock.PgxPoolIface, email, userType string, applyLock bool) {
	t.Helper()
	mDB.ExpectQuery(regexpLoginUserQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(testAdminID, email, userType, "ACTIVE", mustHash(t, testPassword)))
	expectNotLocked(mDB)
	mDB.ExpectExec(regexpFailedAttemptUPSERT).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	if applyLock {
		mDB.ExpectExec(regexpApplyLockoutQuery).
			WithArgs(pgxmock.AnyArg()).
			WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	}
}

// TestAdminLogin_LockoutAfterThreeFailedPasswords (TD-114): 3x password salah
// mengunci akun; percobaan ke-4 tetap 429 walau password-nya benar.
func TestAdminLogin_LockoutAfterThreeFailedPasswords(t *testing.T) {
	mr := miniredis.RunT(t)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	for attempt := 1; attempt <= maxFailedAttempts; attempt++ {
		expectFailedLoginAttempt(t, mDB, adminEmailPlain, "admin", attempt == maxFailedAttempts)

		h := newTestHandler(t, mDB, mr)
		w := doLogin(loginRouter(h), loginBody(adminEmailPlain, wrongPasswordPlain))

		assert.Equal(t, 401, w.Code, "attempt %d harus 401", attempt)
		assert.Contains(t, w.Body.String(), "INVALID_CREDENTIALS")
	}

	// Counter Redis = 3 dan lock flag terpasang.
	attempts, ok := mrGet(mr, attemptKeyPrefix+testAdminID.String())
	assert.True(t, ok)
	assert.Equal(t, "3", attempts)
	lockFlag, ok := mrGet(mr, lockoutKeyPrefix+testAdminID.String())
	assert.True(t, ok)
	assert.Equal(t, "1", lockFlag)

	// Percobaan ke-4 dengan password BENAR -> tetap 429 (lock tidak bisa
	// dilewati), retry_after diambil dari TTL lock Redis.
	mDB.ExpectQuery(regexpLoginUserQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(testAdminID, adminEmailPlain, "admin", "ACTIVE", mustHash(t, testPassword)))

	h := newTestHandler(t, mDB, mr)
	w := doLogin(loginRouter(h), loginBody(adminEmailPlain, testPassword))

	assert.Equal(t, 429, w.Code)
	parsed := decodeErrorBody(t, w.Body.String())
	assert.False(t, parsed.Success)
	assert.Equal(t, "ACCOUNT_LOCKED", parsed.Error.Code)
	assert.InDelta(t, int(lockoutDuration.Seconds()), parsed.Error.Details.RetryAfter, 5)
	assert.NotContains(t, w.Body.String(), "access_token")

	retryAfterHeader, err := strconv.Atoi(w.Header().Get("Retry-After"))
	require.NoError(t, err, "header Retry-After harus berisi angka detik")
	assert.InDelta(t, int(lockoutDuration.Seconds()), retryAfterHeader, 5)

	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestAdminLogin_SuccessResetsFailedAttempts: login berhasil menghapus counter
// percobaan (Redis + PostgreSQL) sehingga admin tidak terkunci karena 2x gagal
// sebelumnya.
func TestAdminLogin_SuccessResetsFailedAttempts(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.Set(attemptKeyPrefix+testAdminID.String(), "2")

	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery(regexpLoginUserQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(testAdminID, adminEmailPlain, "admin", "ACTIVE", mustHash(t, testPassword)))
	expectNotLocked(mDB)
	mDB.ExpectExec(regexpResetAttemptsQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	h := newTestHandler(t, mDB, mr)
	w := doLogin(loginRouter(h), loginBody(adminEmailPlain, testPassword))

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "access_token")

	_, stillThere := mrGet(mr, attemptKeyPrefix+testAdminID.String())
	assert.False(t, stillThere, "counter attempts harus terhapus setelah login sukses")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestAdminLogin_BelowThresholdNotLocked: 2x password salah belum mengunci —
// password benar pada percobaan ketiga tetap berhasil (threshold = 3).
func TestAdminLogin_BelowThresholdNotLocked(t *testing.T) {
	mr := miniredis.RunT(t)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	for attempt := 1; attempt < maxFailedAttempts; attempt++ {
		expectFailedLoginAttempt(t, mDB, adminEmailPlain, "admin", false)
		h := newTestHandler(t, mDB, mr)
		w := doLogin(loginRouter(h), loginBody(adminEmailPlain, wrongPasswordPlain))
		assert.Equal(t, 401, w.Code, "attempt %d harus 401", attempt)
	}
	_, locked := mrGet(mr, lockoutKeyPrefix+testAdminID.String())
	assert.False(t, locked, "counter di bawah threshold tidak boleh mengunci akun")

	// Password benar -> 200.
	mDB.ExpectQuery(regexpLoginUserQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(testAdminID, adminEmailPlain, "admin", "ACTIVE", mustHash(t, testPassword)))
	expectNotLocked(mDB)
	mDB.ExpectExec(regexpResetAttemptsQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	h := newTestHandler(t, mDB, mr)
	w := doLogin(loginRouter(h), loginBody(adminEmailPlain, testPassword))

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "access_token")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestAdminLogin_LockedAdminCannotLoginWithCorrectPassword: lock flag Redis
// sudah aktif -> password benar tetap ditolak 429 dan tidak ada token terbit.
// Retry_after dihitung dari locked_until PostgreSQL (flag Redis tanpa TTL).
func TestAdminLogin_LockedAdminCannotLoginWithCorrectPassword(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.Set(lockoutKeyPrefix+testAdminID.String(), "1")

	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	// Lock terdeteksi langsung dari flag Redis (tanpa query PostgreSQL).
	mDB.ExpectQuery(regexpLoginUserQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(testAdminID, adminEmailPlain, "admin", "ACTIVE", mustHash(t, testPassword)))
	// Flag Redis tanpa TTL -> retry_after fallback ke locked_until PostgreSQL.
	mDB.ExpectQuery(regexpLockoutTTLQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgmockRowsWithLockedUntil(time.Now().Add(7 * time.Minute)))

	h := newTestHandler(t, mDB, mr)
	w := doLogin(loginRouter(h), loginBody(adminEmailPlain, testPassword))

	assert.Equal(t, 429, w.Code)
	parsed := decodeErrorBody(t, w.Body.String())
	assert.Equal(t, "ACCOUNT_LOCKED", parsed.Error.Code)
	assert.InDelta(t, 420, parsed.Error.Details.RetryAfter, 5)
	assert.NotContains(t, w.Body.String(), "access_token")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestAdminLogin_LockoutExpiresAfterTTL: setelah TTL lock Redis habis
// (FastForward > lockoutDuration) admin boleh login lagi dengan password benar.
func TestAdminLogin_LockoutExpiresAfterTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.Set(lockoutKeyPrefix+testAdminID.String(), "1")
	mr.SetTTL(lockoutKeyPrefix+testAdminID.String(), lockoutDuration)
	mr.Set(attemptKeyPrefix+testAdminID.String(), "3")

	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery(regexpLoginUserQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(testAdminID, adminEmailPlain, "admin", "ACTIVE", mustHash(t, testPassword)))
	expectNotLocked(mDB)
	mDB.ExpectExec(regexpResetAttemptsQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	// Majukan clock miniredis melewati TTL lock.
	mr.FastForward(lockoutDuration + time.Minute)

	h := newTestHandler(t, mDB, mr)
	w := doLogin(loginRouter(h), loginBody(adminEmailPlain, testPassword))

	assert.Equal(t, 200, w.Code, "lockout harus kedaluwarsa setelah TTL")
	assert.Contains(t, w.Body.String(), "access_token")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestAdminLogin_LockoutFallbackToPostgresWhenRedisDown: Redis mati ->
// checkLockout fallback ke PostgreSQL (admin_lockouts) dan retry_after diambil
// dari locked_until.
func TestAdminLogin_LockoutFallbackToPostgresWhenRedisDown(t *testing.T) {
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = rdb.Close() })

	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery(regexpLoginUserQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(testAdminID, adminEmailPlain, "admin", "ACTIVE", mustHash(t, testPassword)))
	// Redis down -> status lock diambil dari PostgreSQL.
	mDB.ExpectQuery(regexpLockoutFlagQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"locked"}).AddRow(true))
	mDB.ExpectQuery(regexpLockoutTTLQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgmockRowsWithLockedUntil(time.Now().Add(10 * time.Minute)))

	svc := NewService(NewRepository(mDB), mDB, testLogger(), nil, nil)
	h := NewHandler(svc, mDB, rdb, testLogger(), NewStaticTwoFactorValidator(""), newTestJWT())
	w := doLogin(loginRouter(h), loginBody(adminEmailPlain, testPassword))

	assert.Equal(t, 429, w.Code)
	parsed := decodeErrorBody(t, w.Body.String())
	assert.Equal(t, "ACCOUNT_LOCKED", parsed.Error.Code)
	assert.InDelta(t, 600, parsed.Error.Details.RetryAfter, 5)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestAdminLogin_NonAdminAccountAlsoLocked: lockout TIDAK dibatasi user_type
// admin. Kalau hanya admin yang dikunci, response 429 menjadi oracle
// "email ini pasti admin" (akun lain selalu 401) — jadi akun non-admin juga
// dikunci agar respons login tetap seragam.
func TestAdminLogin_NonAdminAccountAlsoLocked(t *testing.T) {
	mr := miniredis.RunT(t)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	for attempt := 1; attempt <= maxFailedAttempts; attempt++ {
		expectFailedLoginAttempt(t, mDB, nonAdminEmailPlain, "customer", attempt == maxFailedAttempts)
		h := newTestHandler(t, mDB, mr)
		w := doLogin(loginRouter(h), loginBody(nonAdminEmailPlain, wrongPasswordPlain))
		assert.Equal(t, 401, w.Code, "attempt %d harus 401", attempt)
	}

	mDB.ExpectQuery(regexpLoginUserQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(testAdminID, nonAdminEmailPlain, "customer", "ACTIVE", mustHash(t, testPassword)))

	h := newTestHandler(t, mDB, mr)
	w := doLogin(loginRouter(h), loginBody(nonAdminEmailPlain, testPassword))

	assert.Equal(t, 429, w.Code)
	parsed := decodeErrorBody(t, w.Body.String())
	assert.Equal(t, "ACCOUNT_LOCKED", parsed.Error.Code)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestServiceLogin_NoLockoutGuard: Service yang dibuat tanpa Handler (guard
// nil, pemakaian langsung di internal) tetap login normal dan tidak menyentuh
// tabel admin_lockouts.
func TestServiceLogin_NoLockoutGuard(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery(regexpLoginUserQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(testAdminID, adminEmailPlain, "admin", "ACTIVE", mustHash(t, testPassword)))

	svc := NewService(NewRepository(mDB), mDB, testLogger(), nil, nil)
	res, err := svc.Login(context.Background(), adminEmailPlain, testPassword)

	require.NoError(t, err)
	assert.Equal(t, testAdminID, res.UserID)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestAdminLogin_AttemptCounterHasTTL (TD-154): counter percobaan wajib punya
// TTL 15 menit. Tanpa EXPIRE key hidup selamanya dan policy efektif menjadi
// "3x gagal sejak reset terakhir", bukan "3x dalam 15 menit".
func TestAdminLogin_AttemptCounterHasTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	attemptKey := attemptKeyPrefix + testAdminID.String()

	// Percobaan pertama -> INCR return 1 -> EXPIRE 15 menit di-set.
	expectFailedLoginAttempt(t, mDB, adminEmailPlain, "admin", false)
	h := newTestHandler(t, mDB, mr)
	w := doLogin(loginRouter(h), loginBody(adminEmailPlain, wrongPasswordPlain))
	assert.Equal(t, 401, w.Code, "attempt 1 harus 401")

	_, ok := mrGet(mr, attemptKey)
	require.True(t, ok, "counter attempts harus ter-set setelah 1x gagal")
	ttl := mr.TTL(attemptKey)
	assert.GreaterOrEqual(t, ttl, 14*time.Minute, "TTL counter harus mendekati 15 menit")
	assert.LessOrEqual(t, ttl, 15*time.Minute, "TTL counter tidak boleh melebihi 15 menit")

	// Lewati jendela 15 menit -> counter expired (key hilang) dan admin
	// bisa login lagi.
	mr.FastForward(lockoutDuration + time.Second)
	_, stillThere := mrGet(mr, attemptKey)
	assert.False(t, stillThere, "counter harus expired setelah jendela 15 menit")

	mDB.ExpectQuery(regexpLoginUserQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(testAdminID, adminEmailPlain, "admin", "ACTIVE", mustHash(t, testPassword)))
	expectNotLocked(mDB)
	mDB.ExpectExec(regexpResetAttemptsQuery).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	h = newTestHandler(t, mDB, mr)
	w = doLogin(loginRouter(h), loginBody(adminEmailPlain, testPassword))
	assert.Equal(t, 200, w.Code, "admin harus bisa login lagi setelah counter expired")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestAdminLogin_AttemptCounterFixedWindow (TD-154): jendela counter bersifat
// FIXED, bukan sliding. Attempt ke-2 tidak me-reset TTL sehingga attacker
// tidak bisa memperpanjang window dengan terus-menerus INCR.
func TestAdminLogin_AttemptCounterFixedWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	attemptKey := attemptKeyPrefix + testAdminID.String()

	// Attempt 1 -> TTL 15 menit.
	expectFailedLoginAttempt(t, mDB, adminEmailPlain, "admin", false)
	h := newTestHandler(t, mDB, mr)
	w := doLogin(loginRouter(h), loginBody(adminEmailPlain, wrongPasswordPlain))
	assert.Equal(t, 401, w.Code, "attempt 1 harus 401")
	ttl1 := mr.TTL(attemptKey)
	assert.GreaterOrEqual(t, ttl1, 14*time.Minute, "attempt 1 harus set TTL 15 menit")
	assert.LessOrEqual(t, ttl1, 15*time.Minute, "TTL attempt 1 tidak boleh > 15 menit")

	// Majukan 10 menit -> sisa jendela 5 menit.
	mr.FastForward(10 * time.Minute)
	ttlMid := mr.TTL(attemptKey)
	assert.GreaterOrEqual(t, ttlMid, 4*time.Minute, "sisa jendela setelah 10 menit harus ~5 menit")
	assert.LessOrEqual(t, ttlMid, 5*time.Minute, "sisa jendela setelah 10 menit harus ~5 menit")

	// Attempt 2 -> TTL JANGAN di-reset (harus tetap sisa ~5 menit, bukan 15 menit).
	expectFailedLoginAttempt(t, mDB, adminEmailPlain, "admin", false)
	h = newTestHandler(t, mDB, mr)
	w = doLogin(loginRouter(h), loginBody(adminEmailPlain, wrongPasswordPlain))
	assert.Equal(t, 401, w.Code, "attempt 2 harus 401")

	attempts, ok := mrGet(mr, attemptKey)
	require.True(t, ok, "counter harus ada di attempt 2")
	assert.Equal(t, "2", attempts, "counter harus 2 setelah dua kegagalan")
	ttl2 := mr.TTL(attemptKey)
	assert.Greater(t, ttl2, 4*time.Minute, "TTL tidak boleh di-reset ke 15 menit (fixed window)")
	assert.LessOrEqual(t, ttl2, 5*time.Minute, "TTL harus sisa jendela (~5 menit)")

	// Lewati sisa jendela -> counter expired. Percobaan ke-3 = jendela FRESH
	// (counter mulai dari 1), jadi TIDAK boleh mengunci.
	mr.FastForward(5*time.Minute + time.Second)
	_, stillThere := mrGet(mr, attemptKey)
	require.False(t, stillThere, "counter harus expired setelah jendela asli habis")

	expectFailedLoginAttempt(t, mDB, adminEmailPlain, "admin", false)
	h = newTestHandler(t, mDB, mr)
	w = doLogin(loginRouter(h), loginBody(adminEmailPlain, wrongPasswordPlain))
	assert.Equal(t, 401, w.Code, "attempt setelah expired harus 401 (fresh window)")

	fresh, ok := mrGet(mr, attemptKey)
	require.True(t, ok, "counter baru dibuat dengan jendela fresh")
	assert.Equal(t, "1", fresh, "counter harus mulai dari 1 lagi, bukan 3 (lockout)")
	_, locked := mrGet(mr, lockoutKeyPrefix+testAdminID.String())
	assert.False(t, locked, "jendela fresh belum boleh mengunci akun")
	assert.NoError(t, mDB.ExpectationsWereMet())
}
