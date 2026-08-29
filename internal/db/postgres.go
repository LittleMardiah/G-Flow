// Package db menyediakan koneksi pool PostgreSQL (pgxpool) untuk seluruh aplikasi.
// Konfigurasi mengikuti TDD v2.0-FINAL: MaxConns=15 (Supabase free tier),
// statement_timeout untuk mencegah connection exhaustion.
package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config berisi pengaturan koneksi database. Dapat diisi langsung atau di-load
// dari environment. DSN diambil dari field DatabaseURL (postgres://...).
type Config struct {
	DatabaseURL string
	MaxConns    int32
	MinConns    int32
	MaxConnIdle time.Duration
	MaxConnLife time.Duration
}

// DefaultConfig mengembalikan Config dengan nilai standar Phase 1.
func DefaultConfig() Config {
	return Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		MaxConns:    15,
		MinConns:    2,
		MaxConnIdle: 5 * time.Minute,
		MaxConnLife: 30 * time.Minute,
	}
}

// NewDB membuat dan memvalidasi koneksi pool PostgreSQL.
// Pool dibuat secara lazy; Ping dipanggil di sini untuk memastikan
// koneksi benar-benar tersedia sebelum aplikasi melayani request.
func NewDB(cfg Config) (*pgxpool.Pool, error) {
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("db: DATABASE_URL tidak boleh kosong")
	}
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 15
	}
	if cfg.MinConns < 0 {
		cfg.MinConns = 0
	}
	if cfg.MaxConnIdle <= 0 {
		cfg.MaxConnIdle = 5 * time.Minute
	}
	if cfg.MaxConnLife <= 0 {
		cfg.MaxConnLife = 30 * time.Minute
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: gagal parse config: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdle
	poolCfg.MaxConnLifetime = cfg.MaxConnLife

	// Batasi durasi query agar tidak menahan koneksi saat DB lambat/macet.
	poolCfg.ConnConfig.RuntimeParams["statement_timeout"] = "3000ms"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: gagal membuat pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping gagal: %w", err)
	}

	return pool, nil
}

// Close menutup pool dan melepaskan semua koneksi ke database.
// Dipanggil saat aplikasi shutdown (graceful shutdown).
func Close(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}

// Ping melakukan pengecekan koneksi cepat ke database.
// Dapat digunakan oleh endpoint /health dan /ready.
func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("db: pool nil")
	}
	return pool.Ping(ctx)
}
