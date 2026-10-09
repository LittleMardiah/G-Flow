@echo off
echo Running migrations...

if "%DATABASE_URL%"=="" (
  echo Error: DATABASE_URL is not set.
  exit /b 1
)

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\001_initial_schema.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\002_add_kyc_status_and_fix_idempotency_key.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\003_add_overdue_debt.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\004_ride_orders.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\005_food_send_schema.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\006_food_settlement_guard.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\007_auto_cancel_worker.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\008_fix_idempotency_cache.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\009_fix_order_updated_at.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\010_admin_lockouts.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\011_add_performance_indexes.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\012_add_overdue_debt_trigger.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\013_fix_send_stop_status.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\014_seed_admin.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\015_seed_admin_e2e.up.sql
if errorlevel 1 exit /b 1

psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\016_vouchers.up.sql
if errorlevel 1 exit /b 1

:: Migration 017 - hapus akun admin default (TD-030).
:: Setelah ini TIDAK ada admin; jalankan scripts\seed_admin.sh dengan
:: ADMIN_PASSWORD untuk membuat admin (lihat docs\DEPLOYMENT_GUIDE.md 11).
psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\017_remove_default_admin.up.sql
if errorlevel 1 exit /b 1

:: Migration 018 - ride_orders.updated_at (TD-158). Tanpa kolom ini worker
:: auto-cancel ride gagal dengan SQLSTATE 42703 dan refund escrow ikut
:: ter-rollback (lihat migrations\018_add_ride_orders_updated_at.up.sql).
psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\018_add_ride_orders_updated_at.up.sql
if errorlevel 1 exit /b 1

:: Migration 019 - wallet sistem SYSTEM_PLATFORM_SUBSIDY (TD-132). Tanpa wallet
:: ini, subsidi shortfall delta fare (TD-069) tercampur ke saldo/komisi
:: SYSTEM_PLATFORM. JANGAN tambahkan --single-transaction: nilai enum baru
:: tidak boleh dipakai di transaksi yang sama dengan ALTER TYPE ADD VALUE.
psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\019_add_platform_subsidy_wallet.up.sql
:: Migration 020 - withdrawal_requests (TD-183). Tabel request penarikan +
:: status enum PENDING/PROCESSING/COMPLETED/FAILED/REJECTED (DB source of
:: truth); API memetakan PENDING ke PENDING_APPROVAL. Idempotency di app
:: layer (body idempotency_key), bukan schema.
psql %DATABASE_URL% -v ON_ERROR_STOP=1 -f migrations\020_withdrawal_requests.up.sql
if errorlevel 1 exit /b 1

echo Migrations completed.
