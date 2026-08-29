// Package location berisi akses data (repository) untuk Driver Location
// Tracking (ROADMAP 02 §2.6). Repository bertanggung jawab memetakan lokasi
// driver ke tabel driver_locations dan menemukan driver terdekat memakai
// extension earthdistance (ll_to_earth) — sudah dibuat di migration 001.
//
// Rujukan skema: MIGRATION 004_ride_orders.up.sql (LOCKED) — driver_locations
// dan kolom users (is_online, working_status, status).
package location

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Error definitions tingkat package.
var (
	ErrDriverNotFound = errors.New("driver not found")
)

// RepoDB adalah subset operasi pool yang dipakai Repository untuk query.
// Dipenuhi oleh *pgxpool.Pool (produksi) dan pgxmock (test).
type RepoDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// NearbyDriver adalah representasi satu driver terdekat yang dikembalikan
// query GetNearbyDrivers (termasuk jarak geodesik dalam km).
type NearbyDriver struct {
	DriverID   uuid.UUID
	Lat        float64
	Lng        float64
	DistanceKm float64
}

// Repository adalah akses data untuk tabel driver_locations & users.
type Repository struct {
	db RepoDB
}

// NewRepository membuat Repository baru dengan koneksi database.
func NewRepository(db RepoDB) *Repository {
	return &Repository{db: db}
}

// UpsertLocation meng-insert atau meng-update lokasi driver terbaru.
// ON CONFLICT (driver_id) DO UPDATE memastikan tiap driver hanya punya satu
// baris lokasi yang selalu menunjuk posisi paling baru.
func (r *Repository) UpsertLocation(ctx context.Context, driverID uuid.UUID, lat, lng float64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO driver_locations (driver_id, current_lat, current_lng, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (driver_id) DO UPDATE
		SET current_lat = EXCLUDED.current_lat,
		    current_lng = EXCLUDED.current_lng,
		    updated_at  = NOW()
	`, driverID, lat, lng)
	return err
}

// GetNearbyDrivers mencari driver aktif (IDLE & online) dalam radius radiusKm
// dari koordinat (lat,lng). Memakai earthdistance.ll_to_earth untuk filter
// jarak dan join ke users agar hanya driver memenuhi syarat yang dikembalikan.
func (r *Repository) GetNearbyDrivers(ctx context.Context, lat, lng float64, radiusKm float64) ([]NearbyDriver, error) {
	rows, err := r.db.Query(ctx, `
		SELECT dl.driver_id, dl.current_lat, dl.current_lng,
		       ROUND(
		         earth_distance(ll_to_earth($1, $2), ll_to_earth(dl.current_lat, dl.current_lng)) / 1000.0,
		         3
		       ) AS distance_km
		FROM driver_locations dl
		JOIN users u ON u.id = dl.driver_id
		WHERE u.user_type = 'driver'
		  AND u.working_status = 'IDLE'
		  AND u.is_online = TRUE
		  AND u.status = 'ACTIVE'
		  AND earth_distance(ll_to_earth($1, $2), ll_to_earth(dl.current_lat, dl.current_lng)) / 1000.0 <= $3
		ORDER BY distance_km ASC
	`, lat, lng, radiusKm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var drivers []NearbyDriver
	for rows.Next() {
		var d NearbyDriver
		if err := rows.Scan(&d.DriverID, &d.Lat, &d.Lng, &d.DistanceKm); err != nil {
			return nil, err
		}
		drivers = append(drivers, d)
	}
	return drivers, rows.Err()
}
