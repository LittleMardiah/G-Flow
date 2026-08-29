// Package wallet — Service Layer (F002 + F003).
//
// Service menangani business logic untuk Top-Up dan Transfer P2P dengan
// dual-layer idempotency (Redis L1 + PostgreSQL L2), locking deadlock-free
// (ORDER BY id ASC FOR UPDATE), dan double-entry ledger.
//
// Prinsip kunci:
//   - Semua operasi finansial berjalan dalam satu transaksi DB.
//   - Semua multi-wallet op mengunci wallet dengan urutan deterministik
//     (ORDER BY id ASC) untuk mencegah deadlock.
//   - Trigger DB `sync_wallet_balance` yang memutakhirkan saldo; service
//     TIDAK memanggil UpdateBalance untuk operasi ledger.
//   - SET LOCAL statement_timeout = '3000ms' di setiap transaksi.
package wallet

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// Wallet type constants.
const (
	WalletTypeCustomer          = "CUSTOMER"
	WalletTypeSystemBankGateway = "SYSTEM_BANK_GATEWAY"
	WalletTypeSystemPlatform    = "SYSTEM_PLATFORM"

	referenceTypeTopUp             = "TOPUP"
	referenceTypeTransfer          = "TRANSFER"
	referenceTypeOverdueSettlement = "OVERDUE_SETTLEMENT"

	redisKeyPrefix = "idempotency:"
	redisCompleted = "COMPLETED"
	pgProcessing   = "PROCESSING"

	redisTTL = 24 * time.Hour

	kycUnverified = "UNVERIFIED"
	kycVerified   = "VERIFIED"
)

// Error definitions untuk service layer.
var (
	ErrInvalidAmount         = errors.New("amount must be between 1000 and 50.000.000")
	ErrIdempotencyInProgress = errors.New("idempotency key is still processing")
	ErrKycLimitExceeded      = errors.New("top-up would exceed KYC balance limit")
	ErrWalletInactive        = errors.New("wallet is not ACTIVE")
	ErrInsufficientBalance   = errors.New("insufficient balance")
	ErrSameWalletTransfer    = errors.New("from and to wallet cannot be the same")
	ErrWalletNotOwned        = errors.New("wallet is not owned by the authenticated user")
	ErrInvalidCachedResponse = errors.New("cached idempotency response is invalid")
)

// amount bounds dan limit KYC.
var (
	topUpMin = decimal.NewFromInt(1000)
	topUpMax = decimal.NewFromInt(50000000)

	kycLimitUnverified = decimal.NewFromInt(2000000)
	kycLimitVerified   = decimal.NewFromInt(20000000)
)

// TopUpRequest input untuk operasi top-up.
type TopUpRequest struct {
	UserID         uuid.UUID
	Amount         decimal.Decimal
	IdempotencyKey string
}

// TopUpResponse hasil operasi top-up.
type TopUpResponse struct {
	TransactionID  uuid.UUID       `json:"transaction_id"`
	WalletID       uuid.UUID       `json:"wallet_id"`
	Amount         decimal.Decimal `json:"amount"`
	DebtSettled    decimal.Decimal `json:"debt_settled"`    // Bug #47: dipakai bayar overdue debt
	WalletCredited decimal.Decimal `json:"wallet_credited"` // Bug #47: net_topup ke wallet
	NewBalance     decimal.Decimal `json:"new_balance"`
	Status         string          `json:"status"`
}

// TransferRequest input untuk operasi transfer P2P.
type TransferRequest struct {
	UserID         uuid.UUID
	FromWalletID   uuid.UUID
	ToWalletID     uuid.UUID
	Amount         decimal.Decimal
	IdempotencyKey string
	Description    string
}

// TransferResponse hasil operasi transfer.
type TransferResponse struct {
	TransferID  uuid.UUID       `json:"transfer_id"`
	FromBalance decimal.Decimal `json:"from_balance"`
	ToBalance   decimal.Decimal `json:"to_balance"`
}

// WalletRepo adalah kontrak repository yang dibutuhkan Service.
// Dipenuhi oleh *Repository (internal/wallet/repository.go); dijadikan
// interface agar mudah di-mock pada unit test.
type WalletRepo interface {
	GetByUserIDAndType(ctx context.Context, userID uuid.UUID, walletType string) (*Wallet, error)
	GetByID(ctx context.Context, walletID uuid.UUID) (*Wallet, error)
	GetBalance(ctx context.Context, walletID uuid.UUID) (decimal.Decimal, error)
}

