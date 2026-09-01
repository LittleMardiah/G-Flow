-- ============================================================================
-- MIGRATION 013: SEND STOP STATUS ENUM 4-NILAI (TD-015)
-- ============================================================================
-- Menyelaraskan status send_order_stops dengan ROADMAP 03 (3.5.1 / 3.6.3):
--   SEBELUM : PENDING, ARRIVED, COMPLETED, SKIPPED
--   SESUDAH : PENDING, PICKED_UP, DELIVERED, CANCELLED
--
-- Langkah (aman untuk data existing):
--   1. Drop constraint lama.
--   2. Migrasi data lama ke nilai baru (mapping kompatibilitas).
--   3. Tambah kembali constraint dengan nilai 4-nilai yang baru.
--
-- Catatan: SKIPPED (RTS/partial) dan ARRIVED/COMPLETED di-remap sebagai
-- berikut karena partial stop cancellation tidak didukung di MVP:
--   ARRIVED   -> PICKED_UP
--   COMPLETED -> DELIVERED
--   SKIPPED   -> CANCELLED
-- ============================================================================

ALTER TABLE send_order_stops DROP CONSTRAINT IF EXISTS stops_status_valid;

UPDATE send_order_stops SET status = 'PICKED_UP' WHERE status = 'ARRIVED';
UPDATE send_order_stops SET status = 'DELIVERED' WHERE status = 'COMPLETED';
UPDATE send_order_stops SET status = 'CANCELLED' WHERE status = 'SKIPPED';

ALTER TABLE send_order_stops
  ADD CONSTRAINT stops_status_valid
  CHECK (status IN ('PENDING', 'PICKED_UP', 'DELIVERED', 'CANCELLED'));

-- ============================================================================
-- END OF MIGRATION 013
-- ============================================================================
