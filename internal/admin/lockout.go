// Package admin — Lockout & 2FA Validation (Task 4.2.4).
//
// Menyediakan validasi 2FA header X-Admin-2FA-Token (simulasi untuk MVP,
// token statis admin-2fa-secret) serta penguncian akun admin setelah 3 kali
// percobaan 2FA gagal (lockout 15 menit).
//
// Penyimpanan lockout bersifat DUAL-WRITE:
//   - Redis (L1): counter percobaan (INCR, TTL) + lock flag (TTL 15 menit).
//   - PostgreSQL (L2): tabel admin_lockouts (persisten, fallback & audit).
//
// checkLockout mengevaluasi hasil Redis secara langsung (FIXED v4.23) sebelum
// fallback ke PostgreSQL, sehingga lock yang sudah ada di Redis tidak bisa
// di-bypass karena baris DB belum tercatat.
package admin

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

const (
	maxFailedAttempts = 3
	lockoutDuration    = 15 * time.Minute

	lockoutKeyPrefix     = "admin:lockout:"
	attemptKeyPrefix     = "admin:2fa_attempts:"
	redisReadTimeout     = 200 * time.Millisecond
	admin2FASecretHeader = "X-Admin-2FA-Token"
)

// defaultAdmin2FASecret adalah token 2FA statis untuk MVP (simulasi).
const defaultAdmin2FASecret = "admin-2fa-secret"

// TwoFactorValidator memvalidasi token 2FA. Dipisahkan jadi interface agar
// mudah di-override / di-mock pada pengujian.
type TwoFactorValidator interface {
	Validate(token string) bool
}

// StaticTwoFactorValidator adalah implementasi MVP yang mencocokkan token
// terhadap secret statis (admin-2fa-secret).
type StaticTwoFactorValidator struct {
	secret string
}

// NewStaticTwoFactorValidator membuat validator 2FA statis. Jika secret kosong
// maka memakai default admin-2fa-secret.
func NewStaticTwoFactorValidator(secret string) *StaticTwoFactorValidator {
	if secret == "" {
		secret = defaultAdmin2FASecret
	}
	return &StaticTwoFactorValidator{secret: secret}
}

// Validate mengembalikan true jika token cocok dengan secret.
func (v *StaticTwoFactorValidator) Validate(token string) bool {
	return token != "" && token == v.secret
}

// checkLockout memeriksa apakah admin sedang terkunci.
// Mengembalikan true jika terkunci (Redis ataupun PostgreSQL).
func (h *Handler) checkLockout(ctx context.Context, adminID uuid.UUID) (bool, error) {
	if h.redis == nil {
		return h.checkLockoutPG(ctx, adminID)
	}

	redisCtx, cancel := context.WithTimeout(ctx, redisReadTimeout)
	defer cancel()

	lockoutKey := lockoutKeyPrefix + adminID.String()
	redisLocked, err := h.redis.Get(redisCtx, lockoutKey).Result()
	// FIXED v4.23: Evaluasi hasil Redis secara langsung sebelum fallback,
	// agar lock Redis tidak bisa di-bypass saat baris DB belum tercatat.
	if err == nil && redisLocked == "1" {
		return true, nil
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		h.logger.Warn("Redis read failed/timeout, falling back to PostgreSQL for lockout check",
			"admin_id", adminID, "error", err)
	}
	return h.checkLockoutPG(ctx, adminID)
}

func (h *Handler) checkLockoutPG(ctx context.Context, adminID uuid.UUID) (bool, error) {
	var pgLocked bool
	pgErr := h.db.QueryRow(ctx,
		`SELECT locked_until > NOW() FROM admin_lockouts WHERE admin_id = $1`,
		adminID,
	).Scan(&pgLocked)
	if pgErr == nil && pgLocked {
		return true, nil
	}
	if pgErr != nil && !errors.Is(pgErr, pgx.ErrNoRows) {
		return false, pgErr
	}
	return false, nil
}

