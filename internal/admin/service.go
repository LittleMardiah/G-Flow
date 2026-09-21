// Package admin — Transaction Reversal Service (Task 4.2.4).
//
// ReverseTransaction membalikkan transaksi yang sudah settled secara
// proporsional:
//
//	1. Refund customer penuh (CREDIT kembali ke escrow/wallet asal DEBIT).
//	2. Clawback proporsional dari Merchant, Driver, Platform (DEBIT).
//	3. Jika saldo partai < bagiannya → shortfall dicatat sebagai
//	   SYSTEM_RECEIVABLE_OVERDRAFT (piutang platform atas partai tersebut).
//	4. Auto-sweep journal memindahkan dana yang tersedia ke receivable.
//
// Algoritma menjaga invariant double-entry: SUM(DEBIT) == SUM(CREDIT) per
// reference_id (reference_type = "REVERSAL").
//
// Catatan akuntansi shortfall: untuk satu reference settlement WALLET dengan
// escrow DEBIT T dan partai CREDIT m+d+p (=T), saat saldo merchant mb < m:
//
//	CREDIT escrow T          (refund penuh customer / rebuild escrow)
//	DEBIT merchant mb        (clawback yang tersedia)
//	DEBIT driver  d
//	DEBIT platform p
//	DEBIT SYSTEM_RECEIVABLE_OVERDRAFT (m - mb)
//
// Jumlah DEBIT = mb + d + p + (m - mb) = m + d + p = T = jumlah CREDIT. ✓
package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
)

// Reference type / description constants untuk journal reversal.
const (
	ReferenceTypeReverse = "REVERSAL"
	referenceTypeSweep   = "OVERDRAFT_SWEEP"

	walletTypeMerchant     = "MERCHANT"
	walletTypeDriver       = "DRIVER"
	walletTypeSystemEscrow = "SYSTEM_ESCROW"
	walletTypeSysPlatform  = "SYSTEM_PLATFORM"
	walletTypeSDROverdraft = "SYSTEM_RECEIVABLE_OVERDRAFT"

	descCustomerRefund   = "REVERSAL - full customer refund"
	descMerchantClawback = "REVERSAL - proportional merchant clawback"
	descDriverClawback   = "REVERSAL - proportional driver clawback"
	descPlatformClawback = "REVERSAL - proportional platform clawback"
	descShortfallSweep   = "OVERDRAFT_SWEEP - shortfall to SYSTEM_RECEIVABLE_OVERDRAFT"
)

// Sentinel errors layanan reversal.
var (
	ErrTransactionNotFound = errors.New("admin: transaksi tidak ditemukan (tanpa ledger entry aktif)")
	ErrTransactionReversed = errors.New("admin: transaksi sudah pernah di-reverse")
	ErrInvalidReason       = errors.New("admin: reason wajib diisi")
	ErrAdminForbidden      = errors.New("admin: akses ditolak, role bukan admin")
	ErrNotBalanced         = errors.New("admin: ledger tidak seimbang untuk reversal")
	ErrWalletNotFound      = errors.New("admin: wallet tidak ditemukan")
	ErrReversalFailed      = errors.New("admin: reversal gagal")
	Err2FAInvalid          = errors.New("admin: 2FA token tidak valid")
	ErrLockoutActive       = errors.New("admin: akun terkunci karena terlalu banyak percobaan 2FA")

	ErrInvalidCredentials = errors.New("admin: email atau password salah")
	ErrNotAdmin           = errors.New("admin: akun bukan admin")
	ErrAccountInactive    = errors.New("admin: akun tidak aktif")
)

// Share adalah bagian partai (merchant/driver/platform) yang harus di-clawback.
type Share struct {
	WalletID   uuid.UUID
	WalletType string
	Amount     decimal.Decimal
}

// ReversalRequest adalah input operasi reversal.
type ReversalRequest struct {
	TransactionID uuid.UUID
	AdminID       uuid.UUID
	Reason        string
	Notes         string
}

// ReversalResult adalah output operasi reversal.
type ReversalResult struct {
	ReversalID uuid.UUID
	Refunded   decimal.Decimal
	Shortfall  decimal.Decimal
	HasSweep   bool
}

// LoginResult adalah output operasi login admin.
type LoginResult struct {
	UserID   uuid.UUID
	Email    string
	UserType string
}

