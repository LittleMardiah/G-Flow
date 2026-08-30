-- ============================================================================
-- MIGRATION 007: AUTO-CANCEL WORKER COLUMNS & INDEXES (Task 3.7, ROADMAP 03 §3.7)
-- ============================================================================
-- Version : 007
-- Phase   : 3 (Food Delivery & Logistics / G-Food & G-Send)
-- Doc ref : ROADMAP 03 FOOD LOGISTICS.txt (Section 3.7 — Auto-Cancel Workers)
--
-- ALASAN PERUBAHAN:
--   Worker auto-cancel membatalkan order yang tidak direspons dalam waktu
--   tertentu:
--     * food_orders : status = 'CREATED'  DAN created_at < NOW() - 15 menit
--     * send_orders : status = 'SEARCHING_DRIVER' DAN created_at < NOW() - 10 menit
--   dan menandai status = 'CANCELLED' dengan cancellation_reason = 'EXPIRED'
--   (menyelaraskan dengan alur Phase 2 ride_orders yang memakai
--   CANCELLED + cancellation_reason alih-alih status EXPIRED).
--
--   Tabel food_orders & send_orders (MIGRATION 005) BELUM memiliki kolom
--   cancellation_reason (hanya ride_orders yang punya, Migration 004). Jadi
--   migration ini:
--     1. Menambah kolom cancellation_reason TEXT ke food_orders & send_orders.
--     2. Menambah partial index (status, created_at) agar query worker cepat
--        dan tidak full-scan tabel order.
-- ============================================================================

-- ============================================================================
-- 1. Kolom cancellation_reason (food_orders & send_orders)
-- ============================================================================
ALTER TABLE food_orders
  ADD COLUMN IF NOT EXISTS cancellation_reason TEXT;

ALTER TABLE send_orders
  ADD COLUMN IF NOT EXISTS cancellation_reason TEXT;

-- ============================================================================
-- 2. Partial index untuk Auto-Cancel Worker (query per 1 menit)
-- ============================================================================
-- Food order auto-cancel: SELECT id FROM food_orders
--   WHERE status = 'CREATED' AND created_at < NOW() - INTERVAL '15 minutes'
CREATE INDEX IF NOT EXISTS idx_food_orders_auto_cancel
  ON food_orders (status, created_at)
  WHERE status = 'CREATED';

-- Send order auto-cancel: SELECT id FROM send_orders
--   WHERE status = 'SEARCHING_DRIVER' AND created_at < NOW() - INTERVAL '10 minutes'
CREATE INDEX IF NOT EXISTS idx_send_orders_auto_cancel
  ON send_orders (status, created_at)
  WHERE status = 'SEARCHING_DRIVER';

-- ============================================================================
-- POST-MIGRATION SANITY CHECK (informational — bisa dijalankan manual):
--   SELECT column_name FROM information_schema.columns
--   WHERE table_name IN ('food_orders','send_orders')
--     AND column_name = 'cancellation_reason';
-- ============================================================================
