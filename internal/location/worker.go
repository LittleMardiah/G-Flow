// Package location — Background Worker (flush lokasi driver).
//
// Worker secara berkala membaca seluruh lokasi driver yang masih ada di Redis
// (L1) lalu mem-flush-nya ke PostgreSQL (L2) lewat Repository.UpsertLocation.
// Ini memastikan data lokasi tersimpan persisten walau TTL Redis belum
// kedaluwarsa, dan menjadi baseline bagi query spatial GetNearbyDrivers.
package location

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// FlushInterval adalah periode loop worker mem-flush lokasi (5 detik).
const FlushInterval = 5 * time.Second

// Worker adalah background worker yang mem-flush lokasi driver dari Redis.
// Dependency Service & Repository mengikuti spec; FlushLoop memakai
// RedisClient (baca) dan Repository (tulis).
type Worker struct {
	svc  *Service
	rdb  RedisClient
	repo Repo
}

// NewWorker membuat Worker baru dengan dependency injection.
func NewWorker(svc *Service, rdb RedisClient, repo Repo) *Worker {
	return &Worker{svc: svc, rdb: rdb, repo: repo}
}

// FlushLoop menjalankan loop flush setiap FlushInterval sampai ctx selesai.
// Dipanggil sebagai goroutine dari main.go (dengan context cancel untuk
// graceful shutdown).
func (w *Worker) FlushLoop(ctx context.Context) {
	ticker := time.NewTicker(FlushInterval)
	defer ticker.Stop()

	log.Println("location worker: flush loop dimulai (interval 5s)")

	for {
		select {
		case <-ctx.Done():
			log.Println("location worker: context selesai, hentikan flush loop")
			return
		case <-ticker.C:
			w.flushOnce(ctx)
		}
	}
}

// flushOnce memindai lokasi driver di Redis dan mem-flush ke DB.
// Menghitung jumlah driver yang sukses di-flush lalu mencatat log.
func (w *Worker) flushOnce(ctx context.Context) {
	if w.rdb == nil {
		return
	}

	storages, err := w.scan(ctx)
	if err != nil {
		log.Printf("location worker: gagal memindai Redis: %v", err)
		return
	}

	if len(storages) == 0 {
		return
	}

	flushed := 0
	for _, st := range storages {
		if err := w.repo.UpsertLocation(ctx, st.driverID, st.lat, st.lng); err != nil {
			log.Printf("location worker: gagal flush lokasi driver %s: %v", st.driverID, err)
			continue
		}
		flushed++
	}

	log.Printf("location worker: di-flush %d/%d lokasi driver ke DB", flushed, len(storages))
}

// storage adalah representasi satu lokasi driver hasil baca Redis di worker.
type storage struct {
	driverID uuid.UUID
	lat      float64
	lng      float64
}

// scan memindai seluruh key driver:location:* dan membaca hash-nya.
func (w *Worker) scan(ctx context.Context) ([]storage, error) {
	var out []storage
	var cursor uint64
	for {
		keys, next, err := w.rdb.Scan(ctx, cursor, redisKeyPrefix+"*", 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			st, ok := w.read(ctx, key)
			if !ok {
				continue
			}
			out = append(out, st)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return out, nil
}

// read membaca satu key hash menjadi storage. Mengembalikan ok=false jika
// key tidak berbentuk UUID atau field tidak lengkap.
func (w *Worker) read(ctx context.Context, key string) (storage, bool) {
	idStr := trimPrefix(key)
	if idStr == "" {
		return storage{}, false
	}
	driverID, err := uuid.Parse(idStr)
	if err != nil {
		return storage{}, false
	}

	latStr, err := w.rdb.HGet(ctx, key, fieldLat).Result()
	if err != nil {
		return storage{}, false
	}
	lngStr, err := w.rdb.HGet(ctx, key, fieldLng).Result()
	if err != nil {
		return storage{}, false
	}

	lat, ok1 := parseFloatValue(latStr)
	lng, ok2 := parseFloatValue(lngStr)
	if !ok1 || !ok2 {
		return storage{}, false
	}

	return storage{driverID: driverID, lat: lat, lng: lng}, true
}

// parseFloatValue menafsirkan string sebagai float64 (bisa "123.456").
func parseFloatValue(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
