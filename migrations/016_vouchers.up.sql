-- MIGRATION 016: VOUCHER DISCOUNT — vouchers, user_vouchers, enums
-- Version : 016
-- Phase   : 3 (Ride Voucher Discount / TD-070)
-- Doc ref : docs/schema_clean.sql (VOUCHERS + USER_VOUCHERS + rollback trigger),
--           migrations/004_ride_orders.up.sql (decision #7 — FK ditambahkan di sini),
--           TECHNICAL DEBT TD-070
--
-- DESIGN DECISIONS (disepakati sebelum menulis migration):
--   1. ENUM: voucher_discount_type_enum & order_type_enum BELUM ada di migration
--      manapun (001 hanya user_type/wallet_type/entry_type; 004 membuat
--      order_status/payment_method; 005 food_order_status/send_order_status/
--      package_type/rating_type). Dua enum ini DIBUAT di sini.
--   2. SCOPE RIDE ONLY (TD-070): ALTER TABLE ride_orders ADD CONSTRAINT
--      fk_ride_voucher. Migration 005 sudah menaruh voucher_id nullable (tanpa FK)
--      di food_orders/send_orders — FK food/send DITUNDA (diluar scope TD-070,
--      dicatat di kritik OpenCode).
--   3. RLS: TIDAK diaktifkan — konsisten dengan migration 002/004/006 (service
--      belum mengatur auth.uid()). Policy dicomment; diaktifkan ulang di
--      Phase 4 (hardening).
--   4. Timestamps memakai TIMESTAMPTZ (keputusan migration 004 #6b) walau
--      schema_clean.sql menulis TIMESTAMP.
--   5. Trigger rollback_voucher_soft_delete: dipasang HANYA untuk ride_orders
--      (sesuai scope). Fungsi tetap generic (CASE TG_TABLE_NAME) agar food/send
--      tinggal menambah trigger saat FK food/send dikerjakan.

-- ENUMS (Phase 3)
CREATE TYPE voucher_discount_type_enum AS ENUM ('PERCENTAGE', 'FIXED');
CREATE TYPE order_type_enum AS ENUM ('RIDE', 'FOOD', 'SEND');

-- ============================================================================
-- VOUCHERS TABLE
-- ============================================================================

CREATE TABLE vouchers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code VARCHAR(50) UNIQUE NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  discount_type voucher_discount_type_enum NOT NULL,
  discount_value NUMERIC(10,2) NOT NULL,
  max_discount NUMERIC(10,2),
  applicable_services order_type_enum[] DEFAULT ARRAY['RIDE'::order_type_enum, 'FOOD'::order_type_enum, 'SEND'::order_type_enum],
  min_order_amount NUMERIC(12,2) DEFAULT 0,
  total_quota INTEGER,
  used_count INTEGER DEFAULT 0,
  per_user_limit INTEGER DEFAULT 1,
  valid_from TIMESTAMPTZ NOT NULL,
  valid_to TIMESTAMPTZ NOT NULL,
  status VARCHAR(20) DEFAULT 'ACTIVE',
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW(),
  created_by UUID,
  CONSTRAINT discount_value_positive CHECK (discount_value > 0),
  CONSTRAINT valid_from_before_valid_to CHECK (valid_from < valid_to),
  CONSTRAINT status_valid CHECK (status IN ('ACTIVE', 'EXPIRED', 'DISABLED')),
  CONSTRAINT check_quota CHECK (used_count <= total_quota),
  FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_vouchers_code ON vouchers(code);
CREATE INDEX idx_vouchers_valid ON vouchers(valid_from, valid_to) WHERE status = 'ACTIVE';
CREATE INDEX idx_vouchers_services ON vouchers USING GIN (applicable_services);

-- ============================================================================
-- USER_VOUCHERS TABLE
-- ============================================================================

CREATE TABLE user_vouchers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL,
  voucher_id UUID NOT NULL,
  order_type order_type_enum,
  order_id UUID,
  discount_amount_applied NUMERIC(12,2) NOT NULL,
  status VARCHAR(20) DEFAULT 'APPLIED',
  applied_at TIMESTAMPTZ DEFAULT NOW(),
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (voucher_id) REFERENCES vouchers(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_user_voucher_unique_applied ON user_vouchers (user_id, voucher_id) WHERE status = 'APPLIED';
CREATE INDEX idx_user_vouchers_user ON user_vouchers(user_id);
CREATE INDEX idx_user_vouchers_voucher ON user_vouchers(voucher_id);
CREATE INDEX idx_user_vouchers_applied ON user_vouchers(applied_at DESC);
CREATE INDEX idx_user_vouchers_status ON user_vouchers(status);

-- ============================================================================
-- RIDE_ORDERS FK (decision #7, migration 004)
-- ============================================================================

ALTER TABLE ride_orders
  ADD CONSTRAINT fk_ride_voucher
  FOREIGN KEY (voucher_id) REFERENCES vouchers(id) ON DELETE SET NULL;

-- ============================================================================
-- TRIGGERS (source: schema_clean.sql)
-- ============================================================================

-- Menjaga per_user_limit pada level DB (defense-in-depth).
CREATE OR REPLACE FUNCTION validate_per_user_limit() RETURNS TRIGGER AS $$
DECLARE current_usage INTEGER; max_limit INTEGER;
BEGIN
  SELECT per_user_limit INTO max_limit FROM vouchers WHERE id = NEW.voucher_id FOR UPDATE;
  SELECT COUNT(*) INTO current_usage FROM user_vouchers WHERE user_id = NEW.user_id AND voucher_id = NEW.voucher_id AND status = 'APPLIED';
  IF current_usage >= max_limit THEN RAISE EXCEPTION 'User has reached maximum usage limit (%) for voucher %', max_limit, NEW.voucher_id; END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER user_voucher_per_user_limit BEFORE INSERT ON user_vouchers FOR EACH ROW EXECUTE FUNCTION validate_per_user_limit();

-- Increment used_count saat voucher terpakai.
CREATE OR REPLACE FUNCTION update_voucher_used_count() RETURNS TRIGGER AS $$
BEGIN UPDATE vouchers SET used_count = used_count + 1, updated_at = NOW() WHERE id = NEW.voucher_id; RETURN NEW; END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER voucher_usage_update AFTER INSERT ON user_vouchers FOR EACH ROW EXECUTE FUNCTION update_voucher_used_count();

-- Decrement used_count saat status berubah APPLIED → CANCELLED (rollback voucher).
CREATE OR REPLACE FUNCTION decrement_voucher_used_count() RETURNS TRIGGER AS $$
BEGIN IF NEW.status = 'CANCELLED' AND OLD.status = 'APPLIED' THEN UPDATE vouchers SET used_count = used_count - 1, updated_at = NOW() WHERE id = NEW.voucher_id AND used_count > 0; END IF; RETURN NEW; END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER voucher_usage_decrement AFTER UPDATE OF status ON user_vouchers FOR EACH ROW EXECUTE FUNCTION decrement_voucher_used_count();

-- Decrement used_count saat row user_vouchers dihapus.
CREATE OR REPLACE FUNCTION decrement_voucher_used_count_on_delete() RETURNS TRIGGER AS $$
BEGIN UPDATE vouchers SET used_count = used_count - 1, updated_at = NOW() WHERE id = OLD.voucher_id AND used_count > 0; RETURN OLD; END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER voucher_usage_decrement_delete AFTER DELETE ON user_vouchers FOR EACH ROW EXECUTE FUNCTION decrement_voucher_used_count_on_delete();

-- Soft-delete voucher usage (APPLIED → CANCELLED) saat order di-cancel.
CREATE OR REPLACE FUNCTION rollback_voucher_soft_delete() RETURNS TRIGGER AS $$
DECLARE order_type_val order_type_enum;
BEGIN
  order_type_val := CASE TG_TABLE_NAME WHEN 'ride_orders' THEN 'RIDE'::order_type_enum WHEN 'food_orders' THEN 'FOOD'::order_type_enum WHEN 'send_orders' THEN 'SEND'::order_type_enum ELSE NULL END;
  IF order_type_val IS NOT NULL AND NEW.status = 'CANCELLED' AND OLD.status != 'CANCELLED' THEN UPDATE user_vouchers SET status = 'CANCELLED' WHERE order_id = NEW.id AND order_type = order_type_val; END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ride_order_rollback_voucher AFTER UPDATE OF status ON ride_orders FOR EACH ROW EXECUTE FUNCTION rollback_voucher_soft_delete();

-- ROW-LEVEL SECURITY (menyusul di Phase 4 / hardening — konsisten keputusan #3)
-- ALTER TABLE vouchers ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Anyone can view active vouchers" ON vouchers FOR SELECT USING (status = 'ACTIVE' AND valid_from <= NOW() AND valid_to >= NOW());
-- CREATE POLICY "Admins can manage vouchers" ON vouchers FOR ALL USING ((auth.jwt() ->> 'user_type') = 'admin');
--
-- ALTER TABLE user_vouchers ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Users can view own voucher usage" ON user_vouchers FOR SELECT USING (user_id = auth.uid());
-- CREATE POLICY "Users can insert own voucher usage" ON user_vouchers FOR INSERT WITH CHECK (user_id = auth.uid());

-- POST-MIGRATION SANITY CHECKS (informational — bisa dijalankan manual)
-- SELECT enum_range(NULL::voucher_discount_type_enum);
-- SELECT enum_range(NULL::order_type_enum);
-- \d vouchers
-- \d user_vouchers
-- \d ride_orders

-- END OF MIGRATION 016.