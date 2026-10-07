package db

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// timeoutErr implements net.Error with Timeout() == true.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

// permanentNetErr implements net.Error with Timeout() == false.
type permanentNetErr struct{}

func (permanentNetErr) Error() string   { return "connection refused" }
func (permanentNetErr) Timeout() bool   { return false }
func (permanentNetErr) Temporary() bool { return false }

func transientErr() error {
	return &pgconn.PgError{Code: "40001", Message: "serialization failure"}
}

const (
	kindSuccess          = "success"
	kindTransientThenOK  = "transient_then_success"
	kindAlwaysTransient  = "always_transient"
	kindNonTransient     = "non_transient"
	kindCancelDuringCall = "cancel_during_call"
)

func TestProcessWithRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		isIdempotent  bool
		kind          string
		checkDeadline bool
		wantCalls     int
		wantErr       string // substring; "" = expect nil
		wantCtxErr    error  // if non-nil, assert errors.Is
	}{
		{
			name:         "non_idempotent_fail_fast_no_retry",
			isIdempotent: false,
			kind:         kindAlwaysTransient,
			wantCalls:    1,
			wantErr:      "serialization failure",
		},
		{
			name:         "idempotent_success_first_attempt",
			isIdempotent: true,
			kind:         kindSuccess,
			wantCalls:    1,
		},
		{
			name:         "idempotent_transient_twice_then_success",
			isIdempotent: true,
			kind:         kindTransientThenOK,
			wantCalls:    3,
		},
		{
			name:         "idempotent_non_transient_fail_no_retry",
			isIdempotent: true,
			kind:         kindNonTransient,
			wantCalls:    1,
			wantErr:      "syntax error",
		},
		{
			name:         "idempotent_transient_exhausted",
			isIdempotent: true,
			kind:         kindAlwaysTransient,
			wantCalls:    3,
			wantErr:      "retry exhausted",
		},
		{
			name:         "context_canceled_during_backoff",
			isIdempotent: true,
			kind:         kindCancelDuringCall,
			wantCalls:    1,
			wantCtxErr:   context.Canceled,
		},
		{
			name:          "per_attempt_timeout_set",
			isIdempotent:  true,
			kind:          kindSuccess,
			checkDeadline: true,
			wantCalls:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calls := 0
			parentCtx, cancel := context.WithCancel(context.Background())
			defer cancel()

			fn := func(ctx context.Context) error {
				calls++
				if tt.checkDeadline {
					d, ok := ctx.Deadline()
					if !ok {
						t.Error("attempt ctx has no deadline")
					} else if remaining := time.Until(d); remaining <= 0 || remaining > 3*time.Second+200*time.Millisecond {
						t.Errorf("attempt deadline out of range: remaining=%v", remaining)
					}
				}
				switch tt.kind {
				case kindSuccess:
					return nil
				case kindTransientThenOK:
					if calls < 3 {
						return transientErr()
					}
					return nil
				case kindAlwaysTransient:
					return transientErr()
				case kindNonTransient:
					return errors.New("syntax error")
				case kindCancelDuringCall:
					cancel() // parent ctx dies while ProcessWithRetry is in backoff
					return transientErr()
				default:
					t.Fatalf("unknown kind %q", tt.kind)
					return nil
				}
			}

			err := ProcessWithRetry(parentCtx, fn, tt.isIdempotent)

			if calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", calls, tt.wantCalls)
			}
			if tt.wantCtxErr != nil {
				if !errors.Is(err, tt.wantCtxErr) {
					t.Errorf("err = %v, want errors.Is(%v)", err, tt.wantCtxErr)
				}
				return
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("err = nil, want containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestIsTransient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"context_canceled", context.Canceled, false},
		{"context_deadline_exceeded", context.DeadlineExceeded, false},
		{"io_eof", io.EOF, true},
		{"io_unexpected_eof", io.ErrUnexpectedEOF, true},
		{"pg_08006", &pgconn.PgError{Code: "08006"}, true},
		{"pg_08000", &pgconn.PgError{Code: "08000"}, true},
		{"pg_08P01", &pgconn.PgError{Code: "08P01"}, true},
		{"pg_40001", &pgconn.PgError{Code: "40001"}, true},
		{"pg_40P01", &pgconn.PgError{Code: "40P01"}, true},
		{"pg_53300", &pgconn.PgError{Code: "53300"}, true},
		{"pg_57P01", &pgconn.PgError{Code: "57P01"}, true},
		{"pg_57P03", &pgconn.PgError{Code: "57P03"}, true},
		{"pg_23505", &pgconn.PgError{Code: "23505"}, false},
		{"pg_42P01", &pgconn.PgError{Code: "42P01"}, false},
		{"pg_55P03", &pgconn.PgError{Code: "55P03"}, false},
		{"pg_empty_code", &pgconn.PgError{Code: ""}, false},
		{"wrapped_pg_error", errors.Join(errors.New("ctx wrapper"), &pgconn.PgError{Code: "40P01"}), true},
		{"net_timeout", timeoutErr{}, true},
		{"net_no_timeout", permanentNetErr{}, false},
		{"generic", errors.New("generic failure"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsTransient(tt.err); got != tt.want {
				t.Errorf("IsTransient(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsTransientSQLState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code string
		want bool
	}{
		{"", false},
		{"08000", true},
		{"08003", true},
		{"08P01", true},
		{"40001", true},
		{"40P01", true},
		{"53300", true},
		{"57P01", true},
		{"57P02", true},
		{"57P03", true},
		{"23505", false},
		{"42P01", false},
		{"55P03", false},
		{"00000", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			t.Parallel()
			if got := isTransientSQLState(tt.code); got != tt.want {
				t.Errorf("isTransientSQLState(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}
