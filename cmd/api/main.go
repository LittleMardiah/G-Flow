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

	"github.com/g-flow/g-flow/internal/admin"
	"github.com/g-flow/g-flow/internal/auth"
	"github.com/g-flow/g-flow/internal/config"
	"github.com/g-flow/g-flow/internal/db"
	"github.com/g-flow/g-flow/internal/driver"
	"github.com/g-flow/g-flow/internal/food"
	"github.com/g-flow/g-flow/internal/location"
	"github.com/g-flow/g-flow/internal/middleware"
	"github.com/g-flow/g-flow/internal/ride"
	"github.com/g-flow/g-flow/internal/send"
	"github.com/g-flow/g-flow/internal/wallet"
	"github.com/g-flow/g-flow/internal/worker"
)

func main() {
	// Muat config terlebih dahulu agar level logger bisa disesuaikan dengan
	// environment (TD-013): development=debug, production=info, dan bisa
	// di-override via LOG_LEVEL env.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("gagal memuat config: %v", err)
	}

	// Structured logging (F014 / TD-013): default slog logger dalam format
	// JSON ke stdout agar bisa di-parse oleh pipeline observability. Level
	// logger ditentukan dari cfg.App.LogLevel (via ENV LOG_LEVEL / ENV).
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.App.SlogLevel()})
	slog.SetDefault(slog.New(jsonHandler))
	pool, err := db.NewDB(cfg.DB)
	if err != nil {
		log.Fatalf("gagal terhubung ke database: %v", err)
	}
	defer db.Close(pool)

	ctx := context.Background()

	// Redis client (opsional). Jika REDIS_URL kosong/tidak valid -> nil
	// (graceful degradation: service tetap berjalan tanpa L1 idempotency).
	rdb := initRedis(ctx, cfg.App.RedisURL)

	// TD-014: Redis graceful shutdown — tutup client saat server berhenti.
	if rdb != nil {
		defer rdb.Close()
	}

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
	// Catalog Management; Task 3.3: Food Order Creation & Cart). Service
	// memakai pool untuk transaksi (merchant register atomik; food order +
	// escrow atomik), Redis untuk idempotency L1, dan LedgerService untuk
	// double-entry escrow FOOD_ESCROW / FOOD_REFUND.
	foodRepository := food.NewRepository(pool)
	foodLedger := wallet.NewLedgerService(pool)
	foodService := food.NewService(foodRepository, pool, rdb, foodLedger)
	foodHandler := food.NewHandler(foodService)

	// Wire up dependencies send (Phase 3, Task 3.5: G-Send Order Creation &
	// Pricing). Service memakai pool untuk transaksi (order + stops + escrow
	// atomik), Redis untuk idempotency L1, dan LedgerService untuk double-entry
	// escrow SEND_ESCROW / SEND_REFUND.
	sendRepository := send.NewRepository(pool)
	sendLedger := wallet.NewLedgerService(pool)
	sendService := send.NewService(sendRepository, pool, rdb, sendLedger)
	sendHandler := send.NewHandler(sendService)

	// Wire up dependencies auto-cancel worker (Phase 3, Task 3.7). Worker
	// memakai pool DB, LedgerService untuk refund escrow double-entry, dan
	// Redis untuk distributed lock (SET NX) agar aman di multi-instance.
	workerRepository := worker.NewRepository(pool)
	workerLedger := wallet.NewLedgerService(pool)
	autoCancelWorker := worker.NewWorker(workerRepository, pool, rdb, workerLedger)

	// Wire up dependencies driver available orders (TD-009). Membaca lokasi
	// driver dari Redis (L1) dengan fallback ke PostgreSQL (driver_locations),
	// lalu men-query order tersedia dari semua layanan (ride/food/send).
	driverRepository := driver.NewRepository(pool)
	driverService := driver.NewService(driverRepository, rdb)
	driverHandler := driver.NewHandler(driverService)

	// Wire up dependencies admin (Task 4.2.4 Transaction Reversal).
	// Service memakai pool untuk transaksi reversal (clawback proporsional +
	// shortfall -> SYSTEM_RECEIVABLE_OVERDRAFT), Redis untuk lockout 2FA (L1)
	// dengan fallback PostgreSQL (L2/persisten), dan validator 2FA statis MVP.
	// TD-023 (FIXED STEP 1B): secret 2FA dibaca dari env ADMIN_2FA_SECRET;
	// fail-fast (panic) saat ENV=production dan secret kosong.
	adminRepository := admin.NewRepository(pool)
	adminService := admin.NewService(adminRepository, pool, slog.Default())
	adminTwoFA := admin.NewStaticTwoFactorValidator(os.Getenv("ADMIN_2FA_SECRET"))
	adminHandler := admin.NewHandler(adminService, pool, rdb, slog.Default(), adminTwoFA, jwtService)

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

	// Admin login publik (tanpa auth): khusus role admin, divalidasi di Service.
	r.POST("/api/v1/admin/login", adminHandler.AdminLogin)

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
		// Driver available orders (TD-009): auth wajib + role driver.
		api.GET("/drivers/available-orders", auth.RBACMiddleware("driver"), driverHandler.GetAvailableOrders)
	}

	// G-Food: catalog discovery publik (Task 3.3 catalog search & retrieval).
	// Auth OPSIONAL: pemilik merchant (token valid) melihat item penuh miliknya;
	// public hanya merchant ACTIVE + item is_available.
	catalog := r.Group("/api/v1", middleware.OptionalAuthMiddleware(jwtService, blacklistService))
	{
		catalog.GET("/merchants", foodHandler.GetMerchants)
		catalog.GET("/merchants/:id/items", foodHandler.GetMerchantItems)
	}

	// G-Food: merchant onboarding & catalog management (Task 3.2).
	// Semua resource merchant dilindungi Auth + RBAC role 'merchant' agar
	// hanya akun merchant yang bisa mengelola. Ownership per-resource (apakah
	// merchant tersebut milik user yang login) divalidasi di Service.
	// Catatan: GET /merchants/:id/items menabrak route publik di atas,
	// sehingga katalog item owner sekarang dilayani oleh /:id/items publik
	// (GetMerchantItems) — dihapus dari grup RBAC.
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
	}

	// G-Food: food orders (Task 3.3). Auth wajib; RBAC: POST & GET "" hanya
	// customer. Detail/status bisa diakses customer, merchant owner, atau
	// driver tertunjuk (ownership divalidasi di Service).
	foodOrders := r.Group("/api/v1/food-orders", middleware.AuthMiddleware(jwtService, blacklistService))
	{
		foodOrders.POST("", auth.RBACMiddleware("customer"), foodHandler.CreateFoodOrder)
		foodOrders.GET("", auth.RBACMiddleware("customer"), foodHandler.GetFoodOrderHistory)
		foodOrders.GET("/:id", foodHandler.GetFoodOrder)
		foodOrders.PATCH("/:id", foodHandler.UpdateFoodOrderStatus)
	}

	// G-Send: send orders (Task 3.5). Auth wajib; POST hanya customer.
	// Detail/status bisa diakses sender (pemilik) atau driver tertunjuk
	// (ownership divalidasi di Service). GET history ditunda ke Task 3.6.
	sendOrders := r.Group("/api/v1/send-orders", middleware.AuthMiddleware(jwtService, blacklistService))
	{
		sendOrders.POST("", auth.RBACMiddleware("customer"), sendHandler.CreateSendOrder)
		sendOrders.GET("/:id", sendHandler.GetSendOrder)
		sendOrders.PATCH("/:id", sendHandler.UpdateSendOrderStatus)
		sendOrders.POST("/:id/accept", auth.RBACMiddleware("driver"), sendHandler.AcceptSendOrder)
		sendOrders.PATCH("/:id/stops/:stop_id", auth.RBACMiddleware("driver"), sendHandler.UpdateSendOrderStop)
	}

	// Admin endpoints (Task 4.2.4 Transaction Reversal).
	// Semua resource admin dilindungi Auth + RBAC role 'admin'; 2FA tambahan
	// divalidasi di dalam handler via header X-Admin-2FA-Token.
	adminGroup := r.Group("/api/v1/admin",
		middleware.AuthMiddleware(jwtService, blacklistService),
		auth.RBACMiddleware("admin"),
	)
	{
		adminGroup.GET("/transactions/:id", adminHandler.GetTransaction)
		adminGroup.POST("/transactions/:id/reverse", adminHandler.ReverseTransaction)
	}

	// Jalankan background worker flush lokasi driver (2.6) sebagai goroutine.
	// Context terpisah agar graceful shutdown bisa menghentikan loop worker.
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()
	go locationWorker.FlushLoop(workerCtx)

	// Jalankan background worker auto-cancel G-Food & G-Send (3.7) sebagai
	// goroutine. Loop 1 menit + Redis distributed lock (SET NX) memastikan
	// hanya satu instance yang mengeksekusi sweep pada satu waktu. Dalam loop
	// yang sama juga berjalan purge idempotency_cache kedaluwarsa (TD-002,
	// setiap 1 jam) sebelum/ketika sweep auto-cancel berjalan.
	go autoCancelWorker.Run(workerCtx)

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