// recordFailedAttempt mencatat satu percobaan 2FA gagal dan menerapkan lockout
// jika sudah mencapai batas (3x). Dual-write Redis (INCR + TTL) dan PostgreSQL
// (UPSERT). FIXED v4.23: UPSERT dijalankan juga pada path normal agar baris
// admin pertama selalu tercipta (tidak ada silent data loss).
func (h *Handler) recordFailedAttempt(ctx context.Context, adminID uuid.UUID) (int64, error) {
	if h.redis == nil {
		return h.recordFailedAttemptPG(ctx, adminID)
	}

	attemptKey := attemptKeyPrefix + adminID.String()
	attempts, err := h.redis.Incr(ctx, attemptKey).Result()
	if err != nil {
		h.logger.Warn("Redis write failed/timeout, falling back to PostgreSQL for attempt counter",
			"admin_id", adminID, "error", err)
		return h.recordFailedAttemptPG(ctx, adminID)
	}

	// Path normal Redis: tetap UPSERT PostgreSQL agar baris persisten ada.
	if _, dbErr := h.db.Exec(ctx, `
		INSERT INTO admin_lockouts (admin_id, failed_attempts, updated_at)
		VALUES ($1, 1, NOW())
		ON CONFLICT (admin_id) DO UPDATE
			SET failed_attempts = admin_lockouts.failed_attempts + 1, updated_at = NOW()
	`, adminID); dbErr != nil {
		h.logger.Warn("PostgreSQL UPSERT failed on Redis-normal path", "admin_id", adminID, "error", dbErr)
	}

	if attempts >= maxFailedAttempts {
		lockoutKey := lockoutKeyPrefix + adminID.String()
		h.redis.Set(ctx, lockoutKey, "1", lockoutDuration)
		if _, dbErr := h.db.Exec(ctx, `
			UPDATE admin_lockouts SET locked_until = NOW() + INTERVAL '15 minutes', updated_at = NOW()
			WHERE admin_id = $1
		`, adminID); dbErr != nil {
			h.logger.Warn("PostgreSQL lockout persist failed", "admin_id", adminID, "error", dbErr)
		}
	}
	return attempts, nil
}

func (h *Handler) recordFailedAttemptPG(ctx context.Context, adminID uuid.UUID) (int64, error) {
	var currentAttempts int
	errDb := h.db.QueryRow(ctx, `
		INSERT INTO admin_lockouts (admin_id, failed_attempts, updated_at)
		VALUES ($1, 1, NOW())
		ON CONFLICT (admin_id) DO UPDATE
			SET failed_attempts = admin_lockouts.failed_attempts + 1, updated_at = NOW()
		RETURNING failed_attempts
	`, adminID).Scan(&currentAttempts)
	if errDb != nil {
		return 0, errDb
	}

	attempts := int64(currentAttempts)
	if attempts >= maxFailedAttempts {
		if _, err := h.db.Exec(ctx, `
			UPDATE admin_lockouts SET locked_until = NOW() + INTERVAL '15 minutes', updated_at = NOW()
			WHERE admin_id = $1
		`, adminID); err != nil {
			return 0, err
		}
	}
	return attempts, nil
}

// resetAttempts menghapus counter percobaan setelah 2FA berhasil (Redis + PG).
func (h *Handler) resetAttempts(ctx context.Context, adminID uuid.UUID) {
	attemptKey := attemptKeyPrefix + adminID.String()
	if h.redis != nil {
		h.redis.Del(ctx, attemptKey)
		h.redis.Del(ctx, lockoutKeyPrefix+adminID.String())
	}
	if _, err := h.db.Exec(ctx, `
		UPDATE admin_lockouts SET failed_attempts = 0, locked_until = NULL, updated_at = NOW()
		WHERE admin_id = $1
	`, adminID); err != nil {
		h.logger.Warn("reset attempts PostgreSQL failed", "admin_id", adminID, "error", err)
	}
}
