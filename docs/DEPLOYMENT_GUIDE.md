# Deployment Guide — G-Flow (Laptop + ngrok, Gratis)

Panduan praktis untuk men-deploy backend G-Flow dari laptop ke internet **secara
gratis** memakai **ngrok** (free tier), sehingga aplikasi Flutter (customer/driver/merchant)
bisa terhubung ke backend real via URL publik HTTPS.

> Catatan: dokumen ini berfokus pada metode **laptop + ngrok** untuk demo/portfolio.
> Untuk deployment production skala penuh (Render 2 instance + Supabase PgBouncer)
> lihat [ROADMAP 04 HARDENING DEPLOY.txt](./ROADMAP%2004%20HARDENING%20DEPLOY.txt).

---

## 1. Prasyarat

- Go 1.26+
- PostgreSQL (dipakai dari Supabase gratis, atau Docker local)
- Redis (opsional, untuk L1 idempotency / rate limit)
- `psql` client
- Akun & binary **ngrok** (https://ngrok.com — free tier cukup)

---

## 2. Siapkan Database (Supabase)

1. Buat project di [Supabase](https://supabase.com) (free tier PostgreSQL 15+).
2. Salin **connection string** dari *Project Settings → Database*.
   - Gunakan port **5432** (bukan 6543/PgBouncer) untuk psql/CLI langsung.
3. Jalankan semua migration dari folder `migrations/`:

   ```bash
   export DATABASE_URL='postgresql://postgres.<ref>:<password>@aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres'

   for f in migrations/*.up.sql; do
     echo "== $f =="
     psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$f"
   done
   ```

   > Windows PowerShell:
   > ```powershell
   > $env:DATABASE_URL='postgresql://...'
   > Get-ChildItem migrations/*.up.sql | ForEach-Object { psql $env:DATABASE_URL -v ON_ERROR_STOP=1 -f $_.FullName }
   > ```

4. Verifikasi index dari migration 011:
   ```bash
   psql "$DATABASE_URL" -c "SELECT indexname FROM pg_indexes WHERE indexname LIKE 'idx_%_status_created';"
   ```

---

## 3. Konfigurasi Env

Salin `.env.example` → `.env` dan isi:

```env
DATABASE_URL=postgresql://postgres.<ref>:<password>@aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres
REDIS_URL=redis://localhost:6380
PORT=8080
ENV=production
JWT_SECRET=<ganti-dengan-secret-kuat>
```

- `REDIS_URL` boleh dikosongkan jika tidak memakai Redis (graceful degradation).
- `ENV=production` → log level `info`. `ENV=development` → `debug`.

---

## 4. Build & Jalankan Backend Lokal

```bash
# dari root project
go mod tidy
go build -o api.exe ./cmd/api
./api.exe
```

Server berjalan di `http://localhost:8080`.

Verifikasi health:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready        # cek DB + Redis
```

---

## 5. Expose ke Internet dengan ngrok (Gratis)

1. Install ngrok dan login:
   ```bash
   ngrok config add-authtoken <YOUR_AUTHTOKEN>
   ```

2. Tunneling ke port 8080:
   ```bash
   ngrok http 8080
   ```

3. Dapatkan URL publik (mis. `https://abcd-123-45.ngrok-free.app`).
   - Tulis URL ini, akan dipakai sebagai **BASE URL** di aplikasi Flutter.

> **Catatan demo/portfolio:** ngrok free domain berubah setiap restart. Untuk demo
> live yang stabil, upgrade ke ngrok domain statis (ada di free tier juga) atau
> aktifkan *Static Domain* agar BASE URL tidak berubah.

---

## 6. Konfigurasi Aplikasi Flutter

Pada masing-masing app (`customer_app`, `driver_app`, `merchant_app`), set base URL
ke URL ngrok:

- Buka `lib/services/api_client.dart` (dan service lain yang memuat base URL).
- Ganti `http://localhost:8080` → `https://<your-domain>.ngrok-free.app`.

> Flutter web/desktop di Chrome memblokir koneksi ke origin berbeda; nonaktifkan
> CORS sementara di `main.go` hanya untuk testing local, atau gunakan emulator
> mobile.

---

## 7. Migration & Utilitas Script

Tersedia skrip bantu:

- `scripts/migrate.sh` (Linux/Mac) / `scripts/migrate.bat` (Windows) — jalankan semua migration.
- `scripts/test_task3.2.sh` — test utilitas.

---

## 8. Menjalankan Integration & Unit Test

```bash
# Unit test (tanpa DB eksternal)
go test ./... -cover

# Integration test (butuh DB + Redis local / Supabase)
export DATABASE_URL='postgresql://postgres:password@localhost:15432/g_flow_dev'
go test -tags integration ./internal/... -v
```

Prasyarat DB + Redis local:

```bash
docker-compose up -d   # postgres:15432, redis:6380
```

---

## 9. Troubleshooting

| Gejala | Solusi |
|--------|--------|
| `dial tcp ... connection refused` | Pastikan `api.exe` berjalan & port 8080 bebas |
| `ready: { "db": "down" }` | Cek `DATABASE_URL`; tes `psql "$DATABASE_URL" -c "select 1"` |
| ngrok URL 404 di root | Akses path API, mis. `https://<domain>.ngrok-free.app/health` |
| Flutter CORS error | Gunakan emulator mobile, atau tambahkan header CORS di backend (dev only) |
| Index tidak terlihat | Cek nama tabel `topup_transactions` dll. sudah ada sebelum menjalankan migration 011 |

---

## 10. Catatan Keamanan untuk Demo

- Jangan pernah mengekspos `JWT_SECRET` / password ke publik.
- ngrok free menampilkan warning page saat pertamakali diakses (bisa di-klik "Visit Site").
- Untuk portofolio: nonaktifkan demo/mock flag di Flutter (lihat TD-019) dan tampilkan
  peringatan bahwa ini adalah sandbox.

---

## 11. Admin Default Credentials

Akun admin default dibuat via migration `014_seed_admin.up.sql` (STEP 1C — admin auth, TD-024):

- Email    : `admin@g-flow.local`
- Password : `AdminP@ssw0rd!2026`

> **Warning:** WAJIB ganti password setelah login pertama di production (TD-030).

---

## Referensi

- [Backend API](./API_REFERENCE.md)
- [OpenAPI Spec](./openapi.yaml)
- [Blueprint Roadmap](./BLUEPRINT%20ROADMAP.txt)
- [ROADMAP 04 (Hardening & Deploy)](./ROADMAP%2004%20HARDENING%20DEPLOY.txt)
