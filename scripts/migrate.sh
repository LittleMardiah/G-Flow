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

echo "Migrations completed."
