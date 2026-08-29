-- ============================================================================
-- MIGRATION 004: RIDE HAILING (Phase 2) — ride_orders, ride_order_events,
--                 driver_locations, enums
-- ============================================================================
-- Version : 004
-- Phase   : 2 (Ride Hailing / G-Ride)
-- Doc ref : DATABASE_SCHEMA v10.4-FINAL (LOCKED), ROADMAP_02 v2.3-FINAL,
--           API_CONTRACT v1.0, LOGIC_FLOW v6.1-FINAL
--
-- ============================================================================
-- DESIGN DECISIONS (disepakati bersama sebelum menulis migration):
--   1. STATUS MACHINE: TIDAK ada status EXPIRED di enum. Time-out order /
--      auto-cancel worker diwakili oleh CANCELLED + cancellation_reason.
--      order_status_enum SESUAI DATABASE_SCHEMA v10.4-FINAL (LOCKED).
--   2. KOMISI PLATFORM: 20% dari estimated_fare/actual_fare (ROADMAP_02 &
--      API_CONTRACT). LOGIC_FLOW v6.1 yang menyebut 10% adalah versi lama
--      (akan di-update di TECHNICAL_DEBT). Nilai 20% TIDAK di-hardcode di
--      database; dihitung di layer service dan disimpan di
--      platform_commission.
--   3. LOCK ORDERING STANDAR (SEMUA alur — termasuk Driver Emergency Cancel
--      dan Auto-Cancel Worker):
--          users (jika ada) → ride_orders → wallets  (masing-masing ORDER BY id)
--      CATATAN: ROADMAP_02 v2.3 masih menulis "ride_orders → wallets" untuk
--      auto-cancel worker — ITU VERSI LAMA. Standar baru per keputusan review:
--      lock users DULU, baru ride_orders, baru wallets. Locking adalah
--      tanggung jawab layer Go, bukan trigger/fungsi DB.
--   4. ENUM: order_status_enum & payment_method_enum BELUM ada di migration
--      001 (001 hanya membuat user_type_enum, wallet_type_enum,
--      entry_type_enum). Dibuat di sini.
--   5. driver_locations: TIDAK ada di migration 001 (diperiksa — hanya
--      disebut di komentar 001 sebagai persiapan ekstensi cube/
--      earthdistance). DIBUAT di sini.
--   6. SELISIH vs DATABASE_SCHEMA v10.4 (LOCKED) — POIN REVIEW:
--      a. pickup_address / dropoff_address: DATABASE_SCHEMA bernilai
--         NOT NULL. Dipakai NOT NULL di sini (konsisten dokumen LOCKED).
--         ★ Perlu konfirmasi review (spec user sempat menulis nullable).
--      b. Timestamps memakai TIMESTAMPTZ (keputusan migration ini; doc
--         memakai TIMESTAMP). TIMESTAMPTZ lebih aman untuk operasi
--         timezone-aware (auto-cancel worker).
--      c. cancellation_reason memakai TEXT (per ROADMAP_02), bukan
--         VARCHAR(255) (per DATABASE_SCHEMA).
--      d. expires_at DITAMBAHKAN (ROADMAP_02: tambah expires_at +
--         cancellation_reason) lengkap dengan idx_ride_expires partial
--         untuk auto-cancel worker.
--   7. VOUCHER_ID: TANPA FK (keputusan review Phase 2). Tabel vouchers belum
--      ada. voucher_id tetap UUID nullable. FK (voucher_id → vouchers(id)
--      ON DELETE SET NULL) akan ditambahkan di Phase 3 bersama migration
--      vouchers.
--   8. RLS: TIDAK diaktifkan di fase ini — konsisten dengan migration 002
--      (service Phase 1/2 belum mengatur auth.uid()). Policy contoh
--      dicomment; akan diaktifkan ulang di Phase 4 (hardening).
--   9. driver_locations: index GIST sesuai spec + dua index tambahan &
--      trigger dari DATABASE_SCHEMA v10.4 (LOCKED) untuk konsistensi.
--      ★ Trigger update_driver_online_timestamp perlu direview (menulis
--      ke users pada setiap update lokasi = write amplification).
-- ============================================================================

-- ============================================================================
-- ENUMS (Phase 2)
-- ============================================================================
CREATE TYPE order_status_enum AS ENUM (
  'CREATED', 'SEARCHING_DRIVER', 'DRIVER_ASSIGNED', 'DRIVER_ARRIVED',
  'TRIP_STARTED', 'COMPLETED', 'CANCELLED', 'SETTLED'
);

