# G-Flow — Bayar • Pesan • Kirim

Super-app ecosystem **G-Flow** — satu platform untuk **ride-hailing (G-Ride)**,
**food delivery (G-Food)**, **instant send (G-Send)**, dan **digital wallet (PayPulse)**.

Dibangun sebagai **modular monolith backend (Go)** di atas **PostgreSQL (Supabase)** & **Redis**,
dengan 3 aplikasi mobile (**Flutter**), admin panel web (**Next.js**), dan landing page (**Next.js**).

> **Live Demo:** [https://g-flow-tau.vercel.app](https://g-flow-tau.vercel.app)

> **Status:** Portfolio Project — V1.0-MVP (Production Ready, backend belum deployed)

> Dokumentasi lengkap tersedia di [`docs/`](./docs/), termasuk
> [API Reference](./docs/API_REFERENCE.md) dan [OpenAPI spec](./docs/openapi.yaml).

---

## Fitur Utama

| Modul | Deskripsi |
|-------|-----------|
| **PayPulse** | Digital wallet & auth — register/login/logout, JWT + RBAC, top-up, transfer, ledger akurat, token blacklist |
| **G-Ride** | Ride-hailing — booking, dispatch driver, accept/cancel, settlement pembayaran |
| **G-Food** | Food delivery — merchant, menu, order, 4-way settlement (merchant, driver, platform, user) |
| **G-Send** | Instant send (logistik multi-stop) — order dengan multiple stop, 3-way settlement |
| **Admin Web Panel** | Dashboard, ledger, user management, transaction reversal (dengan 2FA) |

---

## Tech Stack

**Backend (Modular Monolith, Golang)**
- **Go** 1.26
- **PostgreSQL (Supabase)** — pgx/v5, pgxpool
- **Redis** (go-redis/v9) — idempotency L1, token blacklist, driver location, distributed lock
- **Gin** HTTP framework
- JWT (HS256) + RBAC, structured logging (slog JSON), graceful shutdown

**Mobile (Flutter 3.x)**
- `apps/customer_app` — Ride, Food, Send, Wallet
- `apps/driver_app` — Accept orders, multi-stop delivery, earnings
- `apps/merchant_app` — Catalog management, order processing, analytics

**Web (Next.js)**
- `apps/admin_web` — Admin panel (dashboard, ledger, users, reversal 2FA)
- `apps/landing_page` — Landing page & live demo (deployed di Vercel)

---

## Struktur Monorepo

```
cmd/api/                 # entrypoint + router Gin
internal/
  config/                # konfigurasi dari environment
  db/                    # pool PostgreSQL
  middleware/            # auth middleware, recovery, logger
  auth/                  # PayPulse: register/login/logout, JWT, RBAC, blacklist
  wallet/                # PayPulse: repository, ledger, service, handler
  ride/                  # G-Ride: booking, dispatch, cancel, settlement
  food/                  # G-Food: merchant, menu, order, 4-way settlement
  send/                  # G-Send: order multi-stop, 3-way settlement
  location/              # driver location tracking (Redis + flush worker)
  driver/                # driver available orders / kapasitas
  worker/                # auto-cancel + purge idempotency cache
  admin/                 # dashboard, ledger, user mgmt, reversal 2FA
migrations/              # SQL migration (001-011)
scripts/                 # migration & test helper scripts
apps/
  customer_app/          # Flutter customer
  driver_app/            # Flutter driver
  merchant_app/          # Flutter merchant
  admin_web/             # Next.js admin panel
  landing_page/          # Next.js landing page (live demo)
docs/                    # seluruh dokumentasi proyek
.github/workflows/       # GitHub Actions CI
```

---

## Cara Menjalankan (Development)

### Backend (Go)

Prasyarat: Go 1.22+, Docker, PostgreSQL client (`psql`).

1. Salin env contoh:
   ```bash
   cp .env.example .env     # Linux/Mac
   copy .env.example .env   # Windows
   ```

2. Jalankan PostgreSQL & Redis via Docker Compose:
   ```bash
   docker-compose up -d     # postgres:15432, redis:6380
   ```

3. Jalankan semua migration:
   ```bash
   # Linux/Mac
   export DATABASE_URL=postgresql://postgres:password@localhost:15432/g_flow_dev
   ./scripts/migrate.sh

   # Windows PowerShell
   $env:DATABASE_URL='postgresql://postgres:password@localhost:15432/g_flow_dev'
   scripts\migrate.bat
   ```

   Secara manual per file:
   ```bash
   for f in migrations/*.up.sql; do psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$f"; done
   ```

   > Untuk Supabase, gunakan connection string project dan jalankan migration yang sama
   > (lihat [DEPLOYMENT_GUIDE.md](./docs/DEPLOYMENT_GUIDE.md)).

4. Jalankan server:
   ```bash
   go mod tidy
   go run ./cmd/api
   ```

   Server berjalan di `http://localhost:8080`. Periksa kesehatan:
   ```bash
   curl http://localhost:8080/health   # liveness
   curl http://localhost:8080/ready    # readiness (DB + Redis)
   ```

### Landing Page (Next.js)

```bash
cd apps/landing_page
npm install
npm run dev
```

### Admin Web (Next.js)

```bash
cd apps/admin_web
npm install
npm run dev
```

---

## Environment Variables (`.env`)

| Variable | Wajib | Keterangan |
|----------|-------|------------|
| `DATABASE_URL` | Ya | Connection string PostgreSQL |
| `REDIS_URL` | Opsional | `redis://...`; kosongkan = tanpa Redis (graceful degradation) |
| `PORT` | Tidak | Default `8080` |
| `ENV` | Tidak | `development` (debug) / `production` (info) |
| `JWT_SECRET` | Ya | Secret untuk penandatanganan JWT |

> Jangan commit `.env` ke repository. Contoh nilai tersedia di `.env.example`.

---

## Run Tests

### Unit test (tanpa DB/Redis eksternal)
```bash
go test ./... -cover
```

### Integration test (butuh PostgreSQL + Redis)
Prasyarat: `docker-compose up -d` sudah jalan dan telah dimigrasi.

```bash
# Linux/Mac
export DATABASE_URL=postgresql://postgres:password@localhost:15432/g_flow_dev

# Windows PowerShell
$env:DATABASE_URL='postgresql://postgres:password@localhost:15432/g_flow_dev'

# Jalankan semua integration test
go test -tags integration ./internal/... -v

# Atau per modul
go test -tags integration ./internal/wallet/ -run Integration -v
go test -tags integration ./internal/ride/ -run Integration -v
```

> Integration test menggunakan build tag `integration` dan me-reset saldo wallet sistem
> agar idempotent lintas run. Test ini **di-skip di CI** karena butuh database live.

### Flutter apps
```bash
cd apps/customer_app && flutter test
cd apps/driver_app && flutter test
cd apps/merchant_app && flutter test
```

---

## Endpoint Utama

Lihat [API_REFERENCE.md](./docs/API_REFERENCE.md) untuk tabel lengkap. Ringkasan:

| Area | Contoh Endpoint |
|------|-----------------|
| Auth | `POST /api/v1/auth/register`, `/login`, `/logout` |
| Wallet | `POST /wallets/:id/topup`, `/transfer`, `GET /balance` |
| Ride | `POST /rides/book`, `POST /rides/:id/accept`, `PATCH /rides/:id/status` |
| Food | `GET /merchants`, `GET /merchants/:id/items`, `POST /food-orders` |
| Send | `POST /send-orders`, `GET /send-orders/:id` |
| Driver | `GET /drivers/available-orders`, `POST /drivers/location` |
| Admin | `GET /admin/ledger`, `POST /admin/transactions/:id/reverse` |
| Health | `GET /health`, `GET /ready` |

---

## CI / CD

GitHub Actions workflow (`.github/workflows/ci.yml`) berjalan pada push & pull request ke `main`:

- **go-backend** — `go build ./...`, `go vet ./...`, `go test ./... -short` (integration test di-skip).
- **landing-page** — `npm ci && npm run build`.
- **flutter-analyze** — `flutter pub get && flutter analyze`.

No secrets digunakan di CI — seluruh job berjalan tanpa konfigurasi tambahan.

---

## Roadmap

- [x] Phase 1: Wallet & Auth (PayPulse, JWT/RBAC)
- [x] Phase 2: G-Ride (ride-hailing)
- [x] Phase 3: G-Food & G-Send (food logistics)
- [x] Phase 4: Hardening, Admin Panel, Deployment, Documentation
  - [x] 4.1 Testing suite (coverage ≥85%)
  - [x] 4.2 Admin Web Panel (termasuk reversal 2FA)
  - [x] 4.3 Deployment infrastructure (Supabase)
  - [x] 4.4 Documentation (user guides, API docs, deployment)
  - [x] 4.5 Final polish & smoke test

Detail lengkap: [`docs/BLUEPRINT ROADMAP.txt`](./docs/BLUEPRINT%20ROADMAP.txt) dan
[`docs/ROADMAP 04 HARDENING DEPLOY.txt`](./docs/ROADMAP%2004%20HARDENING%20DEPLOY.txt).

---

## Status Project

| Aspek | Status |
|-------|--------|
| Nama | G-Flow |
| Versi | V1.0-MVP (pre-release) |
| Jenis | Portfolio project |
| Backend | Production Ready (belum deployed ke hosting publik) |
| Frontend | Landing page deployed di Vercel |
| Lisensi | MIT |
| Repo | [github.com/LittleMardiah/G-Flow](https://github.com/LittleMardiah/G-Flow) |

---

## Lisensi

Distributed under the **MIT License**.

---

## Kontak

- **GitHub:** [LittleMardiah](https://github.com/LittleMardiah)
- **LinkedIn:** (placeholder — tambahkan profil kamu di sini)