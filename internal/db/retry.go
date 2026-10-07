// Package db — transient retry utility (TD-091, ROADMAP 04 §4.3.2 / ADR-006).
package db

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// ProcessWithRetry menjalankan fn dengan retry untuk error transient.
//
// ATURAN (SCOPE ENFORCED):
//   - isIdempotent = true  → retry allowed (SELECT / mutation with Idempotency-Key)
//   - isIdempotent = false → fail-fast, TANPA retry (mutation tanpa idempotency guard)
//
// Retry policy: 3 attempts, base 50ms exponential + jitter, per-attempt timeout 3s.
// HANYA error yang lolos IsTransient yang di-retry; error lain langsung return.
func ProcessWithRetry(ctx context.Context, fn func(ctx context.Context) error, isIdempotent bool) error {
	if !isIdempotent {
		return fn(ctx)
	}

	const (
		maxAttempts    = 3
		baseDelay      = 50 * time.Millisecond
		attemptTimeout = 3 * time.Second
		jitterMax      = 50 * time.Millisecond
	)

	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		lastErr = fn(attemptCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		if !IsTransient(lastErr) {
			return lastErr
		}
		// Backoff sebelum attempt berikutnya (kecuali attempt terakhir).
		if i < maxAttempts-1 {
			backoff := baseDelay * (1 << i)
			jitter := time.Duration(rand.Int63n(int64(jitterMax)))
			select {
			case <-time.After(backoff + jitter):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return fmt.Errorf("db: retry exhausted after %d attempts: %w", maxAttempts, lastErr)
}

// IsTransient mengklasifikasi apakah error layak di-retry.
// Transient (retry-able):
//   - 08xxx  — connection exception (08000, 08003, 08006, 08001, 08004, 08007, 08P01)
//   - 40001  — serialization failure
//   - 40P01  — deadlock detected
//   - 53300  — too many connections
//   - 57P01/57P02/57P03 — admin shutdown / crash / cannot connect
//   - io.EOF, io.ErrUnexpectedEOF — koneksi putus
//   - net.Error Timeout
//
// NOT transient: 22xxx, 23xxx, 42xxx, 55P03 (lock NOWAIT = by design fail-fast).
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	// context errors: BUKAN transient (client cancel/timeout)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return isTransientSQLState(pgErr.Code)
	}
	return false
}

// isTransientSQLState return true untuk SQLSTATE yang layak di-retry.
func isTransientSQLState(code string) bool {
	if code == "" {
		return false
	}
	if strings.HasPrefix(code, "08") {
		return true // connection exception
	}
	switch code {
	case "40001", "40P01", "53300", "57P01", "57P02", "57P03":
		return true
	}
	return false
}
