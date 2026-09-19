-- MIGRATION 006: FOOD ORDER SETTLEMENT GUARD (Task 3.4, ROADMAP 03 §3.4)
-- Version : 006
-- Phase   : 3 (Food Delivery & Logistics / G-Food)
-- Doc ref : ROADMAP 03 FOOD LOGISTICS.txt (Section 3.4, F009)
--
-- ALASAN PERUBAHAN:
--   Settlement 4-way (WALLET) & CASH food order kini ditangani SEPENUHNYA di
--   Go service layer (internal/food/service.go — settleFoodOrderTx) memakai
--   wallet.LedgerService untuk double-entry ledger, lock wallet ORDER BY id
--   ASC FOR UPDATE, audit food_order_events, dan cek balance driver
--   (ceiling -Rp 50.000 → SUSPENDED). Keputusan ini konsisten dengan modul
--   ride Phase 2 (settlement CASH di Go service layer, bukan trigger).
--
--   Trigger `process_cash_settlement` dari migration 005 bermasalah:
--     1. Memanggil auth.uid() yang TIDAK tersedia di codebase ini (RLS
--        nonaktif, tanpa JWT di dalam koneksi DB) → INSERT ledger_entries
--        akan gagal saat trigger menyala.
--     2. Hanya meng-insert komisi platform (tanpa merchant_share & delivery
--        commission) → jika dibiarkan menyala, terjadi DOBEL settlement
--        dengan LedgerService (ledger imbalance).
--   Karena itu trigger lama di-DROP dan diganti dengan trigger GUARD yang
--   idempoten (BEFORE UPDATE) yang MENJAMIN invariant settlement:
--   status = 'SETTLED' ⇒ is_settled = TRUE dan settled_at terisi. Guard ini
--   TIDAK menulis ledger apapun — distribusi dana tetap milik service layer.

DROP TRIGGER IF EXISTS food_order_cash_settlement ON food_orders;
DROP TRIGGER IF EXISTS send_order_cash_settlement ON send_orders;
DROP FUNCTION IF EXISTS process_cash_settlement();

-- GUARD: settlement invariant (status SETTLED ⇔ is_settled & settled_at)
CREATE OR REPLACE FUNCTION assert_food_settlement_guard()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.status = 'SETTLED' THEN
    IF NEW.is_settled = FALSE THEN
      NEW.is_settled := TRUE;
    END IF;
    IF NEW.settled_at IS NULL THEN
      NEW.settled_at := NOW();
    END IF;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Terpasang ke food_orders (Task 3.4). send_orders (Task 3.5/3.6) menunggu
-- implementasi settlement-nya; trigger lama sudah di-drop agar tidak rusak.
CREATE TRIGGER food_order_settlement_guard
BEFORE UPDATE ON food_orders
FOR EACH ROW EXECUTE FUNCTION assert_food_settlement_guard();

-- POST-MIGRATION SANITY CHECK (informational — bisa dijalankan manual):
--   SELECT trigger_name, event_manipulation FROM information_schema.triggers
--   WHERE event_object_table = 'food_orders';