CREATE TYPE payment_method_enum AS ENUM ('WALLET', 'CASH', 'QRIS', 'CREDIT_CARD');

-- ============================================================================
-- TABLE: ride_orders (G-Ride) — core order
-- ============================================================================
-- Status flow : CREATED → SEARCHING_DRIVER → DRIVER_ASSIGNED →
--               DRIVER_ARRIVED → TRIP_STARTED → COMPLETED → SETTLED
--               (atau → CANCELLED dari status apa pun yang belum final)
-- ============================================================================
CREATE TABLE IF NOT EXISTS ride_orders (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  customer_id         UUID NOT NULL,
  driver_id           UUID,

  customer_wallet_id  UUID,
  driver_wallet_id    UUID,

  pickup_lat          NUMERIC(10,8) NOT NULL,
  pickup_lng          NUMERIC(11,8) NOT NULL,
  pickup_address      TEXT NOT NULL,

  dropoff_lat         NUMERIC(10,8) NOT NULL,
  dropoff_lng         NUMERIC(11,8) NOT NULL,
  dropoff_address     TEXT NOT NULL,

  distance_km         NUMERIC(8,3),
  base_fare           NUMERIC(12,2) NOT NULL DEFAULT 10000.00,
  per_km_rate         NUMERIC(8,2) NOT NULL DEFAULT 4000.00,
  estimated_fare      NUMERIC(12,2) NOT NULL,
  actual_fare         NUMERIC(12,2),

  surge_multiplier    NUMERIC(3,2) DEFAULT 1.00,
  toll_fee            NUMERIC(12,2) DEFAULT 0,
  cancellation_fee    NUMERIC(12,2) DEFAULT 0,

  discount_amount     NUMERIC(12,2) DEFAULT 0,
  voucher_id          UUID,
  -- voucher_id TANPA FK (keputusan #7) — FK ditambahkan di Phase 3.

  payment_method      payment_method_enum DEFAULT 'WALLET',

  platform_commission NUMERIC(12,2),
  driver_earning      NUMERIC(12,2),

  status              order_status_enum DEFAULT 'CREATED',
  cancellation_reason TEXT,

  created_at          TIMESTAMPTZ DEFAULT NOW(),
  expires_at          TIMESTAMPTZ,
  assigned_at         TIMESTAMPTZ,
  pickup_at           TIMESTAMPTZ,
  completed_at        TIMESTAMPTZ,
  settled_at          TIMESTAMPTZ,

  is_settled          BOOLEAN DEFAULT FALSE,
  settlement_notes    TEXT,

  CONSTRAINT fare_positive             CHECK (estimated_fare > 0),
  CONSTRAINT base_fare_positive        CHECK (base_fare > 0),
  CONSTRAINT per_km_rate_positive      CHECK (per_km_rate > 0),
  CONSTRAINT discount_non_negative     CHECK (discount_amount >= 0),
  CONSTRAINT surge_positive            CHECK (surge_multiplier >= 1.0),
  CONSTRAINT toll_non_negative         CHECK (toll_fee >= 0),
  CONSTRAINT cancellation_fee_non_negative CHECK (cancellation_fee >= 0),

  FOREIGN KEY (customer_id)        REFERENCES users(id),
  FOREIGN KEY (driver_id)          REFERENCES users(id),
  FOREIGN KEY (customer_wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (driver_wallet_id)   REFERENCES wallets(id)
  -- voucher_id: tidak ada FK (lihat keputusan #7). FK ditambahkan Phase 3.
);

CREATE INDEX idx_ride_customer ON ride_orders(customer_id);
CREATE INDEX idx_ride_driver ON ride_orders(driver_id);
CREATE INDEX idx_ride_status ON ride_orders(status);
CREATE INDEX idx_ride_status_created ON ride_orders(status, created_at DESC)
  WHERE status = 'SEARCHING_DRIVER';
CREATE INDEX idx_ride_created ON ride_orders(created_at DESC);
CREATE INDEX idx_ride_settled ON ride_orders(is_settled);
CREATE INDEX idx_ride_payment ON ride_orders(payment_method);
-- OPSIONAL-LOCKED: dipakai auto-cancel worker untuk menemukan order
-- SEARCHING_DRIVER yang expires_at-nya sudah lewat.
CREATE INDEX idx_ride_expires ON ride_orders(expires_at)
  WHERE status = 'SEARCHING_DRIVER';

-- ============================================================================
-- TABLE: ride_order_events (audit trail status transisi)
-- ============================================================================
CREATE TABLE IF NOT EXISTS ride_order_events (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id     UUID NOT NULL,

  from_status  order_status_enum,
  to_status    order_status_enum NOT NULL,
  reason       TEXT,

  triggered_by UUID,
  created_at   TIMESTAMPTZ DEFAULT NOW(),
  metadata     JSONB,

  FOREIGN KEY (order_id)     REFERENCES ride_orders(id) ON DELETE CASCADE,
  FOREIGN KEY (triggered_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_ride_events_order ON ride_order_events(order_id);
CREATE INDEX idx_ride_events_created ON ride_order_events(created_at DESC);

-- ============================================================================
-- TABLE: driver_locations (untuk driver matching spatial)
-- ============================================================================
-- TIDAK ada di migration 001 (hanya komentar). DIBUAT di sini.
-- Colom mengikuti spec + DATABASE_SCHEMA v10.4.
-- ============================================================================
CREATE TABLE IF NOT EXISTS driver_locations (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  driver_id   UUID NOT NULL UNIQUE,

  current_lat NUMERIC(10,8) NOT NULL,
  current_lng NUMERIC(11,8) NOT NULL,
  updated_at  TIMESTAMPTZ DEFAULT NOW(),

  FOREIGN KEY (driver_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Index spatial utama (ll_to_earth membutuhkan extension cube + earthdistance,
-- sudah dibuat di migration 001).
CREATE INDEX idx_driver_locations_gist ON driver_locations
  USING GIST (ll_to_earth(current_lat, current_lng));

-- Index tambahan + trigger dari DATABASE_SCHEMA v10.4 (LOCKED) — POIN REVIEW #9.
CREATE INDEX idx_driver_locations_driver ON driver_locations(driver_id);
CREATE INDEX idx_driver_locations_updated ON driver_locations(updated_at DESC);

CREATE OR REPLACE FUNCTION update_driver_online_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  UPDATE users
  SET last_online_at = NOW(), updated_at = NOW()
  WHERE id = NEW.driver_id AND user_type = 'driver';
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER driver_location_online_update
AFTER INSERT OR UPDATE ON driver_locations
FOR EACH ROW EXECUTE FUNCTION update_driver_online_timestamp();

-- ============================================================================
-- ROW-LEVEL SECURITY (menyusul di Phase 4 / hardening — lihat keputusan #8)
-- ============================================================================
-- ALTER TABLE ride_orders ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Users can view own ride orders" ON ride_orders
--   FOR SELECT USING (customer_id = auth.uid() OR driver_id = auth.uid());
-- CREATE POLICY "Users can insert ride orders" ON ride_orders
--   FOR INSERT WITH CHECK (customer_id = auth.uid());
-- CREATE POLICY "Users can update own ride orders" ON ride_orders
--   FOR UPDATE USING (customer_id = auth.uid() OR driver_id = auth.uid());
--
-- ALTER TABLE ride_order_events ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Users can view own ride events" ON ride_order_events
--   FOR SELECT USING (
--     EXISTS (SELECT 1 FROM ride_orders
--             WHERE id = order_id AND (customer_id = auth.uid() OR driver_id = auth.uid()))
--   );
--
-- ALTER TABLE driver_locations ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Drivers can update own location" ON driver_locations
--   FOR INSERT WITH CHECK (driver_id = auth.uid());
-- CREATE POLICY "Drivers can update own location" ON driver_locations
--   FOR UPDATE USING (driver_id = auth.uid());
-- CREATE POLICY "Anyone can view driver locations" ON driver_locations
--   FOR SELECT USING (TRUE);

-- ============================================================================
-- POST-MIGRATION SANITY CHECKS (informational — bisa dijalankan manual)
-- ============================================================================
-- SELECT enum_range(NULL::order_status_enum);
-- SELECT enum_range(NULL::payment_method_enum);
-- \d ride_orders
-- \d ride_order_events
-- \d driver_locations
-- SELECT count(*) FROM pg_indexes WHERE tablename = 'ride_orders';
-- ============================================================================

-- END OF MIGRATION 004.