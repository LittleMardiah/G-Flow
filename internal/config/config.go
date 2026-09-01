// Package config menyediakan pemuatan dan validasi konfigurasi aplikasi
// dari environment variable. Memisahkan konfigurasi DB ke struct milik
// package db agar konsisten dengan db.NewDB(cfg).
package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/g-flow/g-flow/internal/db"
)

// Config adalah konfigurasi keseluruhan aplikasi.
type Config struct {
	DB  db.Config
	App AppConfig
}

// LogLevel default per environment (TD-013): development = debug,
// production = info. Bisa di-override explicit via LOG_LEVEL env.
const (
	DefaultLogLevelDevelopment = "debug"
	DefaultLogLevelProduction  = "info"
)

// AppConfig adalah konfigurasi spesifik aplikasi (HTTP & auth & logging).
type AppConfig struct {
	Port      string
	Env       string
	RedisURL  string
	JWTSecret string
	LogLevel  string
}

// Load membaca konfigurasi dari environment variable.
// Mengembalikan error jika field wajib (DATABASE_URL) kosong.
func Load() (*Config, error) {
	cfg := &Config{
		DB: db.Config{
			DatabaseURL: os.Getenv("DATABASE_URL"),
		},
		App: AppConfig{
			Port:      os.Getenv("PORT"),
			Env:       os.Getenv("ENV"),
			RedisURL:  os.Getenv("REDIS_URL"),
			JWTSecret: os.Getenv("JWT_SECRET"),
			LogLevel:  os.Getenv("LOG_LEVEL"),
		},
	}

	if err := cfg.App.ApplyLogLevelDefault(); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadWithDefaults membaca env dan mengisi nilai default untuk field yang
// tidak di-set (Port=8080, Env=development, dan pool DB standar) lalu
// memvalidasi field wajib.
func LoadWithDefaults() (*Config, error) {
	cfg := &Config{
		DB: db.DefaultConfig(),
		App: AppConfig{
			Port:      os.Getenv("PORT"),
			Env:       os.Getenv("ENV"),
			RedisURL:  os.Getenv("REDIS_URL"),
			JWTSecret: os.Getenv("JWT_SECRET"),
			LogLevel:  os.Getenv("LOG_LEVEL"),
		},
	}

	if cfg.App.Port == "" {
		cfg.App.Port = "8080"
	}
	if cfg.App.Env == "" {
		cfg.App.Env = "development"
	}

	if err := cfg.App.ApplyLogLevelDefault(); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate memastikan field wajib terisi. Saat ini hanya DATABASE_URL yang
// wajib untuk tahap awal; Redis/JWT diecek terpisah saat modul terkait aktif.
func (c *Config) Validate() error {
	if c.DB.DatabaseURL == "" {
		return fmt.Errorf("config: DATABASE_URL wajib diisi")
	}
	return nil
}

// ApplyLogLevelDefault (TD-013) menetapkan LogLevel default berdasarkan Env
// bila LOG_LEVEL tidak di-set secara eksplisit:
//   - development → debug
//   - production / selain development → info
//
// Nilai yang valid hanya salah satu dari: debug, info, warn, error.
func (a *AppConfig) ApplyLogLevelDefault() error {
	if a.LogLevel == "" {
		switch a.Env {
		case "", "development", "dev", "local", "test":
			a.LogLevel = DefaultLogLevelDevelopment
		default:
			a.LogLevel = DefaultLogLevelProduction
		}
	}
	switch a.LogLevel {
	case "debug", "info", "warn", "error":
		return nil
	default:
		return fmt.Errorf("config: LOG_LEVEL tidak valid (%q), gunakan debug/info/warn/error", a.LogLevel)
	}
}

// SlogLevel memetakan LogLevel string ke slog.Level (debug/info/warn/error).
func (a *AppConfig) SlogLevel() slog.Level {
	switch a.LogLevel {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
