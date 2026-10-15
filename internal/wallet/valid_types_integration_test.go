//go:build integration

// Package wallet — integration test drift check validWalletTypes <-> pg_enum (TD-172).
//
// validWalletTypes (internal/wallet/handler.go:57) adalah whitelist Go yang
// menduplikasi enum wallet_type_enum PostgreSQL. Setiap migration yang
// menambah enum WAJIB update whitelist manual (sudah 2x: 010:50, 019:68),
// sehingga ada risiko drift. Test ini membandingkan kedua sumber secara
// langsung terhadap DB nyata dan gagal kalau ada selisih (arah pun).
//
// Prasyarat: PostgreSQL ter-migrate (docker-compose) + DATABASE_URL diset.
// Jalankan dengan:
//
//	DATABASE_URL=... go test -tags integration ./internal/wallet/ -run TestIntegrationWallet_ValidWalletTypes -v -count=1
package wallet

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/g-flow/g-flow/internal/db"
)

// setupWalletTypePool baca DATABASE_URL, skip kalau kosong (mirror
// setupAdminPool TD-181 di internal/admin/repository_integration_test.go:26).
// Funksi ini privat per paket: test ini pakai package wallet (bukan
// wallet_test) supaya bisa mengakses validWalletTypes yang unexported, jadi
// tidak bisa memakai setupPool milik paket wallet_test.
func setupWalletTypePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL tidak diset, melewati integration test")
	}
	pool, err := db.NewDB(db.Config{DatabaseURL: url})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close(pool) })
	return pool
}

// dbWalletTypeEnums mengembalikan label wallet_type_enum dari pg_enum
// (runtime DB, bukan baca migration file) — exact set, tanpa urutan.
func dbWalletTypeEnums(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `
		SELECT enumlabel
		FROM pg_enum
		WHERE enumtypid = 'wallet_type_enum'::regtype
		ORDER BY enumsortorder`)
	require.NoError(t, err)
	defer rows.Close()

	labels := make([]string, 0, 8)
	for rows.Next() {
		var label string
		require.NoError(t, rows.Scan(&label))
		labels = append(labels, label)
	}
	require.NoError(t, rows.Err())
	return labels
}

// TestIntegrationWallet_ValidWalletTypesMatchesDB — bandingkan whitelist Go
// validWalletTypes dengan label enum wallet_type_enum di pg_enum (TD-172).
// Kedua sisi di-sort lalu di-equal-kan persis: selisih di arah mana pun
// (enum baru tanpa whitelist update, atau whitelist label yang tidak ada di
// DB) membuat test gagal — bukan vacuous check.
func TestIntegrationWallet_ValidWalletTypesMatchesDB(t *testing.T) {
	pool := setupWalletTypePool(t)

	dbLabels := dbWalletTypeEnums(t, pool)
	require.NotEmpty(t, dbLabels, "wallet_type_enum kosong di DB — migrasi belum jalan?")

	// Salin dulu supaya slice package tidak di-sort in-place oleh test.
	goLabels := append([]string(nil), validWalletTypes...)
	sort.Strings(goLabels)
	sortedDB := append([]string(nil), dbLabels...)
	sort.Strings(sortedDB)

	require.Equal(t, sortedDB, goLabels,
		"DRIFT TD-172: validWalletTypes (internal/wallet/handler.go) != wallet_type_enum (pg_enum). "+
			"Enum baru tanpa whitelist update = label valid ditolak; whitelist tanpa enum = SQLSTATE 22P02. "+
			"Update keduanya dalam satu change.")
}
