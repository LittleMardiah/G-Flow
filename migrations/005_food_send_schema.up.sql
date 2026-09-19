-- MIGRATION 005: FOOD & SEND (Phase 3) — food_merchants, merchant_menus,
--                 merchant_items, item_option_groups, item_options,
--                 merchant_item_option_groups, food_orders, food_order_items,
--                 food_order_events, send_orders, send_order_stops,
--                 send_order_events
-- Version : 005
-- Phase   : 3 (Food Delivery & Logistics / G-Food & G-Send)
-- Doc ref : DATABASE_SCHEMA v10.4-FINAL (LOCKED), ROADMAP_03 v2.3-FINAL
--
-- DESIGN DECISIONS (disepakati mengikuti keputusan Migration 004):
--   1. ENUM: food_order_status_enum, send_order_status_enum, package_type_enum
--      BELUM ada di migration 001-004. Dibuat di sini. payment_method_enum
--      sudah dibuat di migration 004 — TIDAK dibuat ulang.
--   2. VOUCHER_ID: TANPA FK (keputusan Phase 2 #7 — Migration 004). Tabel
--      vouchers & user_vouchers belum ada. FK (voucher_id → vouchers(id)
--      ON DELETE SET NULL) akan ditambahkan bersama migration vouchers.
--   3. FEED-ORDER TRIGGER `rollback_voucher_soft_delete`: DITUNDA karena
--      bergantung pada tabel user_vouchers yang belum ada (sama seperti
--      keputusan #2). Akan dibuat bersama migration vouchers.
--   4. Timestamp memakai TIMESTAMPTZ (keputusan Migration 004) — lebih aman
--      untuk operasi timezone-aware (auto-cancel worker).
--   5. RLS: TIDAK diaktifkan di fase ini — konsisten dengan migration 002/004
--      (service Phase 1-3 belum mengatur auth.uid()). Policy contoh
--      dicomment; akan diaktifkan ulang di Phase 4 (hardening).
--   6. SELISIH vs DATABASE_SCHEMA v10.4 (LOCKED) — POIN REVIEW:
--      a. send_order_stops memakai kolom stop_number (bukan stop_sequence)
--         sesuai DATABASE_SCHEMA v10.4 LOCKED.
--      b. food_orders menambahkan kolom cutlery_included & special_instructions
--         sesuai DATABASE_SCHEMA LOCKED. Kolom cancelled_flag tidak dipakai.
--      c. send_orders TIDAK menambahkan expires_at (bukan bagian spec LOCKED);
--         auto-cancel worker memakai created_at + INTERVAL '10 minutes'
--         (ROADMAP 3.7 di layer service).
--   7. TRIGGER `calc_food_driver_earning` (BEFORE INSERT) & `process_cash_settlement`
--      (AFTER UPDATE) dibuat di sini — self-contained, tidak bergantung pada
--      tabel yang belum ada (hanya wallets & ledger_entries yang sudah ada).
--      Trigger process_cash_settlement dipasang ke food_orders & send_orders.
--      (Untuk ride_orders, settlement CASH sudah ditangani di Go service
--      layer Phase 2 — tidak dibuka ulang di sini.)

-- ENUMS (Phase 3)
CREATE TYPE food_order_status_enum AS ENUM (
  'CREATED', 'CONFIRMED', 'PREPARING', 'READY_FOR_PICKUP',
  'PICKED_UP', 'IN_TRANSIT', 'DELIVERED', 'CANCELLED', 'SETTLED'
);

CREATE TYPE send_order_status_enum AS ENUM (
  'CREATED', 'SEARCHING_DRIVER', 'DRIVER_ASSIGNED', 'PICKED_UP',
  'IN_TRANSIT', 'DELIVERED', 'CANCELLED', 'SETTLED'
);

CREATE TYPE package_type_enum AS ENUM ('STANDARD', 'FRAGILE', 'LIQUID', 'ELECTRONICS');

-- TABLE: food_merchants (G-Food — Merchant / Restoran)
CREATE TABLE IF NOT EXISTS food_merchants (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id             UUID NOT NULL UNIQUE,

  merchant_name       VARCHAR(255) NOT NULL,
  merchant_description TEXT,
  category            VARCHAR(50) NOT NULL,

  latitude            NUMERIC(10,8) NOT NULL,
  longitude           NUMERIC(11,8) NOT NULL,
  address             TEXT NOT NULL,
  phone               VARCHAR(20),

  avg_rating          NUMERIC(3,2) DEFAULT 5.0,
  total_reviews       INTEGER DEFAULT 0,
  total_orders        INTEGER DEFAULT 0,

  opening_time        TIME,
  closing_time        TIME,
  is_open             BOOLEAN DEFAULT TRUE,

  status              VARCHAR(50) DEFAULT 'ACTIVE',
  verified_at         TIMESTAMPTZ,

  logo_url            TEXT,
  created_at          TIMESTAMPTZ DEFAULT NOW(),
  updated_at          TIMESTAMPTZ DEFAULT NOW(),

  CONSTRAINT rating_valid CHECK (avg_rating >= 0 AND avg_rating <= 5),
  CONSTRAINT merchant_status_valid CHECK (status IN ('ACTIVE', 'PENDING_VERIFICATION', 'SUSPENDED', 'CLOSED')),

  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_food_merchants_category ON food_merchants(category);
CREATE INDEX idx_food_merchants_location ON food_merchants USING GIST(
  ll_to_earth(latitude, longitude)
);
CREATE INDEX idx_food_merchants_status ON food_merchants(status);

-- RLS (ditunda ke Phase 4 / hardening — lihat keputusan #5).
-- ALTER TABLE food_merchants ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Anyone can view active merchants" ON food_merchants
--   FOR SELECT USING (status = 'ACTIVE' OR user_id = auth.uid());
-- CREATE POLICY "Merchants can update own data" ON food_merchants
--   FOR UPDATE USING (user_id = auth.uid());

-- TABLE: merchant_menus (G-Food — Kelompok menu merchant)
CREATE TABLE IF NOT EXISTS merchant_menus (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  merchant_id     UUID NOT NULL,

  name            VARCHAR(255) NOT NULL,
  description     TEXT,
  sequence_order  INTEGER DEFAULT 0,
  is_active       BOOLEAN DEFAULT TRUE,

  created_at      TIMESTAMPTZ DEFAULT NOW(),
  updated_at      TIMESTAMPTZ DEFAULT NOW(),

  FOREIGN KEY (merchant_id) REFERENCES food_merchants(id) ON DELETE CASCADE,
  UNIQUE (merchant_id, name)
);

CREATE INDEX idx_merchant_menus_merchant ON merchant_menus(merchant_id);

-- RLS (ditunda — keputusan #5).
-- ALTER TABLE merchant_menus ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Anyone can view active menus" ON merchant_menus
--   FOR SELECT USING (is_active = TRUE);
-- CREATE POLICY "Merchants can manage own menus" ON merchant_menus
--   FOR ALL USING (
--     merchant_id IN (SELECT id FROM food_merchants WHERE user_id = auth.uid())
--   );

-- TABLE: merchant_items (G-Food — Item / produk merchant)
CREATE TABLE IF NOT EXISTS merchant_items (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  menu_id         UUID NOT NULL,
  merchant_id     UUID NOT NULL,

  name            VARCHAR(255) NOT NULL,
  description     TEXT,
  price           NUMERIC(10,2) NOT NULL,
  image_url       TEXT,

  stock           INTEGER DEFAULT 999,
  is_available    BOOLEAN DEFAULT TRUE,

  created_at      TIMESTAMPTZ DEFAULT NOW(),
  updated_at      TIMESTAMPTZ DEFAULT NOW(),

  CONSTRAINT price_positive CHECK (price > 0),

  FOREIGN KEY (menu_id) REFERENCES merchant_menus(id) ON DELETE CASCADE,
  FOREIGN KEY (merchant_id) REFERENCES food_merchants(id) ON DELETE CASCADE
);

CREATE INDEX idx_merchant_items_menu ON merchant_items(menu_id);
CREATE INDEX idx_merchant_items_merchant ON merchant_items(merchant_id);
CREATE INDEX idx_merchant_items_available ON merchant_items(is_available)
  WHERE is_available = TRUE;

-- RLS (ditunda — keputusan #5).
-- ALTER TABLE merchant_items ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Anyone can view available items" ON merchant_items
--   FOR SELECT USING (is_available = TRUE);
-- CREATE POLICY "Merchants can manage own items" ON merchant_items
--   FOR ALL USING (
--     merchant_id IN (SELECT id FROM food_merchants WHERE user_id = auth.uid())
--   );

-- TABLE: item_option_groups (G-Food — Grup opsi item, mis. Ukuran / Topping)
CREATE TABLE IF NOT EXISTS item_option_groups (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  merchant_id     UUID NOT NULL,

  name            VARCHAR(255) NOT NULL,
  description     TEXT,

  selection_type  VARCHAR(20) DEFAULT 'SINGLE',
  is_required     BOOLEAN DEFAULT FALSE,
  max_choices     INTEGER DEFAULT 1,

  sequence_order  INTEGER DEFAULT 0,
  is_active       BOOLEAN DEFAULT TRUE,

  created_at      TIMESTAMPTZ DEFAULT NOW(),
  updated_at      TIMESTAMPTZ DEFAULT NOW(),

  CONSTRAINT selection_type_valid CHECK (selection_type IN ('SINGLE', 'MULTIPLE')),
  CONSTRAINT max_choices_positive CHECK (max_choices >= 1),

  FOREIGN KEY (merchant_id) REFERENCES food_merchants(id) ON DELETE CASCADE,
  UNIQUE (merchant_id, name)
);

CREATE INDEX idx_option_groups_merchant ON item_option_groups(merchant_id);

-- RLS (ditunda — keputusan #5).
-- ALTER TABLE item_option_groups ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Anyone can view active option groups" ON item_option_groups
--   FOR SELECT USING (is_active = TRUE);
-- CREATE POLICY "Merchants can manage own option groups" ON item_option_groups
--   FOR ALL USING (
--     merchant_id IN (SELECT id FROM food_merchants WHERE user_id = auth.uid())
--   );

-- TABLE: item_options (G-Food — Opsi spesifik, mis. Small/Large/Extra Cheese)
CREATE TABLE IF NOT EXISTS item_options (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  option_group_id     UUID NOT NULL,
  merchant_id         UUID NOT NULL,

  name                VARCHAR(255) NOT NULL,
  price_adjustment    NUMERIC(10,2) DEFAULT 0,
  description         TEXT,

  sequence_order      INTEGER DEFAULT 0,
  is_available        BOOLEAN DEFAULT TRUE,

  created_at          TIMESTAMPTZ DEFAULT NOW(),
  updated_at          TIMESTAMPTZ DEFAULT NOW(),

  CONSTRAINT price_adjustment_non_negative CHECK (price_adjustment >= 0),

  FOREIGN KEY (option_group_id) REFERENCES item_option_groups(id) ON DELETE CASCADE,
  FOREIGN KEY (merchant_id) REFERENCES food_merchants(id) ON DELETE CASCADE
);

CREATE INDEX idx_options_group ON item_options(option_group_id);
CREATE INDEX idx_options_merchant ON item_options(merchant_id);

-- RLS (ditunda — keputusan #5).
-- ALTER TABLE item_options ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Anyone can view available options" ON item_options
--   FOR SELECT USING (is_available = TRUE);
-- CREATE POLICY "Merchants can manage own options" ON item_options
--   FOR ALL USING (
--     merchant_id IN (SELECT id FROM food_merchants WHERE user_id = auth.uid())
--   );

-- TABLE: merchant_item_option_groups (G-Food — tabel junction item ↔ opsi)
CREATE TABLE IF NOT EXISTS merchant_item_option_groups (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  item_id         UUID NOT NULL,
  option_group_id UUID NOT NULL,

  sequence_order  INTEGER DEFAULT 0,
  is_active       BOOLEAN DEFAULT TRUE,

  created_at      TIMESTAMPTZ DEFAULT NOW(),
  updated_at      TIMESTAMPTZ DEFAULT NOW(),

  FOREIGN KEY (item_id) REFERENCES merchant_items(id) ON DELETE CASCADE,
  FOREIGN KEY (option_group_id) REFERENCES item_option_groups(id) ON DELETE CASCADE,
  UNIQUE (item_id, option_group_id)
);

CREATE INDEX idx_item_option_groups_item ON merchant_item_option_groups(item_id);
CREATE INDEX idx_item_option_groups_group ON merchant_item_option_groups(option_group_id);

-- RLS (ditunda — keputusan #5).
-- ALTER TABLE merchant_item_option_groups ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Anyone can view active item options" ON merchant_item_option_groups
--   FOR SELECT USING (is_active = TRUE);
-- CREATE POLICY "Merchants can manage own item options" ON merchant_item_option_groups
--   FOR ALL USING (
--     EXISTS (SELECT 1 FROM merchant_items WHERE id = item_id AND
--             merchant_id IN (SELECT id FROM food_merchants WHERE user_id = auth.uid()))
--   );

-- TABLE: food_orders (G-Food — order makanan)
-- Status flow : CREATED → CONFIRMED → PREPARING → READY_FOR_PICKUP →
--               PICKED_UP → IN_TRANSIT → DELIVERED → SETTLED
--               (atau → CANCELLED dari status apa pun yang belum final)
CREATE TABLE IF NOT EXISTS food_orders (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  customer_id         UUID NOT NULL,
  merchant_id         UUID NOT NULL,
  driver_id           UUID,

  customer_wallet_id  UUID,
  merchant_wallet_id  UUID,
  driver_wallet_id    UUID,

  delivery_address    TEXT NOT NULL,
  delivery_lat        NUMERIC(10,8),
  delivery_lng        NUMERIC(11,8),
  special_instructions TEXT,

  item_subtotal       NUMERIC(12,2) NOT NULL,
  delivery_fee        NUMERIC(10,2) NOT NULL,
  platform_commission NUMERIC(10,2),

  driver_earning      NUMERIC(12,2),

  discount_amount     NUMERIC(10,2) DEFAULT 0,
  voucher_id          UUID,
  -- voucher_id TANPA FK (keputusan #2) — FK ditambahkan bersama tabel vouchers.

  payment_method      payment_method_enum DEFAULT 'WALLET',

  cutlery_included    BOOLEAN DEFAULT FALSE,

  total_amount        NUMERIC(12,2) NOT NULL,

  status              food_order_status_enum DEFAULT 'CREATED',
  merchant_status     VARCHAR(50) DEFAULT 'WAITING',
  merchant_notes      TEXT,

  created_at          TIMESTAMPTZ DEFAULT NOW(),
  confirmed_at        TIMESTAMPTZ,
  pickup_at           TIMESTAMPTZ,
  delivered_at        TIMESTAMPTZ,
  settled_at          TIMESTAMPTZ,

  is_settled          BOOLEAN DEFAULT FALSE,
  is_refunded         BOOLEAN DEFAULT FALSE,

  CONSTRAINT amount_positive CHECK (total_amount > 0),
  CONSTRAINT discount_non_negative CHECK (discount_amount >= 0),
  CONSTRAINT driver_earning_non_negative CHECK (driver_earning >= 0),
  CONSTRAINT merchant_status_valid CHECK (merchant_status IN ('WAITING', 'CONFIRMED', 'PREPARING', 'READY')),

  FOREIGN KEY (customer_id) REFERENCES users(id),
  FOREIGN KEY (merchant_id) REFERENCES food_merchants(id),
  FOREIGN KEY (driver_id) REFERENCES users(id),
  FOREIGN KEY (customer_wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (merchant_wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (driver_wallet_id) REFERENCES wallets(id)
  -- voucher_id: tidak ada FK (lihat keputusan #2).
);

-- Trigger auto-calc driver_earning (delivery_fee * 0.9)
CREATE OR REPLACE FUNCTION calc_food_driver_earning()
RETURNS TRIGGER AS $$
BEGIN
  NEW.driver_earning := NEW.delivery_fee * 0.9;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER food_driver_earning_calc
BEFORE INSERT ON food_orders
FOR EACH ROW EXECUTE FUNCTION calc_food_driver_earning();

CREATE INDEX idx_food_orders_customer ON food_orders(customer_id);
CREATE INDEX idx_food_orders_merchant ON food_orders(merchant_id);
CREATE INDEX idx_food_orders_driver ON food_orders(driver_id);
CREATE INDEX idx_food_orders_status ON food_orders(status);
CREATE INDEX idx_food_orders_created ON food_orders(created_at DESC);
CREATE INDEX idx_food_orders_settled ON food_orders(is_settled);
CREATE INDEX idx_food_orders_payment ON food_orders(payment_method);

-- RLS (ditunda — keputusan #5).
-- ALTER TABLE food_orders ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Users can view own food orders" ON food_orders
--   FOR SELECT USING (customer_id = auth.uid() OR merchant_id = auth.uid() OR driver_id = auth.uid());
-- CREATE POLICY "Users can insert food orders" ON food_orders
--   FOR INSERT WITH CHECK (customer_id = auth.uid());
-- CREATE POLICY "Users can update own food orders" ON food_orders
--   FOR UPDATE USING (customer_id = auth.uid() OR merchant_id = auth.uid() OR driver_id = auth.uid());

-- TABLE: food_order_items (G-Food — item dari order makanan)
CREATE TABLE IF NOT EXISTS food_order_items (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id            UUID NOT NULL,
  item_id             UUID NOT NULL,

  item_name           VARCHAR(255) NOT NULL,
  item_price          NUMERIC(10,2) NOT NULL,
  quantity            INTEGER NOT NULL,
  subtotal            NUMERIC(12,2) NOT NULL,

  options             JSONB DEFAULT '{}'::jsonb,
  options_total       NUMERIC(10,2) DEFAULT 0,

  special_instructions TEXT,

  created_at          TIMESTAMPTZ DEFAULT NOW(),

  CONSTRAINT quantity_positive CHECK (quantity > 0),
  CONSTRAINT options_total_non_negative CHECK (options_total >= 0),

  FOREIGN KEY (order_id) REFERENCES food_orders(id) ON DELETE CASCADE,
  FOREIGN KEY (item_id) REFERENCES merchant_items(id) ON DELETE RESTRICT
);

CREATE INDEX idx_food_order_items_order ON food_order_items(order_id);
CREATE INDEX idx_food_order_items_options ON food_order_items USING GIN (options);

-- RLS (ditunda — keputusan #5).
-- ALTER TABLE food_order_items ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Customer can view own order items" ON food_order_items
--   FOR SELECT USING (
--     EXISTS (SELECT 1 FROM food_orders WHERE id = order_id AND customer_id = auth.uid())
--   );
-- CREATE POLICY "Merchant can view order items for own store" ON food_order_items
--   FOR SELECT USING (
--     EXISTS (SELECT 1 FROM food_orders fo
--             JOIN food_merchants fm ON fm.id = fo.merchant_id
--             WHERE fo.id = order_id AND fm.user_id = auth.uid())
--   );
-- CREATE POLICY "Driver can view order items for assigned orders" ON food_order_items
--   FOR SELECT USING (
--     EXISTS (SELECT 1 FROM food_orders WHERE id = order_id AND driver_id = auth.uid())
--   );
-- CREATE POLICY "Customer can insert own order items" ON food_order_items
--   FOR INSERT WITH CHECK (
--     EXISTS (SELECT 1 FROM food_orders WHERE id = order_id AND customer_id = auth.uid())
--   );

-- TABLE: food_order_events (G-Food — audit trail transisi status)
CREATE TABLE IF NOT EXISTS food_order_events (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id     UUID NOT NULL,

  from_status  food_order_status_enum,
  to_status    food_order_status_enum NOT NULL,
  reason       TEXT,

  triggered_by UUID,
  created_at   TIMESTAMPTZ DEFAULT NOW(),
  metadata     JSONB,

  FOREIGN KEY (order_id) REFERENCES food_orders(id) ON DELETE CASCADE,
  FOREIGN KEY (triggered_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_food_events_order ON food_order_events(order_id);
CREATE INDEX idx_food_events_created ON food_order_events(created_at DESC);

-- RLS (ditunda — keputusan #5).
-- ALTER TABLE food_order_events ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Users can view own food events" ON food_order_events
--   FOR SELECT USING (
--     EXISTS (SELECT 1 FROM food_orders WHERE id = order_id AND
--             (customer_id = auth.uid() OR merchant_id = auth.uid() OR driver_id = auth.uid()))
--   );
-- CREATE POLICY "System can insert food events" ON food_order_events
--   FOR INSERT WITH CHECK (TRUE);

-- TABLE: send_orders (G-Send — order pengiriman paket)
-- Status flow : CREATED → SEARCHING_DRIVER → DRIVER_ASSIGNED → PICKED_UP →
--               IN_TRANSIT → DELIVERED → SETTLED
--               (atau → CANCELLED dari status apa pun yang belum final)
CREATE TABLE IF NOT EXISTS send_orders (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  sender_id           UUID NOT NULL,
  driver_id           UUID,

  sender_wallet_id    UUID,
  driver_wallet_id    UUID,

  package_weight_kg   NUMERIC(6,2),
  package_dimensions_cm TEXT,
  package_description TEXT,

  pickup_lat          NUMERIC(10,8) NOT NULL,
  pickup_lng          NUMERIC(11,8) NOT NULL,
  pickup_address      TEXT NOT NULL,

  base_fare           NUMERIC(12,2) NOT NULL,
  distance_km         NUMERIC(8,3),
  weight_surcharge    NUMERIC(10,2) DEFAULT 0,
  total_fare          NUMERIC(12,2) NOT NULL,

  declared_value      NUMERIC(12,2) DEFAULT 0,
  package_type        package_type_enum DEFAULT 'STANDARD',
  insurance_fee       NUMERIC(10,2) DEFAULT 0,

  discount_amount     NUMERIC(10,2) DEFAULT 0,
  voucher_id          UUID,
  -- voucher_id TANPA FK (keputusan #2) — FK ditambahkan bersama tabel vouchers.

  payment_method      payment_method_enum DEFAULT 'WALLET',

  platform_commission NUMERIC(10,2),
  driver_earning      NUMERIC(12,2),

  status              send_order_status_enum DEFAULT 'CREATED',

  delivery_photo_url  TEXT,
  recipient_signature BYTEA,

  created_at          TIMESTAMPTZ DEFAULT NOW(),
  assigned_at         TIMESTAMPTZ,
  pickup_at           TIMESTAMPTZ,
  delivered_at        TIMESTAMPTZ,
  settled_at          TIMESTAMPTZ,

  is_settled          BOOLEAN DEFAULT FALSE,

  CONSTRAINT fare_positive CHECK (total_fare > 0),
  CONSTRAINT discount_non_negative CHECK (discount_amount >= 0),
  CONSTRAINT declared_value_non_negative CHECK (declared_value >= 0),
  CONSTRAINT insurance_fee_non_negative CHECK (insurance_fee >= 0),
  CONSTRAINT weight_positive CHECK (package_weight_kg IS NULL OR package_weight_kg > 0),

  FOREIGN KEY (sender_id) REFERENCES users(id),
  FOREIGN KEY (driver_id) REFERENCES users(id),
  FOREIGN KEY (sender_wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (driver_wallet_id) REFERENCES wallets(id)
  -- voucher_id: tidak ada FK (lihat keputusan #2).
);

CREATE INDEX idx_send_orders_sender ON send_orders(sender_id);
CREATE INDEX idx_send_orders_driver ON send_orders(driver_id);
CREATE INDEX idx_send_orders_status ON send_orders(status);
CREATE INDEX idx_send_orders_created ON send_orders(created_at DESC);
CREATE INDEX idx_send_orders_settled ON send_orders(is_settled);
CREATE INDEX idx_send_orders_driver_active ON send_orders(driver_id, status)
  WHERE status IN ('DRIVER_ASSIGNED', 'PICKED_UP');
CREATE INDEX idx_send_orders_payment ON send_orders(payment_method);

-- RLS (ditunda — keputusan #5).
-- ALTER TABLE send_orders ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Users can view own send orders" ON send_orders
--   FOR SELECT USING (sender_id = auth.uid() OR driver_id = auth.uid());
-- CREATE POLICY "Users can insert send orders" ON send_orders
--   FOR INSERT WITH CHECK (sender_id = auth.uid());
-- CREATE POLICY "Users can update own send orders" ON send_orders
--   FOR UPDATE USING (sender_id = auth.uid() OR driver_id = auth.uid());

-- TABLE: send_order_stops (G-Send — multi-stop dengan alokasi ongkos)
CREATE TABLE IF NOT EXISTS send_order_stops (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id            UUID NOT NULL,

  stop_number         INTEGER NOT NULL,

  recipient_name      VARCHAR(255),
  recipient_phone     VARCHAR(20),

  dropoff_lat         NUMERIC(10,8) NOT NULL,
  dropoff_lng         NUMERIC(11,8) NOT NULL,
  dropoff_address     TEXT NOT NULL,

  distance_km         NUMERIC(8,3),
  allocated_fare      NUMERIC(12,2),

  status              VARCHAR(50) DEFAULT 'PENDING',

  delivery_photo_url  TEXT,
  recipient_signature BYTEA,

  arrived_at          TIMESTAMPTZ,
  completed_at        TIMESTAMPTZ,

  notes               TEXT,

  CONSTRAINT stop_number_positive CHECK (stop_number > 0),
  CONSTRAINT stops_status_valid CHECK (status IN ('PENDING', 'PICKED_UP', 'DELIVERED', 'CANCELLED')),
  CONSTRAINT allocated_fare_non_negative CHECK (allocated_fare >= 0),

  FOREIGN KEY (order_id) REFERENCES send_orders(id) ON DELETE CASCADE,
  UNIQUE (order_id, stop_number)
);

CREATE INDEX idx_send_stops_order ON send_order_stops(order_id);
CREATE INDEX idx_send_stops_status ON send_order_stops(status);
CREATE INDEX idx_send_stops_created ON send_order_stops(completed_at DESC);

-- RLS (ditunda — keputusan #5).
-- ALTER TABLE send_order_stops ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Sender can view own stops" ON send_order_stops
--   FOR SELECT USING (
--     EXISTS (SELECT 1 FROM send_orders WHERE id = order_id AND sender_id = auth.uid())
--   );
-- CREATE POLICY "Driver can view assigned stops" ON send_order_stops
--   FOR SELECT USING (
--     EXISTS (SELECT 1 FROM send_orders WHERE id = order_id AND driver_id = auth.uid())
--   );
-- CREATE POLICY "Sender can insert stops" ON send_order_stops
--   FOR INSERT WITH CHECK (
--     EXISTS (SELECT 1 FROM send_orders WHERE id = order_id AND sender_id = auth.uid())
--   );
-- CREATE POLICY "Sender can update own stops" ON send_order_stops
--   FOR UPDATE USING (
--     EXISTS (SELECT 1 FROM send_orders WHERE id = order_id AND sender_id = auth.uid())
--   );
-- CREATE POLICY "Driver can update assigned stops" ON send_order_stops
--   FOR UPDATE USING (
--     EXISTS (SELECT 1 FROM send_orders WHERE id = order_id AND driver_id = auth.uid())
--   );

-- TABLE: send_order_events (G-Send — audit trail transisi status)
CREATE TABLE IF NOT EXISTS send_order_events (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id     UUID NOT NULL,

  from_status  send_order_status_enum,
  to_status    send_order_status_enum NOT NULL,
  reason       TEXT,

  triggered_by UUID,
  created_at   TIMESTAMPTZ DEFAULT NOW(),
  metadata     JSONB,

  FOREIGN KEY (order_id) REFERENCES send_orders(id) ON DELETE CASCADE,
  FOREIGN KEY (triggered_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_send_events_order ON send_order_events(order_id);
CREATE INDEX idx_send_events_created ON send_order_events(created_at DESC);

-- RLS (ditunda — keputusan #5).
-- ALTER TABLE send_order_events ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "System can insert send events" ON send_order_events
--   FOR INSERT WITH CHECK (TRUE);

-- FUNCTION: process_cash_settlement (G-Food & G-Send cash settlement)
-- Dipasang ke food_orders & send_orders. Saat order TUNGGU (CASH) berubah ke
-- SETTLED, komisi platform di-DEBIT dari wallet driver & di-CREDIT ke
-- SYSTEM_PLATFORM (driver menyetor kas dari customer). Menggunakan NEW.*
-- (keputusan DATABASE_SCHEMA v10.4 — tanpa SELECT ulang redundant).
CREATE OR REPLACE FUNCTION process_cash_settlement()
RETURNS TRIGGER AS $$
DECLARE
  driver_wallet_id UUID;
  platform_wallet_id UUID;
  commission NUMERIC(12,2);
BEGIN
  IF NEW.payment_method = 'CASH' AND NEW.status = 'SETTLED' AND OLD.status != 'SETTLED' THEN
    driver_wallet_id := NEW.driver_wallet_id;
    commission := NEW.platform_commission;

    IF driver_wallet_id IS NOT NULL AND commission > 0 THEN
      SELECT id INTO platform_wallet_id
      FROM wallets WHERE wallet_type = 'SYSTEM_PLATFORM';

      INSERT INTO ledger_entries (
        wallet_id, entry_type, amount, reference_type, reference_id,
        description, created_by
      ) VALUES (
        driver_wallet_id, 'DEBIT', commission,
        TG_TABLE_NAME || '_COMMISSION', NEW.id,
        'Platform commission deduction for cash order',
        auth.uid()
      );

      INSERT INTO ledger_entries (
        wallet_id, entry_type, amount, reference_type, reference_id,
        description, created_by
      ) VALUES (
        platform_wallet_id, 'CREDIT', commission,
        TG_TABLE_NAME || '_COMMISSION', NEW.id,
        'Platform commission from cash order',
        auth.uid()
      );
    END IF;
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS food_order_cash_settlement ON food_orders;
DROP TRIGGER IF EXISTS send_order_cash_settlement ON send_orders;

CREATE TRIGGER food_order_cash_settlement
AFTER UPDATE OF status ON food_orders
FOR EACH ROW EXECUTE FUNCTION process_cash_settlement();

CREATE TRIGGER send_order_cash_settlement
AFTER UPDATE OF status ON send_orders
FOR EACH ROW EXECUTE FUNCTION process_cash_settlement();

-- POST-MIGRATION SANITY CHECKS (informational — bisa dijalankan manual)
-- SELECT enum_range(NULL::food_order_status_enum);
-- SELECT enum_range(NULL::send_order_status_enum);
-- SELECT enum_range(NULL::package_type_enum);
-- \d food_merchants
-- \d send_orders
-- SELECT count(*) FROM pg_indexes WHERE tablename = 'food_orders';

-- END OF MIGRATION 005.
