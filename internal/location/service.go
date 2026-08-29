// Package location — Service Layer (ROADMAP 02 §2.6: Driver Location Tracking).
//
// Service menangani business logic pelacakan lokasi driver dengan pola
// dual-layer: Redis sebagai L1 (hash driver:location:{driverID} + TTL 60s)
// dan PostgreSQL sebagai L2 (driver_locations, di-flush oleh Worker).
// Setiap operasi memiliki fallback ke DB jika Redis tidak tersedia/gagal
// (graceful degradation, selaras dengan modul wallet & ride).
//
// Untuk pencarian driver terdekat, service mencoba membaca seluruh lokasi
// segar dari Redis lalu menghitung jarak haversine di aplikasi; jika Redis
// kosong/error, fallback ke query spatial earthdistance di Repository.
package location

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
)

// Constanta domain Driver Location Tracking.
const (
	// Prefix & TTL key Redis lokasi driver.
	redisKeyPrefix = "driver:location:"
	redisTTL       = 60 * time.Second

	// Field hash Redis untuk satu lokasi driver.
	fieldLat       = "lat"
	fieldLng       = "lng"
	fieldUpdatedAt = "updated_at"

	// Ambang kesegaran lokasi (sama dengan TTL): lokasi dianggap "hidup"
	// bila updated_at tidak lebih tua dari 60 detik.
	freshWindow = 60 * time.Second

	// Earth radius (km) untuk haversine.
	earthRadiusKm = 6371.0
)

// Error definitions untuk service layer lokasi.
var (
	ErrInvalidCoordinates = errors.New("invalid coordinates")
	ErrInvalidRadius      = errors.New("invalid radius")
)

// Repo adalah kontrak repository yang dibutuhkan Service. Dipenuhi oleh
// *Repository (internal/location/repository.go); dijadikan interface agar
// mudah di-mock pada unit test.
type Repo interface {
	UpsertLocation(ctx context.Context, driverID uuid.UUID, lat, lng float64) error
	GetNearbyDrivers(ctx context.Context, lat, lng float64, radiusKm float64) ([]NearbyDriver, error)
}

// DB adalah subset operasi pool yang dipakai Service untuk fallback.
// Dipenuhi oleh *pgxpool.Pool (produksi) dan pgxmock (test).
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// RedisClient adalah subset operasi Redis yang dipakai Service (L1).
// Dipenuhi oleh *redis.Client.
type RedisClient interface {
	HSet(ctx context.Context, key string, values ...any) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
	HGet(ctx context.Context, key string, field string) *redis.StringCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
}

// redisStorage adalah penyimpan lokasi driver hasil baca dari Redis.
type redisStorage struct {
	driverID  uuid.UUID
	lat       float64
	lng       float64
	updatedAt time.Time
}

// Service adalah business logic untuk modul lokasi driver.
type Service struct {
	repo Repo
	rdb  RedisClient
	db   DB
}

// NewService membuat Service baru dengan dependency injection.
func NewService(repo Repo, rdb RedisClient, db DB) *Service {
	return &Service{repo: repo, rdb: rdb, db: db}
}