// Service adalah business logic reversal transaksi.
type Service struct {
	repo   *Repository
	db     DB
	logger *slog.Logger
}

// NewService membuat Service reversal baru.
func NewService(repo *Repository, db DB, logger *slog.Logger) *Service {
	return &Service{repo: repo, db: db, logger: logger}
}

// bcryptCost adalah cost hash password admin (12), sama dengan seed admin di
// migration 014. Digunakan juga untuk dummy hash agar durasi bcrypt compare
// sebanding (mitigasi timing attack / user enumeration).
const bcryptCost = 12

// dummyPasswordHash adalah bcrypt hash (cost 12) dari string acak, dihitung
// sekali saat init. Saat email tidak terdaftar, Login tetap menjalankan
// bcrypt.CompareHashAndPassword memakai hash ini sehingga durasi respon hampir
// identik dengan kasus "password salah" — attacker tidak bisa membedakan email
// yang terdaftar vs tidak.
var dummyPasswordHash = func() string {
	h, err := bcrypt.GenerateFromPassword(
		[]byte("g-flow-dummy-hash-"+uuid.NewString()),
		bcryptCost,
	)
	if err != nil {
		panic(fmt.Sprintf("admin: gagal generate dummy bcrypt hash: %v", err))
	}
	return string(h)
}()

// Login memvalidasi kredensial admin (email + password) dan mengembalikan
// identitas user bila valid. Urutan pemeriksaan (bcrypt DULU — mencegah
// account-type oracle / enumerasi email):
//
//	1. Format email (minimal "@" dan ".").
//	2. bcrypt match password (ErrInvalidCredentials). Email yang tidak
//	   terdaftar tetap melewati bcrypt compare dengan dummy hash supaya durasi
//	   respon seragam.
//	3. user_type == "admin" (akun lain -> ErrNotAdmin).
//	4. status == "ACTIVE" (ErrAccountInactive).
func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) {
		return nil, ErrInvalidCredentials
	}

	user, err := s.repo.GetLoginUser(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		// Email tidak terdaftar: tetap jalankan bcrypt compare dengan dummy
		// hash supaya durasi respon tidak bocor (timing attack).
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Hash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	if user.UserType != "admin" {
		return nil, ErrNotAdmin
	}
	if user.Status != "ACTIVE" {
		return nil, ErrAccountInactive
	}

	return &LoginResult{UserID: user.UserID, Email: user.Email, UserType: user.UserType}, nil
}

