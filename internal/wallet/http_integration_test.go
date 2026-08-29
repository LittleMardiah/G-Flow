//go:build integration

// Package wallet_test — integration test berbasis HTTP (httptest + Gin)
// untuk endpoint wallet terhadap database nyata PostgreSQL.
//
// Menyalin setup router dari cmd/api/main.go. Jalankan dengan:
//
//	go test -tags integration ./internal/wallet/ -run Integration -v
//
// Prasyarat: docker-compose (PostgreSQL:15432, Redis:6380) up + migrations.
package wallet_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/g-flow/g-flow/internal/auth"
	"github.com/g-flow/g-flow/internal/middleware"
	"github.com/g-flow/g-flow/internal/wallet"
)

// tg adalah dependensi router untuk test HTTP wallet.
type tg struct {
	pool *pgxpool.Pool
	jwt  *auth.JWTService
}

func setupHTTPRouter(t *testing.T) (*gin.Engine, *tg) {
	t.Helper()
	pool := setupPool(t)

	rdb, err := redis.ParseURL("redis://localhost:6380/0")
	require.NoError(t, err)
	rd := redis.NewClient(rdb)
	t.Cleanup(func() { _ = rd.Close() })

	jwt := auth.NewJWTService("integration-test-secret")
	blacklist := auth.NewBlacklistService(rd)

	authHandler := auth.NewHandler(jwt, blacklist, pool)

	repo := wallet.NewRepository(pool)
	ledger := wallet.NewLedgerService(pool)
	// Redis L1 aktif: L1 idempotency (Redis) + L2 (PostgreSQL) keduanya bekerja.
	svc := wallet.NewService(repo, ledger, rd, pool)
	walletHandler := wallet.NewHandler(svc)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/webhooks/topup", walletHandler.ProcessTopUpWebhook)

	r.POST("/api/v1/auth/register", authHandler.Register)
	r.POST("/api/v1/auth/login", authHandler.Login)

	api := r.Group("/api/v1", middleware.AuthMiddleware(jwt, blacklist))
	{
		api.POST("/wallets/:wallet_id/topup", walletHandler.TopUp)
		api.POST("/wallets/:wallet_id/transfer", walletHandler.Transfer)
		api.GET("/wallets/:wallet_id/balance", walletHandler.GetBalance)
	}

	return r, &tg{pool: pool, jwt: jwt}
}

type walletDataResp struct {
	Success bool `json:"success"`
	Data    struct {
		WalletID uuid.UUID `json:"wallet_id"`
		Balance  string    `json:"balance"`
	} `json:"data"`
}

type authResp struct {
	Success bool `json:"success"`
	Data    struct {
		AccessToken string    `json:"access_token"`
		UserID      uuid.UUID `json:"user_id"`
	} `json:"data"`
}

func wDoJSON(r *gin.Engine, method, path string, body any, token string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// registerAndLogin mendaftar user lalu login lewat HTTP; mengembalikan token
// dan user id.
func registerAndLogin(t *testing.T, r *gin.Engine) (token string, userID uuid.UUID) {
	t.Helper()
	email := "it" + strings.ReplaceAll(uuid.New().String(), "-", "") + "@test.com"

	w := wDoJSON(r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":     email,
		"password":  "password123",
		"name":      "HTTP User",
		"user_type": "customer",
	}, "")
	require.Equal(t, http.StatusCreated, w.Code, "register: %s", w.Body.String())

	w = wDoJSON(r, http.MethodPost, "/api/v1/auth/login", gin.H{
		"email":    email,
		"password": "password123",
	}, "")
	require.Equal(t, http.StatusOK, w.Code, "login: %s", w.Body.String())

	var ar authResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ar))
	require.NotEmpty(t, ar.Data.AccessToken)
	return ar.Data.AccessToken, ar.Data.UserID
}

