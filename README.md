# G-Flow Super-App — Backend

Backend untuk G-Flow Super-App. Saat ini fokus pada **Phase 1: Wallet & Authentication** (wallet engine PayPulse dengan top-up, transfer, dan idempotensi + JWT/RBAC) dan **Phase 2: G-Ride** (booking, dispatch, cancel, auto-cancel, settlement 80/20).

## Tech Stack
- **Go** 1.26.1
- **PostgreSQL** 14 (pgx/v5, pgxpool)
- **Redis** (go-redis/v9) — cache idempotensi + rate limit (opsional, graceful degradation)
- **Gin** HTTP framework

## Prerequisites
- Go 1.22+
- Docker (untuk database & Redis local)
- PostgreSQL client (`psql`) untuk menjalankan migration

## Setup Database

1. Buat file `.env` dari contoh (opsional, salin dari `.env.example`):
   ```bash
   cp .env.example .env
   ```

2. Jalankan PostgreSQL & Redis via Docker Compose:
   ```bash
   docker-compose up -d
   ```

3. Jalankan migration:
   ```bash
   export DATABASE_URL=postgresql://postgres:password@localhost:5432/g_flow_dev
   ./scripts/migrate.sh        # Linux/Mac
   # atau
   # set DATABASE_URL=postgresql://postgres:password@localhost:5432/g_flow_dev
   # scripts\migrate.bat       # Windows
   ```

## Run Server
```bash
go run cmd/api/main.go
```

Server berjalan di `http://localhost:8080`. Endpoint tersedia:

| Method | Path | Deskripsi |
|--------|------|-----------|
| GET    | `/health` | Health check |
| GET    | `/ready` | Readiness check (DB + Redis) |
| POST   | `/api/v1/auth/register` | Registrasi user (public) |
| POST   | `/api/v1/auth/login` | Login & token JWT (public) |
| POST   | `/api/v1/auth/logout` | Logout (revoke token) |
| POST   | `/api/v1/wallets/:wallet_id/topup` | Top-up wallet |
| POST   | `/api/v1/wallets/:wallet_id/transfer` | Transfer antar wallet |
| GET    | `/api/v1/wallets/:wallet_id/balance` | Cek saldo wallet |
| POST   | `/webhooks/topup` | Webhook top-up (internal) |
| POST   | `/api/v1/rides/book` | Booking ride (header `X-Idempotency-Key` wajib) |
| POST   | `/api/v1/rides/:order_id/accept` | Driver menerima order |
| PATCH  | `/api/v1/rides/:order_id/status` | Transisi status (`DRIVER_ARRIVED`/`TRIP_STARTED`/`COMPLETED`/`CANCELLED`) |

## Test dengan curl

Pastikan `wallet_id` dan `user_id` sudah ada di DB (insert manual / integration test).

```bash
# Contoh TopUp
curl -X POST http://localhost:8080/api/v1/wallets/<wallet_id>/topup \
  -H "Content-Type: application/json" \
  -d '{"amount":500000,"idempotency_key":"test-1"}'

# Contoh Transfer
curl -X POST http://localhost:8080/api/v1/wallets/<from_wallet_id>/transfer \
  -H "Content-Type: application/json" \
  -d '{"to_wallet_id":"<to_wallet_id>","amount":100000,"idempotency_key":"test-2","description":"p2p"}'

# Cek saldo
curl http://localhost:8080/api/v1/wallets/<wallet_id>/balance
```

## Run Tests

```bash
# Unit test biasa (tanpa DB/Redis eksternal)
go test ./... -cover

# Integration test — prasyarat: PostgreSQL docker-compose (port 15432) sudah
# di-migrate dan Redis:6380 up. Set DATABASE_URL sebelum menjalankan:
export DATABASE_URL=postgresql://postgres:password@localhost:15432/g_flow_dev
#     Windows PowerShell: $env:DATABASE_URL='postgresql://postgres:password@localhost:15432/g_flow_dev'

# Integration test per modul:
go test -tags integration ./internal/wallet/ -run Integration -v
go test -tags integration ./internal/ride/ -run Integration -v
```

> Catatan: integration test ride memakai build tag `integration` dan me-reset
> saldo wallet sistem ke 0 tiap test agar idempotent lintas run.

## Struktur
```
cmd/api/               # entrypoint + router Gin
internal/config/       # konfigurasi dari environment
internal/db/           # pool PostgreSQL
internal/wallet/       # repository, ledger, service, handler (+ test)
internal/middleware/   # auth middleware
migrations/            # SQL migration
scripts/               # script migration
```

## Roadmap
1. ~~Wallet & Authentication (Phase 1)~~ — dalam progres
2. JWT/RBAC & OTP
3. Integrasi payment gateway
4. Aplikasi Flutter