// ReverseTransaction membalikkan transaksi dengan clawback proporsional.
func (s *Service) ReverseTransaction(ctx context.Context, req ReversalRequest) (*ReversalResult, error) {
	if req.Reason == "" {
		return nil, ErrInvalidReason
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReversalFailed, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return nil, err
	}

	// 1) Ambil entry asli (non-reversed) milik reference_id.
	entries, err := s.repo.GetLedgerEntries(ctx, tx, req.TransactionID)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, ErrTransactionNotFound
	}

	// Idempotency: jika semua entry sudah reversed, tolak.
	if allReversed(entries) {
		return nil, ErrTransactionReversed
	}

	// 2) Kumpulkan seluruh wallet yang terlibat + jenisnya.
	walletIDList := uniqueWalletIDs(entries)
	walletTypes, err := s.repo.GetWalletTypes(ctx, tx, walletIDList)
	if err != nil {
		return nil, err
	}

	// 3) Map: refund wallet (asal DEBIT = escrow) dan partai clawback
	//    (wallet yang di-CREDIT, merchant/driver/platform).
	refundWallet := uuid.Nil
	var shares []Share
	var totalRefund decimal.Decimal

	for _, e := range entries {
		wt, _ := walletTypes[e.WalletID]
		if e.EntryType == "DEBIT" {
			if refundWallet == uuid.Nil {
				refundWallet = e.WalletID
			}
			continue
		}
		if wt == walletTypeMerchant || wt == walletTypeDriver || wt == walletTypeSysPlatform {
			shares = append(shares, Share{WalletID: e.WalletID, WalletType: wt, Amount: e.Amount})
			totalRefund = totalRefund.Add(e.Amount)
		}
	}

	if refundWallet == uuid.Nil {
		return nil, ErrNotBalanced
	}

	// 4) Kunci saldo wallet terkait (deadlock-free) + receivable.
	receivableID, err := s.repo.SystemWalletID(ctx, tx, walletTypeSDROverdraft)
	if err != nil {
		return nil, err
	}
	lockIDs := append(unique(append([]uuid.UUID{}, walletIDList...)), receivableID)

	balances, err := s.repo.GetWalletBalances(ctx, tx, lockIDs)
	if err != nil {
		return nil, err
	}
	balanceByWallet := make(map[uuid.UUID]decimal.Decimal, len(balances))
	for _, b := range balances {
		balanceByWallet[b.WalletID] = b.Balance
	}

	// 5) Bangun compensating entries + shortfall.
	createdBy := req.AdminID
	entriesToInsert := make([]LedgerInsert, 0)
	var shortfall decimal.Decimal

	if totalRefund.IsPositive() {
		entriesToInsert = append(entriesToInsert, LedgerInsert{
			WalletID:      refundWallet,
			EntryType:     "CREDIT",
			Amount:        totalRefund,
			ReferenceType: ReferenceTypeReverse,
			ReferenceID:   req.TransactionID,
			Description:   descCustomerRefund,
			CreatedBy:     &createdBy,
		})
	}

	for _, sh := range shares {
		avail := balanceByWallet[sh.WalletID]
		actual := sh.Amount
		if avail.LessThan(sh.Amount) {
			actual = decimal.Zero
			if avail.IsPositive() {
				actual = avail
			}
			shortfall = shortfall.Add(sh.Amount.Sub(actual))
		}
		if actual.IsPositive() {
			entriesToInsert = append(entriesToInsert, LedgerInsert{
				WalletID:      sh.WalletID,
				EntryType:     "DEBIT",
				Amount:        actual,
				ReferenceType: ReferenceTypeReverse,
				ReferenceID:   req.TransactionID,
				Description:   clawbackDescription(sh.WalletType),
				CreatedBy:     &createdBy,
			})
		}
	}

	if shortfall.IsPositive() {
		entriesToInsert = append(entriesToInsert, LedgerInsert{
			WalletID:      receivableID,
			EntryType:     "DEBIT",
			Amount:        shortfall,
			ReferenceType: referenceTypeSweep,
			ReferenceID:   req.TransactionID,
			Description:   descShortfallSweep,
			CreatedBy:     &createdBy,
		})
	}

	// 6) Invariant double-entry: DEBIT == CREDIT.
	if !isBalanced(entriesToInsert) {
		return nil, ErrNotBalanced
	}

	// 7) Masukkan compensating entries.
	if err := s.repo.InsertLedgerEntries(ctx, tx, entriesToInsert); err != nil {
		return nil, err
	}

	// 8) Tandai entry asli is_reversed = TRUE (trigger mengembalikan saldo).
	entryIDs := make([]uuid.UUID, 0, len(entries))
	for _, e := range entries {
		entryIDs = append(entryIDs, e.ID)
	}
	if err := s.repo.UpdateLedgerReversal(ctx, tx, req.TransactionID, entryIDs); err != nil {
		return nil, err
	}

	// 9) Log aksi admin (audit trail).
	details := map[string]any{
		"reason":    req.Reason,
		"notes":     req.Notes,
		"refunded":  totalRefund.String(),
		"shortfall": shortfall.String(),
	}
	if err := s.repo.CreateAdminActionLog(ctx, tx, req.AdminID, "transaction_reverse", "transaction", &req.TransactionID, &req.TransactionID, details); err != nil {
		s.logger.Warn("failed to write admin action log", "admin_id", req.AdminID, "error", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReversalFailed, err)
	}

	return &ReversalResult{
		ReversalID: uuid.New(),
		Refunded:   totalRefund,
		Shortfall:  shortfall,
		HasSweep:   shortfall.IsPositive(),
	}, nil
}

// LedgerItem adalah item detail transaksi untuk ditampilkan di halaman reversal.
type LedgerItem struct {
	WalletID   uuid.UUID       `json:"wallet_id"`
	WalletType string          `json:"wallet_type"`
	EntryType  string          `json:"entry_type"`
	Amount     decimal.Decimal `json:"amount"`
	Reference  string          `json:"reference"`
	Balance    decimal.Decimal `json:"balance"`
}