// customerWalletID mengambil wallet CUSTOMER milik user dari DB.
func (g *tg) customerWalletID(t *testing.T, ctx context.Context, userID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := g.pool.QueryRow(ctx,
		`SELECT id FROM wallets WHERE user_id = $1 AND wallet_type = 'CUSTOMER'`, userID,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestWalletTopUp_Success(t *testing.T) {
	ctx := context.Background()
	r, g := setupHTTPRouter(t)

	token, userID := registerAndLogin(t, r)
	walletID := g.customerWalletID(t, ctx, userID)

	// Balance awal harus 0.
	bal0 := g.getBalance(t, ctx, walletID)
	require.True(t, bal0.IsZero(), "balance awal = %v", bal0)

	resp := wDoJSON(r, http.MethodPost, "/api/v1/wallets/"+walletID.String()+"/topup", gin.H{
		"amount":         "100000",
		"idempotency_key": uuid.New().String(),
	}, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("topup gagal: %d %s", resp.Code, resp.Body.String())
	}

	bal := g.getBalance(t, ctx, walletID)
	require.True(t, bal.Equal(decimal.NewFromInt(100000)), "balance = %v, want 100000", bal)
}

func TestWalletTransfer_Success(t *testing.T) {
	ctx := context.Background()
	r, g := setupHTTPRouter(t)

	tokenFrom, fromUserID := registerAndLogin(t, r)
	_, toUserID := registerAndLogin(t, r)
	fromWalletID := g.customerWalletID(t, ctx, fromUserID)
	toWalletID := g.customerWalletID(t, ctx, toUserID)

	// Topup dulu wallet sumber.
	resp := wDoJSON(r, http.MethodPost, "/api/v1/wallets/"+fromWalletID.String()+"/topup", gin.H{
		"amount":         "200000",
		"idempotency_key": uuid.New().String(),
	}, tokenFrom)
	if resp.Code != http.StatusOK {
		t.Fatalf("topup gagal: %d %s", resp.Code, resp.Body.String())
	}

	// Transfer 50000 ke wallet tujuan.
	resp = wDoJSON(r, http.MethodPost, "/api/v1/wallets/"+fromWalletID.String()+"/transfer", gin.H{
		"to_wallet_id":   toWalletID.String(),
		"amount":         "50000",
		"idempotency_key": uuid.New().String(),
		"description":     "integration-http-transfer",
	}, tokenFrom)
	if resp.Code != http.StatusOK {
		t.Fatalf("transfer gagal: %d %s", resp.Code, resp.Body.String())
	}

	fromBal := g.getBalance(t, ctx, fromWalletID)
	toBal := g.getBalance(t, ctx, toWalletID)
	require.True(t, fromBal.Equal(decimal.NewFromInt(150000)), "from = %v", fromBal)
	require.True(t, toBal.Equal(decimal.NewFromInt(50000)), "to = %v", toBal)
}

func TestWalletTopUp_Unauthorized(t *testing.T) {
	ctx := context.Background()
	r, g := setupHTTPRouter(t)

	token, userID := registerAndLogin(t, r)
	walletID := g.customerWalletID(t, ctx, userID)
	_ = token

	// Tanpa token -> middleware auth menolak sebelum handler.
	resp := wDoJSON(r, http.MethodPost, "/api/v1/wallets/"+walletID.String()+"/topup", gin.H{
		"amount":         "100000",
		"idempotency_key": uuid.New().String(),
	}, "")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (%s)", resp.Code, resp.Body.String())
	}
}

// getBalance membaca saldo wallet langsung dari DB.
func (g *tg) getBalance(t *testing.T, ctx context.Context, walletID uuid.UUID) decimal.Decimal {
	t.Helper()
	var bal decimal.Decimal
	err := g.pool.QueryRow(ctx, `SELECT balance FROM wallets WHERE id = $1`, walletID).Scan(&bal)
	require.NoError(t, err)
	return bal
}

// TestWalletTopUp_IdempotentSameKey: topup dua kali dengan idempotency key
// yang sama -> diproses sekali (menutup jalur cache L1 Redis).
func TestWalletTopUp_IdempotentSameKey(t *testing.T) {
	ctx := context.Background()
	r, g := setupHTTPRouter(t)

	token, userID := registerAndLogin(t, r)
	walletID := g.customerWalletID(t, ctx, userID)
	key := uuid.New().String()

	body := gin.H{"amount": "50000", "idempotency_key": key}

	w1 := wDoJSON(r, http.MethodPost, "/api/v1/wallets/"+walletID.String()+"/topup", body, token)
	require.Equal(t, http.StatusOK, w1.Code, "topup pertama: %s", w1.Body.String())

	// Kedua kalinya harus memakai hasil tersimpan (tidak menambah saldo lagi).
	w2 := wDoJSON(r, http.MethodPost, "/api/v1/wallets/"+walletID.String()+"/topup", body, token)
	require.Equal(t, http.StatusOK, w2.Code, "topup kedua: %s", w2.Body.String())

	bal := g.getBalance(t, ctx, walletID)
	require.True(t, bal.Equal(decimal.NewFromInt(50000)), "balance = %v, want 50000", bal)
}

// TestWalletTransfer_NotOwned: token user A tidak boleh transfer dari wallet user B.
func TestWalletTransfer_NotOwned(t *testing.T) {
	ctx := context.Background()
	r, g := setupHTTPRouter(t)

	tokenA, userA := registerAndLogin(t, r)
	_, userB := registerAndLogin(t, r)
	walletA := g.customerWalletID(t, ctx, userA)
	walletB := g.customerWalletID(t, ctx, userB)

	// A mencoba transfer DARI wallet milik B -> harus ditolak.
	resp := wDoJSON(r, http.MethodPost, "/api/v1/wallets/"+walletB.String()+"/transfer", gin.H{
		"to_wallet_id":    walletA.String(),
		"amount":          "10000",
		"idempotency_key": uuid.New().String(),
	}, tokenA)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (%s)", resp.Code, resp.Body.String())
	}
}