// UpdateLocation menyimpan lokasi terbaru driver.
//
//  1. Tulis ke Redis hash driver:location:{driverID} (field lat, lng,
//     updated_at) lalu set TTL 60 detik.
//  2. Jika Redis tidak tersedia (nil) atau gagal, fallback langsung ke
//     repository.UpsertLocation (L2) agar lokasi tetap tercatat di DB.
func (s *Service) UpdateLocation(ctx context.Context, driverID uuid.UUID, lat, lng float64) error {
	if !validLatLng(lat, lng) {
		return ErrInvalidCoordinates
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if s.rdb != nil {
		key := redisKey(driverID)
		if err := s.rdb.HSet(ctx, key, fieldLat, lat, fieldLng, lng, fieldUpdatedAt, now).Err(); err == nil {
			// TTL berhasil di-set → selesai (data segar di Redis).
			if ttlErr := s.rdb.Expire(ctx, key, redisTTL).Err(); ttlErr == nil {
				return nil
			}
			// Expire gagal walau HSet sukses: tetap catat ke DB baseline.
		} else {
			// HSet gagal → fallback L2.
			return s.repo.UpsertLocation(ctx, driverID, lat, lng)
		}
	}

	return s.repo.UpsertLocation(ctx, driverID, lat, lng)
}

// GetNearbyDrivers mencari driver terdekat dari (lat,lng) dalam radiusKm.
//
//  1. Coba baca lokasi segar dari Redis (SCAN driver:location:*), hitung
//     jarak haversine di aplikasi, filter radius & kesegaran.
//  2. Jika Redis nil/error/kosong, fallback ke repository.GetNearbyDrivers
//     (query spatial earthdistance di DB).
func (s *Service) GetNearbyDrivers(ctx context.Context, lat, lng float64, radiusKm float64) ([]NearbyDriver, error) {
	if !validLatLng(lat, lng) {
		return nil, ErrInvalidCoordinates
	}
	if radiusKm <= 0 {
		return nil, ErrInvalidRadius
	}

	if s.rdb != nil {
		if drivers, fresh, err := s.readNearbyRedis(ctx, lat, lng, radiusKm); err == nil && fresh {
			return drivers, nil
		}
	}

	return s.repo.GetNearbyDrivers(ctx, lat, lng, radiusKm)
}

// readNearbyRedis membaca lokasi dari Redis, menghitung jarak, dan memfilter
// radius. Return (drivers, true, nil) bila Redis sehat & menemukan setidaknya
// satu driver segar dalam radius; jika Redis error/kosong, kembalikan false
// agar pemanggil fallback ke DB.
func (s *Service) readNearbyRedis(ctx context.Context, lat, lng float64, radiusKm float64) ([]NearbyDriver, bool, error) {
	storages, err := s.scanLocations(ctx)
	if err != nil {
		return nil, false, err
	}

	var hits []NearbyDriver
	now := time.Now().UTC()
	for _, st := range storages {
		// Hanya lokasi yang masih segar (tidak lebih tua dari TTL).
		if now.Sub(st.updatedAt) > freshWindow {
			continue
		}
		dist := haversineKm(lat, lng, st.lat, st.lng)
		if dist > radiusKm {
			continue
		}
		hits = append(hits, NearbyDriver{
			DriverID:   st.driverID,
			Lat:        st.lat,
			Lng:        st.lng,
			DistanceKm: dist,
		})
	}

	if len(hits) == 0 {
		return nil, false, nil
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].DistanceKm < hits[j].DistanceKm })
	return hits, true, nil
}

// scanLocations memindai seluruh key driver:location:* di Redis dan membaca
// hash-nya menjadi slice redisStorage. Mengabaikan key yang rusak/parsial.
func (s *Service) scanLocations(ctx context.Context) ([]redisStorage, error) {
	var storages []redisStorage
	var cursor uint64
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, redisKeyPrefix+"*", 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			st, ok := s.readHash(ctx, key)
			if !ok {
				continue
			}
			storages = append(storages, st)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return storages, nil
}

// readHash membaca satu key hash menjadi redisStorage. Mengembalikan ok=false
// jika field tidak lengkap atau parsing gagal.
func (s *Service) readHash(ctx context.Context, key string) (redisStorage, bool) {
	idStr := trimPrefix(key)
	if idStr == "" {
		return redisStorage{}, false
	}
	driverID, err := uuid.Parse(idStr)
	if err != nil {
		return redisStorage{}, false
	}

	latStr, err := s.rdb.HGet(ctx, key, fieldLat).Result()
	if err != nil {
		return redisStorage{}, false
	}
	lngStr, err := s.rdb.HGet(ctx, key, fieldLng).Result()
	if err != nil {
		return redisStorage{}, false
	}
	updatedStr, err := s.rdb.HGet(ctx, key, fieldUpdatedAt).Result()
	if err != nil {
		return redisStorage{}, false
	}

	var lat, lng float64
	if v, err := strconv.ParseFloat(latStr, 64); err != nil {
		return redisStorage{}, false
	} else {
		lat = v
	}
	if v, err := strconv.ParseFloat(lngStr, 64); err != nil {
		return redisStorage{}, false
	} else {
		lng = v
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, updatedStr)
	if err != nil {
		return redisStorage{}, false
	}

	return redisStorage{
		driverID:  driverID,
		lat:       lat,
		lng:       lng,
		updatedAt: updatedAt,
	}, true
}

// redisKey membangun key Redis untuk driver tertentu.
func redisKey(driverID uuid.UUID) string {
	return redisKeyPrefix + driverID.String()
}

// trimPrefix membuang prefix "driver:location:" dari key Redis.
func trimPrefix(key string) string {
	const p = redisKeyPrefix
	if len(key) >= len(p) && key[:len(p)] == p {
		return key[len(p):]
	}
	return key
}

// validLatLng memastikan koordinat berada pada rentang geografis valid.
func validLatLng(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

// haversineKm menghitung jarak geodesik (km) antara dua koordinat.
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	degToRad := math.Pi / 180
	dLat := (lat2 - lat1) * degToRad
	dLng := (lng2 - lng1) * degToRad
	lat1Rad := lat1 * degToRad
	lat2Rad := lat2 * degToRad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Asin(math.Sqrt(a))
	return earthRadiusKm * c
}