// Ledger adalah kontrak double-entry ledger yang dibutuhkan Service.
// Dipenuhi oleh *LedgerService (internal/wallet/ledger.go).
type Ledger interface {
	CreateLedgerEntries(ctx context.Context, tx pgx.Tx, entries []LedgerEntry) error
}

// DB adalah subset operasi pool yang dipakai Service. Dipenuhi oleh
// *pgxpool.Pool (produksi) dan pgxmock (test).
type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Service adalah business logic untuk modul wallet.
type Service struct {
	repo   WalletRepo
	ledger Ledger
	redis  *redis.Client
	db     DB
}

// NewService membuat Service baru dengan dependency injection.
func NewService(repo WalletRepo, ledger Ledger, rdb *redis.Client, db DB) *Service {
	return &Service{repo: repo, ledger: ledger, redis: rdb, db: db}
}

// TopUp melakukan pengisian saldo customer wallet melalui gateway.
// Flow: validate -> L1 Redis -> wallet & KYC -> L2 PG -> tx DB
// (lock -> ledger -> topup_transactions -> webhook) -> cache -> response.
func (s *Service) TopUp(ctx context.Context, req TopUpRequest) (*TopUpResponse, error) {
	if req.Amount.LessThan(topUpMin) || req.Amount.GreaterThan(topUpMax) {
		return nil, ErrInvalidAmount
	}
	if req.IdempotencyKey == "" {
		return nil, errors.New("idempotency key is required")
	}

	// L1: Redis (scope per user agar tidak collision antar user).
	if resp, ok := s.redisGetCachedResp(ctx, req.UserID, req.IdempotencyKey); ok {
		var out TopUpResponse
		if err := json.Unmarshal(resp, &out); err != nil {
			return nil, ErrInvalidCachedResponse
		}
		return &out, nil
	}

	// Wallet customer
	customerWallet, err := s.repo.GetByUserIDAndType(ctx, req.UserID, WalletTypeCustomer)
	if err != nil {
		return nil, err
	}

	// Overdue debt settlement (Bug #47): hutang dilunasi dulu sebelum saldo
	// top-up masuk ke wallet. Hanya sisanya (net_topup) yang dikreditkan.
	overdueDebt, err := s.overdueDebtFor(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	debtSettled, netTopUp := settleOverdueDebt(req.Amount, overdueDebt)

	// KYC limit berdasarkan kyc_status. Batas diperiksa terhadap net_topup
	// (bagian yang benar-benar masuk ke wallet), bukan amount bruto.
	limit, err := s.kycLimitFor(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	if customerWallet.Balance.Add(netTopUp).GreaterThan(limit) {
		return nil, ErrKycLimitExceeded
	}

	// L2: PostgreSQL idempotency.
	if res, err := s.idemAcquire(ctx, req.UserID, req.IdempotencyKey); err != nil {
		return nil, err
	} else if !res.proceed {
		s.redisSet(ctx, req.UserID, req.IdempotencyKey, redisCompleted, res.cached)
		var out TopUpResponse
		if err := json.Unmarshal(res.cached, &out); err != nil {
			return nil, ErrInvalidCachedResponse
		}
		return &out, nil
	}

	txnID := uuid.New()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return nil, err
	}

	// Ambil system bank gateway wallet (identitas statis, user_id IS NULL).
	bankID, err := s.systemWalletID(ctx, tx, WalletTypeSystemBankGateway)
	if err != nil {
		return nil, err
	}

	// Saat ada hutang, SYSTEM_PLATFORM juga terlibat sebagai CREDIT side.
	lockIDs := []uuid.UUID{customerWallet.ID, bankID}
	var platformID uuid.UUID
	if debtSettled.IsPositive() {
		platformID, err = s.systemWalletID(ctx, tx, WalletTypeSystemPlatform)
		if err != nil {
			return nil, err
		}
		lockIDs = append(lockIDs, platformID)
	}

	// Lock semua wallet dengan urutan deterministik (ORDER BY id ASC).
	if err := lockWalletsAsc(ctx, tx, lockIDs...); err != nil {
		return nil, err
	}

	// Double-entry ledger:
	//   - OVERDUE_SETTLEMENT: DEBIT customer wallet, CREDIT SYSTEM_PLATFORM.
	//   - TOPUP (net_topup): DEBIT SYSTEM_BANK_GATEWAY, CREDIT customer wallet.
	entries := make([]LedgerEntry, 0, 4)
	if debtSettled.IsPositive() {
		entries = append(entries,
			LedgerEntry{
				WalletID:      customerWallet.ID,
				EntryType:     EntryDebit,
				Amount:        debtSettled,
				ReferenceID:   txnID,
				ReferenceType: referenceTypeOverdueSettlement,
				Description:   "OVERDUE_SETTLEMENT - DEBIT customer wallet",
			},
			LedgerEntry{
				WalletID:      platformID,
				EntryType:     EntryCredit,
				Amount:        debtSettled,
				ReferenceID:   txnID,
				ReferenceType: referenceTypeOverdueSettlement,
				Description:   "OVERDUE_SETTLEMENT - CREDIT SYSTEM_PLATFORM",
			},
		)
	}
	if netTopUp.IsPositive() {
		entries = append(entries,
			LedgerEntry{
				WalletID:      bankID,
				EntryType:     EntryDebit,
				Amount:        netTopUp,
				ReferenceID:   txnID,
				ReferenceType: referenceTypeTopUp,
				Description:   "TOPUP - DEBIT SYSTEM_BANK_GATEWAY",
			},
			LedgerEntry{
				WalletID:      customerWallet.ID,
				EntryType:     EntryCredit,
				Amount:        netTopUp,
				ReferenceID:   txnID,
				ReferenceType: referenceTypeTopUp,
				Description:   "TOPUP - CREDIT CUSTOMER wallet",
			},
		)
	}
	if err := s.ledger.CreateLedgerEntries(ctx, tx, entries); err != nil {
		return nil, err
	}

	// Update overdue_debt: 0 (hutang lunas) atau sisa hutang (Bug #47).
	if debtSettled.IsPositive() {
		remaining := overdueDebt.Sub(debtSettled)
		if _, err := tx.Exec(ctx, `
			UPDATE users SET overdue_debt = $1, updated_at = NOW() WHERE id = $2
		`, remaining, req.UserID); err != nil {
			return nil, err
		}
	}

	// Insert topup_transactions PENDING (net_amount = jumlah yang masuk wallet).
	if _, err := tx.Exec(ctx, `
		INSERT INTO topup_transactions
			(id, user_id, wallet_id, amount, net_amount, payment_method, status)
		VALUES ($1, $2, $3, $4, $5, 'SIMULATED', 'PENDING')
	`, txnID, req.UserID, customerWallet.ID, req.Amount, netTopUp); err != nil {
		return nil, err
	}

	// Simulasi webhook: tandai COMPLETED (idempotent guard) dalam tx yang sama.
	if err := s.markTopUpCompleted(ctx, tx, txnID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	newBalance, err := s.repo.GetBalance(ctx, customerWallet.ID)
	if err != nil {
		return nil, err
	}

	resp := &TopUpResponse{
		TransactionID:  txnID,
		WalletID:       customerWallet.ID,
		Amount:         req.Amount,
		DebtSettled:    debtSettled,
		WalletCredited: netTopUp,
		NewBalance:     newBalance,
		Status:         redisCompleted,
	}

	// Cache response (Redis L1 + update PG L2 COMPLETED).
	if err := s.cacheResponse(ctx, req.UserID, req.IdempotencyKey, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// Transfer melakukan transfer P2P antar dua wallet.
// Flow: validate -> load wallets -> L1 Redis -> L2 PG -> tx DB
// (lock asc -> balance check -> ledger) -> cache -> response.
//
// Catatan: idempotency L2 diverifikasi setelah wallet dimuat agar bisa
// mendapatkan owner (user_id dari from-wallet) untuk FK idempotency_cache.
func (s *Service) Transfer(ctx context.Context, req TransferRequest) (*TransferResponse, error) {
	if req.FromWalletID == req.ToWalletID {
		return nil, ErrSameWalletTransfer
	}
	if !req.Amount.IsPositive() {
		return nil, ErrInvalidAmount
	}
	if req.IdempotencyKey == "" {
		return nil, errors.New("idempotency key is required")
	}

	// Load kedua wallet (dibutuhkan untuk owner idempotency & validasi status).
	from, err := s.repo.GetByID(ctx, req.FromWalletID)
	if err != nil {
		return nil, err
	}
	to, err := s.repo.GetByID(ctx, req.ToWalletID)
	if err != nil {
		return nil, err
	}
	if from.Status != "ACTIVE" || to.Status != "ACTIVE" {
		return nil, ErrWalletInactive
	}

	// Hanya pemilik wallet (JWT claim) yang boleh transfer dari wallet tsb.
	if from.UserID != req.UserID {
		return nil, ErrWalletNotOwned
	}

	// L1: Redis (scope per user).
	if resp, ok := s.redisGetCachedResp(ctx, from.UserID, req.IdempotencyKey); ok {
		var out TransferResponse
		if err := json.Unmarshal(resp, &out); err != nil {
			return nil, ErrInvalidCachedResponse
		}
		return &out, nil
	}

	// L2: PostgreSQL idempotency (owner = user pemilik from-wallet).
	if res, err := s.idemAcquire(ctx, from.UserID, req.IdempotencyKey); err != nil {
		return nil, err
	} else if !res.proceed {
		s.redisSet(ctx, from.UserID, req.IdempotencyKey, redisCompleted, res.cached)
		var out TransferResponse
		if err := json.Unmarshal(res.cached, &out); err != nil {
			return nil, ErrInvalidCachedResponse
		}
		return &out, nil
	}

	transferID := uuid.New()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return nil, err
	}

	// Deadlock-free: lock kedua wallet urutan ascending lalu reload balance.
	lockedFrom, lockedTo, err := lockAndReloadWallets(ctx, tx, req.FromWalletID, req.ToWalletID)
	if err != nil {
		return nil, err
	}

	if lockedFrom.Balance.LessThan(req.Amount) {
		return nil, ErrInsufficientBalance
	}

	// Double-entry ledger.
	desc := req.Description
	entries := []LedgerEntry{
		{
			WalletID:      req.FromWalletID,
			EntryType:     EntryDebit,
			Amount:        req.Amount,
			ReferenceID:   transferID,
			ReferenceType: referenceTypeTransfer,
			Description:   "TRANSFER - from wallet",
		},
		{
			WalletID:      req.ToWalletID,
			EntryType:     EntryCredit,
			Amount:        req.Amount,
			ReferenceID:   transferID,
			ReferenceType: referenceTypeTransfer,
			Description:   "TRANSFER - to wallet",
		},
	}
	if desc != "" {
		entries[0].Description = "TRANSFER - " + desc
		entries[1].Description = "TRANSFER - " + desc
	}
	if err := s.ledger.CreateLedgerEntries(ctx, tx, entries); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	resp := &TransferResponse{
		TransferID:  transferID,
		FromBalance: lockedFrom.Balance.Sub(req.Amount),
		ToBalance:   lockedTo.Balance.Add(req.Amount),
	}

	if err := s.cacheResponse(ctx, from.UserID, req.IdempotencyKey, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ProcessTopUpWebhook mengubah topup status PENDING -> COMPLETED.
// Idempotent: jika tidak ada baris PENDING yang cocok (misal sudah COMPLETED),
// log warning dan return nil.
func (s *Service) ProcessTopUpWebhook(ctx context.Context, txnID uuid.UUID) error {
	return s.markTopUpCompleted(ctx, s.db, txnID)
}

// GetBalance mengembalikan saldo wallet.
func (s *Service) GetBalance(ctx context.Context, walletID uuid.UUID) (decimal.Decimal, error) {
	return s.repo.GetBalance(ctx, walletID)
}

// ---- helpers internal ----

// markTopUpCompleted memperbarui status topup menjadi COMPLETED dengan guard
// atomik WHERE status = 'PENDING'. q menerima tx atau pool (Querier).
func (s *Service) markTopUpCompleted(ctx context.Context, q interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}, txnID uuid.UUID) error {
	tag, err := q.Exec(ctx, `
		UPDATE topup_transactions
		SET status = 'COMPLETED', completed_at = NOW()
		WHERE id = $1 AND status = 'PENDING'
	`, txnID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		log.Printf("webhook: tidak ada topup PENDING untuk id=%s (idempotent)", txnID)
	}
	return nil
}

// systemWalletID mengambil id wallet sistem (user_id IS NULL) berdasar tipe.
func (s *Service) systemWalletID(ctx context.Context, tx pgx.Tx, walletType string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM wallets WHERE wallet_type = $1 AND user_id IS NULL
	`, walletType).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// lockWalletsAsc mengunci sejumlah wallet dengan urutan id menaik dalam satu
// statement. ORDER BY id ASC membuat PostgreSQL mengambil row lock berurutan,
// sehingga mencegah deadlock antar transaksi yang mengunci wallet sama.
func lockWalletsAsc(ctx context.Context, tx pgx.Tx, walletIDs ...uuid.UUID) error {
	ids := make([]uuid.UUID, len(walletIDs))
	copy(ids, walletIDs)
	sortByUUID(ids)
	_, err := tx.Exec(ctx, `
		SELECT id FROM wallets WHERE id = ANY($1) ORDER BY id ASC FOR UPDATE
	`, ids)
	return err
}

// lockAndReloadWallets mengunci dua wallet (urutan asc) lalu memuat ulang
// balance terkini setelah lock diperoleh.
func lockAndReloadWallets(ctx context.Context, tx pgx.Tx, a, b uuid.UUID) (*Wallet, *Wallet, error) {
	if err := lockWalletsAsc(ctx, tx, a, b); err != nil {
		return nil, nil, err
	}
	from, err := getWalletByIDTx(ctx, tx, a)
	if err != nil {
		return nil, nil, err
	}
	to, err := getWalletByIDTx(ctx, tx, b)
	if err != nil {
		return nil, nil, err
	}
	return from, to, nil
}

// getWalletByIDTx membaca wallet dalam transaksi yang sudah terkunci.
func getWalletByIDTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*Wallet, error) {
	var w Wallet
	err := tx.QueryRow(ctx, `
		SELECT id, user_id, wallet_type, balance, status, created_at, updated_at
		FROM wallets WHERE id = $1
	`, id).Scan(&w.ID, &w.UserID, &w.Type, &w.Balance, &w.Status, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func sortByUUID(ids []uuid.UUID) {
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
}

// overdueDebtFor membaca saldo hutang (overdue_debt) milik user dari users.
// Bug #47: hutang ini dilunasi dari top-up sebelum sisa masuk ke wallet.
func (s *Service) overdueDebtFor(ctx context.Context, userID uuid.UUID) (decimal.Decimal, error) {
	var debt decimal.Decimal
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(overdue_debt, 0) FROM users WHERE id = $1`, userID,
	).Scan(&debt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return decimal.Zero, ErrWalletNotFound
		}
		return decimal.Zero, err
	}
	return debt, nil
}

// settleOverdueDebt menghitung alokasi top-up untuk pelunasan hutang (Bug #47):
//   - Sisa hutang dilunasi lebih dulu (debtSettled).
//   - Sisa amount (netTopUp) yang masuk ke wallet.
//   - Jika amount < hutang, seluruh amount dipakai bayar hutang, wallet tidak
//     bertambah.
func settleOverdueDebt(amount, overdueDebt decimal.Decimal) (debtSettled, netTopUp decimal.Decimal) {
	if !overdueDebt.IsPositive() {
		return decimal.Zero, amount
	}
	if amount.GreaterThanOrEqual(overdueDebt) {
		return overdueDebt, amount.Sub(overdueDebt)
	}
	return amount, decimal.Zero
}

// kycLimitFor menentukan batas saldo berdasarkan kolom users.kyc_status.
// 'VERIFIED' -> 20jt; lainnya (UNVERIFIED/PENDING/unknown) -> 2jt.
func (s *Service) kycLimitFor(ctx context.Context, userID uuid.UUID) (decimal.Decimal, error) {
	var kycStatus string
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(kyc_status, '') FROM users WHERE id = $1`, userID,
	).Scan(&kycStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return decimal.Zero, ErrWalletNotFound
		}
		return decimal.Zero, err
	}
	if kycStatus == kycVerified {
		return kycLimitVerified, nil
	}
	return kycLimitUnverified, nil
}

// ---- idempotency dual-layer ----

// redisKey membangun namespace Redis per user: idempotency:{userID}:{key}.
func redisKey(userID uuid.UUID, key string) string {
	return redisKeyPrefix + userID.String() + ":" + key
}

// redisGetCachedResp mengecek L1 Redis. Mengembalikan raw response JSON jika
// state COMPLETED.
func (s *Service) redisGetCachedResp(ctx context.Context, userID uuid.UUID, key string) (json.RawMessage, bool) {
	if s.redis == nil {
		return nil, false
	}
	val, err := s.redis.Get(ctx, redisKey(userID, key)).Result()
	if err != nil {
		return nil, false
	}
	var cached redisCache
	if err := json.Unmarshal([]byte(val), &cached); err != nil {
		return nil, false
	}
	if cached.State != redisCompleted || len(cached.Response) == 0 {
		return nil, false
	}
	return cached.Response, true
}

// idemAcquire adalah logika L2 idempotency.
//
//   - Tidak ditemukan -> insert PROCESSING (debounce 5 menit) -> proceed.
//   - COMPLETED       -> kembalikan response tersimpan (proceed=false).
//   - PROCESSING stale-> proceed (anggap requester sebelumnya crash).
//   - PROCESSING fresh-> error ErrIdempotencyInProgress.
type idemResult struct {
	cached  json.RawMessage // non-nil hanya saat COMPLETED
	proceed bool            // true jika operasi boleh dilanjutkan
}

func (s *Service) idemAcquire(ctx context.Context, owner uuid.UUID, key string) (idemResult, error) {
	var st string
	var body []byte
	var debounce *time.Time

	qErr := s.db.QueryRow(ctx, `
		SELECT state, response_body, debounce_at FROM idempotency_cache
		WHERE key = $1 AND user_id = $2
	`, key, owner).Scan(&st, &body, &debounce)

	if errors.Is(qErr, pgx.ErrNoRows) {
		if err := s.pgInsertProcessing(ctx, owner, key); err != nil {
			return idemResult{}, err
		}
		return idemResult{proceed: true}, nil
	}
	if qErr != nil {
		return idemResult{}, qErr
	}

	switch st {
	case redisCompleted:
		return idemResult{cached: json.RawMessage(body)}, nil
	case pgProcessing:
		if debounce != nil && debounce.Before(time.Now()) {
			// stale -> lanjutkan
			return idemResult{proceed: true}, nil
		}
		return idemResult{}, ErrIdempotencyInProgress
	default:
		return idemResult{proceed: true}, nil
	}
}

// pgInsertProcessing menulis baris PROCESSING (L2). Jika key sudah ada
// (kompetisi user lain), return ErrIdempotencyInProgress.
func (s *Service) pgInsertProcessing(ctx context.Context, owner uuid.UUID, key string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO idempotency_cache
			(key, user_id, response_body, status_code, state, debounce_at, expires_at)
		VALUES ($1, $2, '{}'::jsonb, 0, 'PROCESSING', NOW() + INTERVAL '5 minutes', NOW() + INTERVAL '24 hours')
	`, key, owner)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrIdempotencyInProgress
		}
		return err
	}
	return nil
}

// cacheResponse menulis hasil sukses ke Redis L1 dan update L2 COMPLETED.
func (s *Service) cacheResponse(ctx context.Context, owner uuid.UUID, key string, resp interface{}) error {
	raw, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	s.redisSet(ctx, owner, key, redisCompleted, raw)
	if _, err := s.db.Exec(ctx, `
		UPDATE idempotency_cache
		SET state = 'COMPLETED', debounce_at = NULL, response_body = $3, status_code = 200
		WHERE key = $1 AND user_id = $2
	`, key, owner, raw); err != nil {
		return err
	}
	return nil
}

// redisSet menulis nilai ke Redis (state COMPLETED) dengan TTL 24 jam.
func (s *Service) redisSet(ctx context.Context, userID uuid.UUID, key string, state string, raw json.RawMessage) {
	if s.redis == nil {
		return
	}
	cached := redisCache{State: state, Response: raw}
	if b, err := json.Marshal(cached); err == nil {
		s.redis.Set(ctx, redisKey(userID, key), b, redisTTL)
	}
}

// redisCache adalah bentuk yang disimpan di Redis L1.
type redisCache struct {
	State    string          `json:"state"`
	Response json.RawMessage `json:"response"`
}