// TestWalletTransfer_SameWallet: transfer ke wallet sendiri -> 422.
func TestWalletTransfer_SameWallet(t *testing.T) {
	ctx := context.Background()
	r, g := setupHTTPRouter(t)

	token, userID := registerAndLogin(t, r)
	walletID := g.customerWalletID(t, ctx, userID)

	resp := wDoJSON(r, http.MethodPost, "/api/v1/wallets/"+walletID.String()+"/transfer", gin.H{
		"to_wallet_id":    walletID.String(),
		"amount":          "10000",
		"idempotency_key": uuid.New().String(),
	}, token)
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", resp.Code, resp.Body.String())
	}
}

// TestWalletTransfer_InsufficientBalance: saldo kurang -> 422.
func TestWalletTransfer_InsufficientBalance(t *testing.T) {
	ctx := context.Background()
	r, g := setupHTTPRouter(t)

	tokenFrom, fromUserID := registerAndLogin(t, r)
	_, toUserID := registerAndLogin(t, r)
	fromWalletID := g.customerWalletID(t, ctx, fromUserID)
	toWalletID := g.customerWalletID(t, ctx, toUserID)

	// Balance 0 (tanpa topup), transfer 5000 -> tidak cukup.
	resp := wDoJSON(r, http.MethodPost, "/api/v1/wallets/"+fromWalletID.String()+"/transfer", gin.H{
		"to_wallet_id":    toWalletID.String(),
		"amount":          "5000",
		"idempotency_key": uuid.New().String(),
	}, tokenFrom)
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", resp.Code, resp.Body.String())
	}
}

// TestWalletWebhook_Topup: webhook tanpa row PENDING -> 200 (idempotent).
func TestWalletWebhook_Topup(t *testing.T) {
	r, _ := setupHTTPRouter(t)

	w := wDoJSON(r, http.MethodPost, "/webhooks/topup", gin.H{
		"transaction_id": uuid.New().String(),
		"status":         "COMPLETED",
	}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestWalletGetBalance_Success: cek saldo lewat endpoint GET.
func TestWalletGetBalance_Success(t *testing.T) {
	ctx := context.Background()
	r, g := setupHTTPRouter(t)

	token, userID := registerAndLogin(t, r)
	walletID := g.customerWalletID(t, ctx, userID)

	w := wDoJSON(r, http.MethodGet, "/api/v1/wallets/"+walletID.String()+"/balance", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body walletDataResp
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Data.WalletID != walletID {
		t.Fatalf("wallet_id mismatch")
	}
}

// TestWalletGetBalance_NotFound: wallet tidak ada -> 404 WALLET_NOT_FOUND.
func TestWalletGetBalance_NotFound(t *testing.T) {
	r, _ := setupHTTPRouter(t)

	token, _ := registerAndLogin(t, r)
	w := wDoJSON(r, http.MethodGet, "/api/v1/wallets/"+uuid.New().String()+"/balance", nil, token)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestWalletTopUp_InvalidAmount: amount tidak positif -> 422.
func TestWalletTopUp_InvalidAmount(t *testing.T) {
	ctx := context.Background()
	r, g := setupHTTPRouter(t)

	token, userID := registerAndLogin(t, r)
	walletID := g.customerWalletID(t, ctx, userID)

	w := wDoJSON(r, http.MethodPost, "/api/v1/wallets/"+walletID.String()+"/topup", gin.H{
		"amount":          "0",
		"idempotency_key": uuid.New().String(),
	}, token)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestWalletTopUp_InvalidWalletID: wallet_id bukan UUID -> 400.
func TestWalletTopUp_InvalidWalletID(t *testing.T) {
	r, _ := setupHTTPRouter(t)

	token, _ := registerAndLogin(t, r)
	w := wDoJSON(r, http.MethodPost, "/api/v1/wallets/not-a-uuid/topup", gin.H{
		"amount":          "10000",
		"idempotency_key": uuid.New().String(),
	}, token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestWalletTopUp_MissingIdempotency: idempotency_key wajib -> 422.
func TestWalletTopUp_MissingIdempotency(t *testing.T) {
	ctx := context.Background()
	r, g := setupHTTPRouter(t)

	token, userID := registerAndLogin(t, r)
	walletID := g.customerWalletID(t, ctx, userID)

	w := wDoJSON(r, http.MethodPost, "/api/v1/wallets/"+walletID.String()+"/topup", gin.H{
		"amount": "10000",
	}, token)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", w.Code, w.Body.String())
	}
}
