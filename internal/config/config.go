// Package config menyediakan pemuatan dan validasi konfigurasi aplikasi
// dari environment variable. Memisahkan konfigurasi DB ke struct milik
// package db agar konsisten dengan db.NewDB(cfg).
package config

import (
	"fmt"
	"os"

	"github.com/g-flow/g-flow/internal/db"
)

// Config adalah konfigurasi keseluruhan aplikasi.
type Config struct {
	DB  db.Config
	App AppConfig
}

// AppConfig adalah konfigurasi spesifik aplikasi (HTTP & auth).
type AppConfig struct {
	Port      string
	Env       string
	RedisURL  string
	JWTSecret string
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
		},
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
		},
	}

	if cfg.App.Port == "" {
		cfg.App.Port = "8080"
	}
	if cfg.App.Env == "" {
		cfg.App.Env = "development"
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