// TransactionDetail adalah ringkasan transaksi + breakdown proporsional.
type TransactionDetail struct {
	TransactionID      uuid.UUID       `json:"transaction_id"`
	RefundWallet       uuid.UUID       `json:"refund_wallet"`
	TotalAmount        decimal.Decimal `json:"total_amount"`
	Entries            []LedgerItem    `json:"entries"`
	MerchantShare      decimal.Decimal `json:"merchant_share"`
	DriverShare        decimal.Decimal `json:"driver_share"`
	PlatformShare      decimal.Decimal `json:"platform_share"`
	PotentialShortfall decimal.Decimal `json:"potential_shortfall"`
	Reversed           bool            `json:"reversed"`
}

// GetTransactionDetail membaca detail transaksi + breakdown proporsional tanpa
// melakukan reversal. Membaca hanya (transaksi ringan, tidak mengubah data).
func (s *Service) GetTransactionDetail(ctx context.Context, transactionID uuid.UUID) (*TransactionDetail, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, ErrReversalFailed
	}
	defer func() { _ = tx.Rollback(ctx) }()

	entriesAll, err := s.repo.GetAllLedgerEntries(ctx, tx, transactionID)
	if err != nil {
		return nil, err
	}
	if len(entriesAll) == 0 {
		return nil, ErrTransactionNotFound
	}

	walletIDList := uniqueWalletIDs(entriesAll)
	walletTypes, err := s.repo.GetWalletTypes(ctx, tx, walletIDList)
	if err != nil {
		return nil, err
	}
	balances, err := s.repo.GetWalletBalances(ctx, tx, walletIDList)
	if err != nil {
		return nil, err
	}
	balanceByWallet := make(map[uuid.UUID]decimal.Decimal, len(balances))
	for _, b := range balances {
		balanceByWallet[b.WalletID] = b.Balance
	}

	detail := &TransactionDetail{TransactionID: transactionID}
	var total decimal.Decimal
	var shortfall decimal.Decimal
	reversed := true
	for _, e := range entriesAll {
		if !e.IsReversed {
			reversed = false
		}
		wt, _ := walletTypes[e.WalletID]
		detail.Entries = append(detail.Entries, LedgerItem{
			WalletID:   e.WalletID,
			WalletType: wt,
			EntryType:  e.EntryType,
			Amount:     e.Amount,
			Reference:  e.ReferenceType + "/" + e.ReferenceID.String(),
			Balance:    balanceByWallet[e.WalletID],
		})
		if e.EntryType == "CREDIT" {
			switch wt {
			case walletTypeMerchant:
				detail.MerchantShare = detail.MerchantShare.Add(e.Amount)
			case walletTypeDriver:
				detail.DriverShare = detail.DriverShare.Add(e.Amount)
			case walletTypeSysPlatform:
				detail.PlatformShare = detail.PlatformShare.Add(e.Amount)
			}
			if wt == walletTypeMerchant || wt == walletTypeDriver || wt == walletTypeSysPlatform {
				if balanceByWallet[e.WalletID].LessThan(e.Amount) {
					shortfall = shortfall.Add(e.Amount.Sub(balanceByWallet[e.WalletID]))
				}
			}
		}
		if e.EntryType == "DEBIT" {
			total = total.Add(e.Amount)
		}
	}
	if shortfall.IsNegative() {
		shortfall = decimal.Zero
	}
	detail.PotentialShortfall = shortfall
	detail.RefundWallet = refundWalletOf(entriesAll)
	detail.TotalAmount = total
	detail.Reversed = reversed
	return detail, nil
}

// jsonDecimal membungkus decimal.Decimal agar di-marshal sebagai JSON number
// (bukan string). shopspring v1.4.0 default MarshalJSON menghasilkan string;
// frontend types.ts mengetikkan field moneter sebagai number.
type jsonDecimal decimal.Decimal

// MarshalJSON menulis nilai desimal tanpa tanda kutip (JSON number asli).
func (d jsonDecimal) MarshalJSON() ([]byte, error) {
	return []byte(decimal.Decimal(d).String()), nil
}

// DashboardKPIs adalah payload GET /admin/dashboard/kpis (snake_case, mengikuti
// apps/admin_web/src/lib/types.ts -> DashboardKPIs).
type DashboardKPIs struct {
	ActiveOrders           int64              `json:"active_orders"`
	TotalTransactionVolume jsonDecimal        `json:"total_transaction_volume"`
	AvgFare                jsonDecimal        `json:"avg_fare"`
	RevenueToday           jsonDecimal        `json:"revenue_today"`
	ErrorRate              float64            `json:"error_rate"`
	OrderStatusBreakdown   []OrderStatusCount `json:"order_status_breakdown"`
}

