// Command api adalah entry point aplikasi backend G-Flow.
// Tahap ini: muat konfigurasi, buka pool DB, inisialisasi Redis (opsional),
// dan jalankan router Gin untuk F001 wallet (top-up/transfer/balance/webhook).
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/g-flow/g-flow/internal/auth"
	"github.com/g-flow/g-flow/internal/config"
	"github.com/g-flow/g-flow/internal/db"
	"github.com/g-flow/g-flow/internal/food"
	"github.com/g-flow/g-flow/internal/location"
	"github.com/g-flow/g-flow/internal/middleware"
	"github.com/g-flow/g-flow/internal/ride"
	"github.com/g-flow/g-flow/internal/wallet"
)

func main() {
	// Structured logging (F014): default slog logger dalam format JSON ke
	// stdout agar bisa di-parse oleh pipeline observability.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("gagal memuat config: %v", err)
	}

	pool, err := db.NewDB(cfg.DB)
	if err != nil {
		log.Fatalf("gagal terhubung ke database: %v", err)
	}
	defer db.Close(pool)

	ctx := context.Background()

	// Redis client (opsional). Jika REDIS_URL kosong/tidak valid -> nil
	// (graceful degradation: service tetap berjalan tanpa L1 idempotency).
	rdb := initRedis(ctx, cfg.App.RedisURL)

	// Auth services: JWT (HS256) + token blacklist via Redis.
	jwtService := auth.NewJWTService(cfg.App.JWTSecret)
	blacklistService := auth.NewBlacklistService(rdb)

	// Wire up dependencies: auth handler (login/register/logout + RBAC).
	authHandler := auth.NewHandler(jwtService, blacklistService, pool)

	// Wire up dependencies wallet.
	repo := wallet.NewRepository(pool)
	ledger := wallet.NewLedgerService(pool)
	svc := wallet.NewService(repo, ledger, rdb, pool)
	handler := wallet.NewHandler(svc)

	// Wire up dependencies ride (F004 booking).
	rideRepository := ride.NewRepository(pool)
	rideLedger := wallet.NewLedgerService(pool)
	rideService := ride.NewService(rideRepository, rideLedger, rdb, pool)
	rideHandler := ride.NewHandler(rideService)

	// Wire up dependencies location (2.6 Driver Location Tracking).
	// L1 Redis (hash driver:location:{id} + TTL) → L2 PostgreSQL. Worker
	// background mem-flush lokasi dari Redis ke DB setiap 5 detik.
	locationRepo := location.NewRepository(pool)
	locationService := location.NewService(locationRepo, rdb, pool)
	locationHandler := location.NewHandler(locationService)
	locationWorker := location.NewWorker(locationService, rdb, locationRepo)

	// Wire up dependencies food (Phase 3, Task 3.2: Merchant Onboarding &
	// Catalog Management). Service memakai pool langsung untuk transaksi
	// merchant register (membuat merchant + wallet MERCHANT secara atomik).
	foodRepository := food.NewRepository(pool)
	foodService := food.NewService(foodRepository, pool)
	foodHandler := food.NewHandler(foodService)

	// Router. gin.New() + middleware eksplisit: Recovery (panic + stack trace)
	// dan Logger (JSON terstruktur / F014) dipasang sebelum route apapun.
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"ready": false, "db": "down"})
			return
		}
		if rdb != nil {
			if err := rdb.Ping(ctx).Err(); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"ready": false, "redis": "down"})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"ready": true, "db": "up", "redis": boolStatus(rdb != nil)})
	})

	// Webhook (tanpa auth).
	r.POST("/webhooks/topup", handler.ProcessTopUpWebhook)

	// Auth endpoint publik (tanpa auth): register & login.
	r.POST("/api/v1/auth/register", authHandler.Register)
	r.POST("/api/v1/auth/login", authHandler.Login)

	// Driver location: cari driver terdekat → auth optional / publik.
	// Lat/lng/radius diambil dari query param; tidak butuh identitas user.
	r.GET("/api/v1/drivers/nearby", locationHandler.GetNearbyDrivers)

	// API v1 (dilindungi auth JWT + blacklist).
	api := r.Group("/api/v1", middleware.AuthMiddleware(jwtService, blacklistService))
	{
		api.POST("/auth/logout", authHandler.Logout)
		api.POST("/rides/book", rideHandler.BookRide)
		api.POST("/rides/:order_id/accept", rideHandler.AcceptOrder)
		api.PATCH("/rides/:order_id/status", rideHandler.UpdateStatus)
		api.POST("/wallets/:wallet_id/topup", handler.TopUp)
		api.POST("/wallets/:wallet_id/transfer", handler.Transfer)
		api.GET("/wallets/:wallet_id/balance", handler.GetBalance)
		// Driver update lokasi: auth wajib + role driver.
		api.POST("/drivers/location", auth.RBACMiddleware("driver"), locationHandler.UpdateLocation)
	}

	// G-Food: Merchant Onboarding & Catalog Management (Task 3.2).
	// Semua resource merchant dilindungi Auth + RBAC role 'merchant' agar
	// hanya akun merchant yang bisa mengelola. Ownership per-resource (apakah
	// merchant tersebut milik user yang login) divalidasi di Service.
	merchants := r.Group("/api/v1/merchants", middleware.AuthMiddleware(jwtService, blacklistService), auth.RBACMiddleware("merchant"))
	{
		merchants.POST("/register", foodHandler.RegisterMerchant)
		merchants.GET("/:id", foodHandler.GetMerchant)
		merchants.PATCH("/:id", foodHandler.UpdateMerchant)
		merchants.POST("/:id/menus", foodHandler.CreateMenu)
		merchants.PATCH("/:id/menus/:menu_id", foodHandler.UpdateMenu)
		merchants.DELETE("/:id/menus/:menu_id", foodHandler.DeleteMenu)
		merchants.GET("/:id/menus", foodHandler.GetMenus)
		merchants.POST("/:id/items", foodHandler.CreateItem)
		merchants.PATCH("/:id/items/:item_id", foodHandler.UpdateItem)
		merchants.DELETE("/:id/items/:item_id", foodHandler.DeleteItem)
		merchants.GET("/:id/items", foodHandler.GetItems)
	}

	// Jalankan background worker flush lokasi driver (2.6) sebagai goroutine.
	// Context terpisah agar graceful shutdown bisa menghentikan loop worker.
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()
	go locationWorker.FlushLoop(workerCtx)

	log.Printf("Server running on port %s (env=%s)", cfg.App.Port, cfg.App.Env)
	if err := r.Run(":" + cfg.App.Port); err != nil {
		log.Fatalf("gagal menjalankan server: %v", err)
	}
}

// initRedis membuat *redis.Client dari URL. Mengembalikan nil jika URL kosong,
// tidak valid, atau tidak dapat di-ping (graceful degradation).
func initRedis(ctx context.Context, url string) *redis.Client {
	if url == "" {
		log.Println("warning: REDIS_URL kosong, lanjut tanpa Redis L1")
		return nil
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("warning: REDIS_URL tidak valid (%v), lanjut tanpa Redis L1", err)
		return nil
	}
	rdb := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		log.Printf("warning: Redis tidak dapat di-ping (%v), lanjut tanpa Redis L1", err)
		_ = rdb.Close()
		return nil
	}
	log.Println("Redis terhubung")
	return rdb
}

func boolStatus(b bool) string {
	if b {
		return "up"
	}
	return "disabled"
}
