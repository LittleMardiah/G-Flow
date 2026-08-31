-- ============================================================================
-- MIGRATION 009: Add missing updated_at on order tables
-- ============================================================================
-- Migration 005 melupakan kolom updated_at pada food_orders, send_orders, dan
-- send_order_stops, padahal seluruh query status/settlement/accept/stop
-- (food & send repository, worker auto-cancel) menulis updated_at = NOW().
-- Akibatnya PATCH food-orders, accept/deliver/settle send order, dan worker
-- auto-cancel gagal dengan SQLSTATE 42703 (kolom tidak ada). Migration ini
-- menyinkronkan skema agar konsisten dengan DATABASE_SCHEMA v10.4-FINAL
-- (kolom timestamp baru selain created_at memakai DEFAULT NOW(), sama seperti
-- tabel order sebelumnya).
--
-- PATCH: psql -h localhost -p 15432 -U postgres -d g_flow_dev -f 009_fix_order_updated_at.up.sql
-- ============================================================================

ALTER TABLE food_orders
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();

ALTER TABLE send_orders
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();

ALTER TABLE send_order_stops
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();