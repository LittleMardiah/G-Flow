// Types, konstanta & error untuk package worker (Task 3.7).
package worker

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Status & payment constants yang dipakai worker auto-cancel (selaras dgn
// internal/food & internal/send).
const (
	foodStatusCreated         = "CREATED"
	foodStatusCancelled       = "CANCELLED"
	sendStatusSearchingDriver = "SEARCHING_DRIVER"
	sendStatusCancelled       = "CANCELLED"

	paymentMethodWallet = "WALLET"

	// Reference type double-entry ledger untuk refund escrow.
	referenceTypeFoodRefund = "FOOD_REFUND"
	referenceTypeSendRefund = "SEND_REFUND"
)

var (
	// PollInterval adalah periode loop worker auto-cancel (1 menit).
	// Workers food & send berbagi satu loop dengan ticker yang sama.
	PollInterval = time.Minute

	// LockTTL adalah masa hidup distributed lock Redis. Lebih pendek dari
	// PollInterval agar selalu ada jeda antar sweep; cukup panjang untuk
	// menyelesaikan satu sweep (tiap transaksi dibatasi statement_timeout 3s).
	LockTTL = 50 * time.Second

	// IdempotencyPurgeInterval adalah interval pembersihan idempotency_cache
	// yang kedaluwarsa (expires_at < NOW()), dijalankan dalam worker loop (TD-002).
	IdempotencyPurgeInterval = time.Hour

	// CancellationReason EXPIRED (ROADMAP 03 §3.7).
	cancellationReasonExpired = "EXPIRED"

	// Distributed lock key Redis (SET NX) — mencegah multi-instance mengeksekusi
	// sweep yang sama secara bersamaan.
	lockKey        = "gflow:worker:auto-cancel:lock"
	lockOwnerToken = uuid.NewString()

	ErrWalletNotFound    = errors.New("wallet not found")
	ErrFoodOrderNotFound = errors.New("food order not found")
	ErrSendOrderNotFound = errors.New("send order not found")
	ErrLockTimeout       = errors.New("order lock not available (NOWAIT timeout)")
	ErrInvalidTransition = errors.New("invalid status transition for current order state")
)

// FoodOrder adalah subset baris food_orders yang dibutuhkan worker.
type FoodOrder struct {
	ID               uuid.UUID
	CustomerWalletID *uuid.UUID
	PaymentMethod    string
	Status           string
	IsRefunded       bool
	IsSettled        bool
	TotalAmount      decimal.Decimal
}

// SendOrder adalah subset baris send_orders yang dibutuhkan worker.
type SendOrder struct {
	ID             uuid.UUID
	SenderWalletID *uuid.UUID
	PaymentMethod  string
	Status         string
	IsSettled      bool
	TotalFare      decimal.Decimal
}

// FoodOrderEvent adalah baris tabel food_order_events (audit trail).
type FoodOrderEvent struct {
	OrderID     uuid.UUID
	FromStatus  *string
	ToStatus    string
	Reason      *string
	TriggeredBy *uuid.UUID
	Metadata    []byte
}

// SendOrderEvent adalah baris tabel send_order_events (audit trail).
type SendOrderEvent struct {
	OrderID     uuid.UUID
	FromStatus  *string
	ToStatus    string
	Reason      *string
	TriggeredBy *uuid.UUID
	Metadata    []byte
}
