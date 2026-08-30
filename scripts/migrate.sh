#!/bin/bash
echo "Running migrations..."

# Pastikan DATABASE_URL sudah diset di environment, atau baca dari .env
if [ -z "$DATABASE_URL" ]; then
  echo "Error: DATABASE_URL is not set. Please set it or load from .env"
  exit 1
fi

# Migration 001
psql $DATABASE_URL -f migrations/001_initial_schema.up.sql

# Migration 002
psql $DATABASE_URL -f migrations/002_add_kyc_status_and_fix_idempotency_key.up.sql

# Migration 003
psql $DATABASE_URL -f migrations/003_add_overdue_debt.up.sql

# Migration 004
psql $DATABASE_URL -f migrations/004_ride_orders.up.sql

# Migration 005
psql $DATABASE_URL -f migrations/005_food_send_schema.up.sql

# Migration 006
psql $DATABASE_URL -f migrations/006_food_settlement_guard.up.sql

# Migration 007
psql $DATABASE_URL -f migrations/007_auto_cancel_worker.up.sql

echo "Migrations completed."
