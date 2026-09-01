-- ============================================================================
-- MIGRATION 011: PERFORMANCE INDEXES (TD-012)
-- ============================================================================
-- Menambahkan index komposit (status, created_at/requested_at) pada
-- topup_transactions, ride_orders, food_orders, send_orders untuk mempercepat
-- query filter status + timestamp (webhook, auto-cancel worker, listing).
--
-- Catatan: CREATE INDEX CONCURRENTLY TIDAK dapat dieksekusi di dalam blok
-- transaksi. Jalankan file ini via psql langsung (per statement autocommit).
-- ============================================================================

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_topup_status_created
  ON topup_transactions (status, requested_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ride_status_created
  ON ride_orders (status, created_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_food_status_created
  ON food_orders (status, created_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_send_status_created
  ON send_orders (status, created_at);

-- ============================================================================
-- END OF MIGRATION 011
-- ============================================================================
