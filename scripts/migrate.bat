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

echo Migrations completed.