// TransactionItem adalah satu baris transaksi untuk dashboard admin
// (apps/admin_web/src/lib/types.ts -> Transaction).
type TransactionItem struct {
	ID        uuid.UUID   `json:"id"`
	WalletID  uuid.UUID   `json:"wallet_id"`
	EntryType string      `json:"entry_type"`
	Amount    jsonDecimal `json:"amount"`
	Reference string      `json:"reference"`
	CreatedAt time.Time   `json:"created_at"`
	Status    string      `json:"status"`
	Note      string      `json:"note"`
}

// TransactionList adalah payload GET /admin/dashboard/transactions
// (apps/admin_web/src/lib/types.ts -> TransactionListResponse).
type TransactionList struct {
	TotalCount   int64             `json:"total_count"`
	Transactions []TransactionItem `json:"transactions"`
}

// GetDashboardKPIs merangkum KPI halaman dashboard admin. Error rate masih
// stub 0.0 karena tracking error belum ada (TD-046).
func (s *Service) GetDashboardKPIs(ctx context.Context) (*DashboardKPIs, error) {
	activeOrders, err := s.repo.CountActiveOrders(ctx)
	if err != nil {
		return nil, err
	}
	volume, err := s.repo.SumTransactionVolume24h(ctx)
	if err != nil {
		return nil, err
	}
	avgFare, err := s.repo.AvgFare24h(ctx)
	if err != nil {
		return nil, err
	}
	revenue, err := s.repo.RevenueToday(ctx)
	if err != nil {
		return nil, err
	}
	breakdown, err := s.repo.CountOrdersByStatus(ctx)
	if err != nil {
		return nil, err
	}

	return &DashboardKPIs{
		ActiveOrders:           activeOrders,
		TotalTransactionVolume: jsonDecimal(volume),
		AvgFare:                jsonDecimal(avgFare),
		RevenueToday:           jsonDecimal(revenue),
		ErrorRate:              0.0,
		OrderStatusBreakdown:   breakdown,
	}, nil
}

// GetDashboardTransactions mengambil daftar transaksi ledger terbaru dengan
// pagination (limit/offset) plus total seluruh baris.
func (s *Service) GetDashboardTransactions(ctx context.Context, limit, offset int) (*TransactionList, error) {
	rows, total, err := s.repo.ListRecentTransactions(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	items := make([]TransactionItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, TransactionItem{
			ID:        r.ID,
			WalletID:  r.WalletID,
			EntryType: r.EntryType,
			Amount:    jsonDecimal(r.Amount),
			Reference: r.Reference,
			CreatedAt: r.CreatedAt,
			Status:    r.Status,
			Note:      r.Note,
		})
	}
	return &TransactionList{TotalCount: total, Transactions: items}, nil
}

func refundWalletOf(entries []LedgerEntry) uuid.UUID {
	for _, e := range entries {
		if e.EntryType == "DEBIT" {
			return e.WalletID
		}
	}
	return uuid.Nil
}

func allReversed(entries []LedgerEntry) bool {
	for _, e := range entries {
		if !e.IsReversed {
			return false
		}
	}
	return true
}

func uniqueWalletIDs(entries []LedgerEntry) []uuid.UUID {
	var out []uuid.UUID
	seen := make(map[uuid.UUID]struct{})
	for _, e := range entries {
		if _, ok := seen[e.WalletID]; !ok {
			seen[e.WalletID] = struct{}{}
			out = append(out, e.WalletID)
		}
	}
	return out
}

func unique(ids []uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	seen := make(map[uuid.UUID]struct{})
	for _, id := range ids {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

func isBalanced(entries []LedgerInsert) bool {
	totalDebit, totalCredit := decimal.Zero, decimal.Zero
	for _, e := range entries {
		if e.EntryType == "DEBIT" {
			totalDebit = totalDebit.Add(e.Amount)
		} else {
			totalCredit = totalCredit.Add(e.Amount)
		}
	}
	return totalDebit.Equal(totalCredit)
}

func clawbackDescription(walletType string) string {
	switch walletType {
	case walletTypeMerchant:
		return descMerchantClawback
	case walletTypeDriver:
		return descDriverClawback
	default:
		return descPlatformClawback
	}
}

// validEmail melakukan validasi ringan format email (regex ketat di DB CHECK).
func validEmail(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}
