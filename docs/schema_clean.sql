-- ============================================================================
-- EXTENSIONS
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS cube;
CREATE EXTENSION IF NOT EXISTS earthdistance;

-- ============================================================================
-- ENUMS
-- ============================================================================

CREATE TYPE user_type_enum AS ENUM ('customer', 'driver', 'merchant', 'admin', 'system');
CREATE TYPE wallet_type_enum AS ENUM ('CUSTOMER', 'DRIVER', 'MERCHANT', 'SYSTEM_ESCROW', 'SYSTEM_PLATFORM');
CREATE TYPE entry_type_enum AS ENUM ('DEBIT', 'CREDIT');
CREATE TYPE order_status_enum AS ENUM ('CREATED', 'SEARCHING_DRIVER', 'DRIVER_ASSIGNED', 'DRIVER_ARRIVED', 'TRIP_STARTED', 'COMPLETED', 'CANCELLED', 'SETTLED');
CREATE TYPE food_order_status_enum AS ENUM ('CREATED', 'CONFIRMED', 'PREPARING', 'READY_FOR_PICKUP', 'PICKED_UP', 'IN_TRANSIT', 'DELIVERED', 'CANCELLED', 'SETTLED');
CREATE TYPE send_order_status_enum AS ENUM ('CREATED', 'SEARCHING_DRIVER', 'DRIVER_ASSIGNED', 'PICKED_UP', 'IN_TRANSIT', 'DELIVERED', 'CANCELLED', 'SETTLED');
CREATE TYPE payment_method_enum AS ENUM ('WALLET', 'CASH', 'QRIS', 'CREDIT_CARD');
CREATE TYPE package_type_enum AS ENUM ('STANDARD', 'FRAGILE', 'LIQUID', 'ELECTRONICS');
CREATE TYPE voucher_discount_type_enum AS ENUM ('PERCENTAGE', 'FIXED');
CREATE TYPE rating_type_enum AS ENUM ('RIDE_DRIVER', 'RIDE_CUSTOMER', 'FOOD_MERCHANT', 'FOOD_DELIVERY', 'SEND_DRIVER');
CREATE TYPE order_type_enum AS ENUM ('RIDE', 'FOOD', 'SEND');

-- ============================================================================
-- USERS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email VARCHAR(255) UNIQUE NOT NULL,
  phone VARCHAR(20),
  name VARCHAR(255) NOT NULL,
  user_type user_type_enum NOT NULL,
  status VARCHAR(50) DEFAULT 'ACTIVE',
  is_online BOOLEAN DEFAULT FALSE,
  working_status VARCHAR(20) DEFAULT 'IDLE',
  last_online_at TIMESTAMP,
  last_status_update_at TIMESTAMP,
  password_hash VARCHAR(255) NOT NULL,
  last_login_at TIMESTAMP,
  profile_photo_url TEXT,
  national_id VARCHAR(50),
  primary_address TEXT,
  license_number VARCHAR(50),
  license_expiry DATE,
  vehicle_type VARCHAR(50),
  vehicle_plate VARCHAR(20),
  min_balance_threshold NUMERIC(15,2) DEFAULT 0,
  bank_name VARCHAR(100),
  bank_account_number VARCHAR(50),
  bank_account_name VARCHAR(255),
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
  deleted_at TIMESTAMP,
  CONSTRAINT email_valid CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}$'),
  CONSTRAINT phone_valid CHECK (phone IS NULL OR phone ~* '^\+?[0-9]{10,15}$'),
  CONSTRAINT status_valid CHECK (status IN ('ACTIVE', 'SUSPENDED', 'FROZEN', 'DELETED')),
  CONSTRAINT working_status_valid CHECK (working_status IN ('IDLE', 'BUSY')),
  CONSTRAINT min_balance_non_negative CHECK (min_balance_threshold >= 0),
  CONSTRAINT driver_fields_required CHECK (user_type != 'driver' OR (vehicle_plate IS NOT NULL AND license_number IS NOT NULL))
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_phone ON users(phone);
CREATE INDEX idx_users_type ON users(user_type);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_created ON users(created_at);
CREATE INDEX idx_users_driver_online ON users(user_type, is_online, working_status) WHERE user_type = 'driver' AND is_online = TRUE AND working_status = 'IDLE';

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Users can view own data" ON users FOR SELECT USING (id = auth.uid());
CREATE POLICY "Users can update own data" ON users FOR UPDATE USING (id = auth.uid());
CREATE POLICY "Admins can view all users" ON users FOR SELECT USING ((auth.jwt() ->> 'user_type') = 'admin');

-- ============================================================================
-- DRIVER_LOCATIONS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS driver_locations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  driver_id UUID NOT NULL UNIQUE,
  current_lat NUMERIC(10,8) NOT NULL,
  current_lng NUMERIC(11,8) NOT NULL,
  updated_at TIMESTAMP DEFAULT NOW(),
  FOREIGN KEY (driver_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_driver_locations_gist ON driver_locations USING GIST (ll_to_earth(current_lat, current_lng));
CREATE INDEX idx_driver_locations_driver ON driver_locations(driver_id);
CREATE INDEX idx_driver_locations_updated ON driver_locations(updated_at DESC);

CREATE OR REPLACE FUNCTION update_driver_online_timestamp() RETURNS TRIGGER AS $$
BEGIN
  UPDATE users SET last_online_at = NOW(), updated_at = NOW() WHERE id = NEW.driver_id AND user_type = 'driver';
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER driver_location_online_update AFTER INSERT OR UPDATE ON driver_locations FOR EACH ROW EXECUTE FUNCTION update_driver_online_timestamp();

ALTER TABLE driver_locations ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Drivers can update own location" ON driver_locations FOR INSERT WITH CHECK (driver_id = auth.uid());
CREATE POLICY "Drivers can update own location" ON driver_locations FOR UPDATE USING (driver_id = auth.uid());
CREATE POLICY "Anyone can view driver locations" ON driver_locations FOR SELECT USING (TRUE);

-- ============================================================================
-- WALLETS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS wallets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID,
  wallet_type wallet_type_enum NOT NULL DEFAULT 'CUSTOMER',
  balance NUMERIC(15,2) NOT NULL DEFAULT 0,
  status VARCHAR(50) DEFAULT 'ACTIVE',
  currency VARCHAR(3) DEFAULT 'IDR',
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
  CONSTRAINT wallet_user_type_unique UNIQUE (user_id, wallet_type)
);

ALTER TABLE wallets ADD CONSTRAINT balance_non_negative CHECK (balance >= -1000000);

CREATE INDEX idx_wallets_user ON wallets(user_id);
CREATE INDEX idx_wallets_type ON wallets(wallet_type);
CREATE INDEX idx_wallets_status ON wallets(status);

CREATE UNIQUE INDEX idx_system_wallet_unique ON wallets(wallet_type) WHERE user_id IS NULL;

INSERT INTO wallets (user_id, wallet_type, balance, status) VALUES (NULL, 'SYSTEM_ESCROW', 0, 'ACTIVE'), (NULL, 'SYSTEM_PLATFORM', 0, 'ACTIVE') ON CONFLICT (wallet_type) WHERE user_id IS NULL DO NOTHING;

ALTER TABLE wallets ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Users can view own wallet" ON wallets FOR SELECT USING (user_id = auth.uid() OR wallet_type IN ('SYSTEM_ESCROW', 'SYSTEM_PLATFORM'));
CREATE POLICY "Users can insert own wallet" ON wallets FOR INSERT WITH CHECK (user_id = auth.uid() AND wallet_type IN ('CUSTOMER', 'DRIVER', 'MERCHANT'));
CREATE POLICY "Admin can insert system wallets" ON wallets FOR INSERT WITH CHECK (wallet_type IN ('SYSTEM_ESCROW', 'SYSTEM_PLATFORM') AND (auth.jwt() ->> 'user_type') IN ('admin', 'system'));
CREATE POLICY "Users can update own wallet" ON wallets FOR UPDATE USING (user_id = auth.uid());
CREATE POLICY "Admin can update system wallets" ON wallets FOR UPDATE USING ((auth.jwt() ->> 'user_type') IN ('admin', 'system'));

-- ============================================================================
-- LEDGER_ENTRIES TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS ledger_entries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  wallet_id UUID NOT NULL,
  entry_type entry_type_enum NOT NULL,
  amount NUMERIC(15,2) NOT NULL,
  reference_type VARCHAR(100),
  reference_id UUID,
  description TEXT,
  created_at TIMESTAMP DEFAULT NOW(),
  created_by UUID,
  balance_after NUMERIC(15,2),
  is_reversed BOOLEAN DEFAULT FALSE,
  reversal_of_id UUID,
  CONSTRAINT amount_positive CHECK (amount > 0),
  FOREIGN KEY (wallet_id) REFERENCES wallets(id) ON DELETE CASCADE,
  FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
  FOREIGN KEY (reversal_of_id) REFERENCES ledger_entries(id) ON DELETE SET NULL
);

CREATE INDEX idx_ledger_wallet ON ledger_entries(wallet_id);
CREATE INDEX idx_ledger_reference ON ledger_entries(reference_type, reference_id);
CREATE INDEX idx_ledger_created ON ledger_entries(created_at DESC);
CREATE INDEX idx_ledger_type ON ledger_entries(entry_type);
CREATE INDEX idx_ledger_reversal ON ledger_entries(is_reversed) WHERE is_reversed = TRUE;

CREATE OR REPLACE FUNCTION validate_ledger_balance() RETURNS TRIGGER AS $$
DECLARE total_debit NUMERIC(15,2); total_credit NUMERIC(15,2);
BEGIN
  IF NEW.reference_id IS NULL AND (NEW.reference_type IS NULL OR NEW.reference_type = '') THEN
    RAISE EXCEPTION 'reference_id wajib diisi untuk transaksi finansial (kecuali system adjustment)';
  END IF;
  IF NEW.reference_id IS NULL THEN RETURN NEW; END IF;
  SELECT COALESCE(SUM(CASE WHEN entry_type = 'DEBIT' AND is_reversed = FALSE THEN amount ELSE 0 END), 0), COALESCE(SUM(CASE WHEN entry_type = 'CREDIT' AND is_reversed = FALSE THEN amount ELSE 0 END), 0) INTO total_debit, total_credit FROM ledger_entries WHERE reference_id = NEW.reference_id AND is_reversed = FALSE;
  IF total_debit != total_credit THEN RAISE EXCEPTION 'Ledger imbalance: DEBIT (%) != CREDIT (%) for reference_id %', total_debit, total_credit, NEW.reference_id; END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER ledger_balance_validation AFTER INSERT ON ledger_entries DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION validate_ledger_balance();

CREATE OR REPLACE FUNCTION raise_immutability_error() RETURNS TRIGGER AS $$
BEGIN
  IF (TG_OP = 'UPDATE') THEN
    IF (OLD.wallet_id != NEW.wallet_id OR OLD.entry_type != NEW.entry_type OR OLD.amount != NEW.amount OR OLD.reference_type != NEW.reference_type OR OLD.reference_id != NEW.reference_id OR OLD.description != NEW.description) THEN
      RAISE EXCEPTION 'Ledger entries are immutable except is_reversed flag';
    END IF;
  ELSIF (TG_OP = 'DELETE') THEN RAISE EXCEPTION 'Ledger entries cannot be deleted';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ledger_immutable BEFORE UPDATE OR DELETE ON ledger_entries FOR EACH ROW EXECUTE FUNCTION raise_immutability_error();

CREATE OR REPLACE FUNCTION sync_wallet_balance() RETURNS TRIGGER AS $$
DECLARE new_balance NUMERIC(15,2); current_balance NUMERIC(15,2); is_admin BOOLEAN;
BEGIN
  IF (TG_OP = 'INSERT') THEN
    SELECT balance INTO current_balance FROM wallets WHERE id = NEW.wallet_id FOR UPDATE;
    UPDATE wallets SET balance = balance + CASE WHEN NEW.entry_type = 'CREDIT' THEN NEW.amount ELSE -NEW.amount END, updated_at = NOW() WHERE id = NEW.wallet_id RETURNING balance INTO new_balance;
    NEW.balance_after := new_balance;
    RETURN NEW;
  ELSIF (TG_OP = 'UPDATE') THEN
    IF (NEW.is_reversed = TRUE AND OLD.is_reversed = FALSE) THEN
      SELECT balance INTO current_balance FROM wallets WHERE id = OLD.wallet_id FOR UPDATE;
      is_admin := ((auth.jwt() ->> 'user_type') IN ('admin', 'system'));
      IF (OLD.entry_type = 'CREDIT' AND current_balance < OLD.amount) THEN
        IF is_admin THEN RAISE NOTICE 'Admin reversal: wallet % balance % < reversal amount %', OLD.wallet_id, current_balance, OLD.amount;
        ELSE RAISE EXCEPTION 'Insufficient balance for reversal: wallet_id % has %, need %', OLD.wallet_id, current_balance, OLD.amount; END IF;
      END IF;
      UPDATE wallets SET balance = balance - CASE WHEN OLD.entry_type = 'CREDIT' THEN OLD.amount ELSE -OLD.amount END, updated_at = NOW() WHERE id = NEW.wallet_id;
    END IF;
    RETURN NEW;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ledger_sync_balance BEFORE INSERT OR UPDATE OF is_reversed ON ledger_entries FOR EACH ROW EXECUTE FUNCTION sync_wallet_balance();

ALTER TABLE ledger_entries ENABLE ROW LEVEL SECURITY;
CREATE POLICY "System can manage ledger entries" ON ledger_entries FOR ALL USING ((auth.jwt() ->> 'user_type') IN ('admin', 'system'));

-- ============================================================================
-- IDEMPOTENCY_CACHE TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS idempotency_cache (
  key UUID PRIMARY KEY,
  user_id UUID NOT NULL,
  request_hash VARCHAR(64),
  request_body JSONB,
  response_body JSONB NOT NULL,
  status_code INTEGER NOT NULL,
  created_at TIMESTAMP DEFAULT NOW(),
  expires_at TIMESTAMP NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_idempotency_user ON idempotency_cache(user_id);
CREATE INDEX idx_idempotency_expires ON idempotency_cache(expires_at);

ALTER TABLE idempotency_cache ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Users can view own idempotency cache" ON idempotency_cache FOR SELECT USING (user_id = auth.uid());
CREATE POLICY "Users can insert own idempotency cache" ON idempotency_cache FOR INSERT WITH CHECK (user_id = auth.uid());

-- ============================================================================
-- TOPUP_TRANSACTIONS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS topup_transactions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL,
  wallet_id UUID NOT NULL,
  amount NUMERIC(15,2) NOT NULL,
  fee NUMERIC(10,2) DEFAULT 0,
  net_amount NUMERIC(15,2) NOT NULL,
  payment_method VARCHAR(50) DEFAULT 'SIMULATED',
  status VARCHAR(50) DEFAULT 'PENDING',
  external_transaction_id VARCHAR(255),
  gateway_response JSONB,
  requested_at TIMESTAMP DEFAULT NOW(),
  completed_at TIMESTAMP,
  expired_at TIMESTAMP,
  notes TEXT,
  ledger_entry_id UUID,
  CONSTRAINT amount_positive CHECK (amount > 0),
  CONSTRAINT status_valid CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED', 'EXPIRED')),
  CONSTRAINT payment_method_valid CHECK (payment_method IN ('SIMULATED', 'BANK_TRANSFER', 'QRIS', 'CREDIT_CARD')),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (ledger_entry_id) REFERENCES ledger_entries(id) ON DELETE SET NULL
);

CREATE INDEX idx_topup_user ON topup_transactions(user_id);
CREATE INDEX idx_topup_wallet ON topup_transactions(wallet_id);
CREATE INDEX idx_topup_status ON topup_transactions(status);
CREATE INDEX idx_topup_created ON topup_transactions(requested_at DESC);
CREATE INDEX idx_topup_external ON topup_transactions(external_transaction_id);

ALTER TABLE topup_transactions ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Users can view own topups" ON topup_transactions FOR SELECT USING (user_id = auth.uid());
CREATE POLICY "Users can insert topup" ON topup_transactions FOR INSERT WITH CHECK (user_id = auth.uid());
CREATE POLICY "Users can update own topup" ON topup_transactions FOR UPDATE USING (user_id = auth.uid() OR status IN ('COMPLETED', 'FAILED'));

-- ============================================================================
-- WITHDRAWAL_REQUESTS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS withdrawal_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL,
  wallet_id UUID NOT NULL,
  amount NUMERIC(15,2) NOT NULL,
  fee NUMERIC(10,2) DEFAULT 0,
  net_amount NUMERIC(15,2) NOT NULL,
  bank_name VARCHAR(100) NOT NULL,
  bank_account_number VARCHAR(50) NOT NULL,
  bank_account_name VARCHAR(255) NOT NULL,
  status VARCHAR(50) DEFAULT 'PENDING',
  external_transaction_id VARCHAR(255),
  gateway_response JSONB,
  requested_at TIMESTAMP DEFAULT NOW(),
  processed_at TIMESTAMP,
  completed_at TIMESTAMP,
  ledger_entry_id UUID,
  admin_notes TEXT,
  rejection_reason TEXT,
  CONSTRAINT amount_positive CHECK (amount > 0),
  CONSTRAINT status_valid CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED', 'REJECTED')),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (ledger_entry_id) REFERENCES ledger_entries(id) ON DELETE SET NULL
);

CREATE INDEX idx_withdrawal_user ON withdrawal_requests(user_id);
CREATE INDEX idx_withdrawal_wallet ON withdrawal_requests(wallet_id);
CREATE INDEX idx_withdrawal_status ON withdrawal_requests(status);
CREATE INDEX idx_withdrawal_created ON withdrawal_requests(requested_at DESC);

ALTER TABLE withdrawal_requests ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Users can view own withdrawals" ON withdrawal_requests FOR SELECT USING (user_id = auth.uid());
CREATE POLICY "Users can insert withdrawal" ON withdrawal_requests FOR INSERT WITH CHECK (user_id = auth.uid());
CREATE POLICY "Users can update own withdrawal" ON withdrawal_requests FOR UPDATE USING (user_id = auth.uid() OR status IN ('COMPLETED', 'FAILED', 'REJECTED'));

-- ============================================================================
-- PAYMENT_GATEWAY_LOGS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS payment_gateway_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  transaction_type VARCHAR(50),
  transaction_id UUID,
  gateway_name VARCHAR(50),
  endpoint VARCHAR(255),
  method VARCHAR(10),
  request_payload JSONB,
  response_payload JSONB,
  http_status INTEGER,
  created_at TIMESTAMP DEFAULT NOW(),
  response_time_ms INTEGER,
  error_message TEXT,
  CONSTRAINT transaction_type_valid CHECK (transaction_type IN ('TOPUP', 'WITHDRAWAL', 'PAYMENT'))
);

CREATE INDEX idx_gateway_logs_transaction ON payment_gateway_logs(transaction_type, transaction_id);
CREATE INDEX idx_gateway_logs_created ON payment_gateway_logs(created_at DESC);
CREATE INDEX idx_gateway_logs_gateway ON payment_gateway_logs(gateway_name);

ALTER TABLE payment_gateway_logs ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Admins can view all gateway logs" ON payment_gateway_logs FOR ALL USING ((auth.jwt() ->> 'user_type') = 'admin');
CREATE POLICY "System can insert gateway logs" ON payment_gateway_logs FOR INSERT WITH CHECK (TRUE);

-- ============================================================================
-- VOUCHERS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS vouchers (
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
  valid_from TIMESTAMP NOT NULL,
  valid_to TIMESTAMP NOT NULL,
  status VARCHAR(20) DEFAULT 'ACTIVE',
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
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

ALTER TABLE vouchers ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Anyone can view active vouchers" ON vouchers FOR SELECT USING (status = 'ACTIVE' AND valid_from <= NOW() AND valid_to >= NOW());
CREATE POLICY "Admins can manage vouchers" ON vouchers FOR ALL USING ((auth.jwt() ->> 'user_type') = 'admin');

-- ============================================================================
-- USER_VOUCHERS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS user_vouchers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL,
  voucher_id UUID NOT NULL,
  order_type order_type_enum,
  order_id UUID,
  discount_amount_applied NUMERIC(12,2) NOT NULL,
  status VARCHAR(20) DEFAULT 'APPLIED',
  applied_at TIMESTAMP DEFAULT NOW(),
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (voucher_id) REFERENCES vouchers(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_user_voucher_unique_applied ON user_vouchers (user_id, voucher_id) WHERE status = 'APPLIED';
CREATE INDEX idx_user_vouchers_user ON user_vouchers(user_id);
CREATE INDEX idx_user_vouchers_voucher ON user_vouchers(voucher_id);
CREATE INDEX idx_user_vouchers_applied ON user_vouchers(applied_at DESC);
CREATE INDEX idx_user_vouchers_status ON user_vouchers(status);

ALTER TABLE user_vouchers ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Users can view own voucher usage" ON user_vouchers FOR SELECT USING (user_id = auth.uid());
CREATE POLICY "Users can insert own voucher usage" ON user_vouchers FOR INSERT WITH CHECK (
  user_id = auth.uid() AND (
    (order_type = 'RIDE' AND EXISTS (SELECT 1 FROM ride_orders WHERE id = order_id AND customer_id = auth.uid())) OR
    (order_type = 'FOOD' AND EXISTS (SELECT 1 FROM food_orders WHERE id = order_id AND customer_id = auth.uid())) OR
    (order_type = 'SEND' AND EXISTS (SELECT 1 FROM send_orders WHERE id = order_id AND sender_id = auth.uid()))
  )
);

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

CREATE OR REPLACE FUNCTION update_voucher_used_count() RETURNS TRIGGER AS $$
BEGIN UPDATE vouchers SET used_count = used_count + 1, updated_at = NOW() WHERE id = NEW.voucher_id; RETURN NEW; END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER voucher_usage_update AFTER INSERT ON user_vouchers FOR EACH ROW EXECUTE FUNCTION update_voucher_used_count();

CREATE OR REPLACE FUNCTION decrement_voucher_used_count() RETURNS TRIGGER AS $$
BEGIN IF NEW.status = 'CANCELLED' AND OLD.status = 'APPLIED' THEN UPDATE vouchers SET used_count = used_count - 1, updated_at = NOW() WHERE id = NEW.voucher_id AND used_count > 0; END IF; RETURN NEW; END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER voucher_usage_decrement AFTER UPDATE OF status ON user_vouchers FOR EACH ROW EXECUTE FUNCTION decrement_voucher_used_count();

CREATE OR REPLACE FUNCTION decrement_voucher_used_count_on_delete() RETURNS TRIGGER AS $$
BEGIN UPDATE vouchers SET used_count = used_count - 1, updated_at = NOW() WHERE id = OLD.voucher_id AND used_count > 0; RETURN OLD; END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER voucher_usage_decrement_delete AFTER DELETE ON user_vouchers FOR EACH ROW EXECUTE FUNCTION decrement_voucher_used_count_on_delete();

-- ============================================================================
-- RIDE_ORDERS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS ride_orders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id UUID NOT NULL,
  driver_id UUID,
  customer_wallet_id UUID,
  driver_wallet_id UUID,
  pickup_lat NUMERIC(10,8) NOT NULL,
  pickup_lng NUMERIC(11,8) NOT NULL,
  pickup_address TEXT NOT NULL,
  dropoff_lat NUMERIC(10,8) NOT NULL,
  dropoff_lng NUMERIC(11,8) NOT NULL,
  dropoff_address TEXT NOT NULL,
  distance_km NUMERIC(8,3),
  base_fare NUMERIC(12,2) NOT NULL DEFAULT 10000.00,
  per_km_rate NUMERIC(8,2) NOT NULL DEFAULT 4000.00,
  estimated_fare NUMERIC(12,2) NOT NULL,
  actual_fare NUMERIC(12,2),
  surge_multiplier NUMERIC(3,2) DEFAULT 1.00,
  toll_fee NUMERIC(12,2) DEFAULT 0,
  cancellation_fee NUMERIC(12,2) DEFAULT 0,
  discount_amount NUMERIC(12,2) DEFAULT 0,
  voucher_id UUID,
  payment_method payment_method_enum DEFAULT 'WALLET',
  platform_commission NUMERIC(12,2),
  driver_earning NUMERIC(12,2),
  status order_status_enum DEFAULT 'CREATED',
  cancellation_reason VARCHAR(255),
  created_at TIMESTAMP DEFAULT NOW(),
  assigned_at TIMESTAMP,
  pickup_at TIMESTAMP,
  completed_at TIMESTAMP,
  settled_at TIMESTAMP,
  is_settled BOOLEAN DEFAULT FALSE,
  settlement_notes TEXT,
  CONSTRAINT fare_positive CHECK (estimated_fare > 0),
  CONSTRAINT discount_non_negative CHECK (discount_amount >= 0),
  CONSTRAINT surge_positive CHECK (surge_multiplier >= 1.0),
  CONSTRAINT toll_non_negative CHECK (toll_fee >= 0),
  CONSTRAINT cancellation_fee_non_negative CHECK (cancellation_fee >= 0),
  FOREIGN KEY (customer_id) REFERENCES users(id),
  FOREIGN KEY (driver_id) REFERENCES users(id),
  FOREIGN KEY (customer_wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (driver_wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (voucher_id) REFERENCES vouchers(id) ON DELETE SET NULL
);

CREATE INDEX idx_ride_customer ON ride_orders(customer_id);
CREATE INDEX idx_ride_driver ON ride_orders(driver_id);
CREATE INDEX idx_ride_status ON ride_orders(status);
CREATE INDEX idx_ride_status_created ON ride_orders(status, created_at DESC) WHERE status = 'SEARCHING_DRIVER';
CREATE INDEX idx_ride_created ON ride_orders(created_at DESC);
CREATE INDEX idx_ride_settled ON ride_orders(is_settled);
CREATE INDEX idx_ride_payment ON ride_orders(payment_method);

ALTER TABLE ride_orders ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Users can view own ride orders" ON ride_orders FOR SELECT USING (customer_id = auth.uid() OR driver_id = auth.uid());
CREATE POLICY "Users can insert ride orders" ON ride_orders FOR INSERT WITH CHECK (customer_id = auth.uid());
CREATE POLICY "Users can update own ride orders" ON ride_orders FOR UPDATE USING (customer_id = auth.uid() OR driver_id = auth.uid());

CREATE OR REPLACE FUNCTION rollback_voucher_soft_delete() RETURNS TRIGGER AS $$
DECLARE order_type_val order_type_enum;
BEGIN
  order_type_val := CASE TG_TABLE_NAME WHEN 'ride_orders' THEN 'RIDE'::order_type_enum WHEN 'food_orders' THEN 'FOOD'::order_type_enum WHEN 'send_orders' THEN 'SEND'::order_type_enum ELSE NULL END;
  IF order_type_val IS NOT NULL AND NEW.status = 'CANCELLED' AND OLD.status != 'CANCELLED' THEN UPDATE user_vouchers SET status = 'CANCELLED' WHERE order_id = NEW.id AND order_type = order_type_val; END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ride_order_rollback_voucher AFTER UPDATE OF status ON ride_orders FOR EACH ROW EXECUTE FUNCTION rollback_voucher_soft_delete();

CREATE OR REPLACE FUNCTION process_cash_settlement() RETURNS TRIGGER AS $$
DECLARE driver_wallet_id UUID; platform_wallet_id UUID; commission NUMERIC(12,2);
BEGIN
  IF NEW.payment_method = 'CASH' AND NEW.status = 'SETTLED' AND OLD.status != 'SETTLED' THEN
    driver_wallet_id := NEW.driver_wallet_id;
    commission := NEW.platform_commission;
    IF driver_wallet_id IS NOT NULL AND commission > 0 THEN
      SELECT id INTO platform_wallet_id FROM wallets WHERE wallet_type = 'SYSTEM_PLATFORM';
      INSERT INTO ledger_entries (wallet_id, entry_type, amount, reference_type, reference_id, description, created_by) VALUES (driver_wallet_id, 'DEBIT', commission, 'PLATFORM_FEE', NEW.id, 'Platform commission deduction for cash order', auth.uid());
      INSERT INTO ledger_entries (wallet_id, entry_type, amount, reference_type, reference_id, description, created_by) VALUES (platform_wallet_id, 'CREDIT', commission, 'PLATFORM_FEE', NEW.id, 'Platform commission from cash order', auth.uid());
    END IF;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS ride_order_cash_settlement ON ride_orders;
DROP TRIGGER IF EXISTS food_order_cash_settlement ON food_orders;
DROP TRIGGER IF EXISTS send_order_cash_settlement ON send_orders;

CREATE TRIGGER ride_order_cash_settlement AFTER UPDATE OF status ON ride_orders FOR EACH ROW EXECUTE FUNCTION process_cash_settlement();

-- ============================================================================
-- RIDE_ORDER_EVENTS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS ride_order_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id UUID NOT NULL,
  from_status order_status_enum,
  to_status order_status_enum NOT NULL,
  reason TEXT,
  triggered_by UUID,
  created_at TIMESTAMP DEFAULT NOW(),
  metadata JSONB,
  FOREIGN KEY (order_id) REFERENCES ride_orders(id) ON DELETE CASCADE,
  FOREIGN KEY (triggered_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_ride_events_order ON ride_order_events(order_id);
CREATE INDEX idx_ride_events_created ON ride_order_events(created_at DESC);

ALTER TABLE ride_order_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Users can view own ride events" ON ride_order_events FOR SELECT USING (EXISTS (SELECT 1 FROM ride_orders WHERE id = order_id AND (customer_id = auth.uid() OR driver_id = auth.uid())));
CREATE POLICY "System can insert ride events" ON ride_order_events FOR INSERT WITH CHECK (TRUE);

-- ============================================================================
-- FOOD_MERCHANTS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS food_merchants (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL UNIQUE,
  merchant_name VARCHAR(255) NOT NULL,
  merchant_description TEXT,
  category VARCHAR(50) NOT NULL,
  latitude NUMERIC(10,8) NOT NULL,
  longitude NUMERIC(11,8) NOT NULL,
  address TEXT NOT NULL,
  phone VARCHAR(20),
  avg_rating NUMERIC(3,2) DEFAULT 5.0,
  total_reviews INTEGER DEFAULT 0,
  total_orders INTEGER DEFAULT 0,
  opening_time TIME,
  closing_time TIME,
  is_open BOOLEAN DEFAULT TRUE,
  status VARCHAR(50) DEFAULT 'ACTIVE',
  verified_at TIMESTAMP,
  logo_url TEXT,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
  CONSTRAINT rating_valid CHECK (avg_rating >= 0 AND avg_rating <= 5),
  CONSTRAINT status_valid CHECK (status IN ('ACTIVE', 'PENDING_VERIFICATION', 'SUSPENDED', 'CLOSED')),
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_food_merchants_category ON food_merchants(category);
CREATE INDEX idx_food_merchants_location ON food_merchants USING GIST(ll_to_earth(latitude, longitude));
CREATE INDEX idx_food_merchants_status ON food_merchants(status);

ALTER TABLE food_merchants ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Anyone can view active merchants" ON food_merchants FOR SELECT USING (status = 'ACTIVE' OR user_id = auth.uid());
CREATE POLICY "Merchants can update own data" ON food_merchants FOR UPDATE USING (user_id = auth.uid());

-- ============================================================================
-- MERCHANT_MENUS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS merchant_menus (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  merchant_id UUID NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  sequence_order INTEGER DEFAULT 0,
  is_active BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
  FOREIGN KEY (merchant_id) REFERENCES food_merchants(id) ON DELETE CASCADE,
  UNIQUE(merchant_id, name)
);

CREATE INDEX idx_merchant_menus_merchant ON merchant_menus(merchant_id);

ALTER TABLE merchant_menus ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Anyone can view active menus" ON merchant_menus FOR SELECT USING (is_active = TRUE);
CREATE POLICY "Merchants can manage own menus" ON merchant_menus FOR ALL USING (merchant_id IN (SELECT id FROM food_merchants WHERE user_id = auth.uid()));

-- ============================================================================
-- MERCHANT_ITEMS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS merchant_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  menu_id UUID NOT NULL,
  merchant_id UUID NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  price NUMERIC(10,2) NOT NULL,
  image_url TEXT,
  stock INTEGER DEFAULT 999,
  is_available BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
  CONSTRAINT price_positive CHECK (price > 0),
  FOREIGN KEY (menu_id) REFERENCES merchant_menus(id) ON DELETE CASCADE,
  FOREIGN KEY (merchant_id) REFERENCES food_merchants(id) ON DELETE CASCADE
);

CREATE INDEX idx_merchant_items_menu ON merchant_items(menu_id);
CREATE INDEX idx_merchant_items_merchant ON merchant_items(merchant_id);
CREATE INDEX idx_merchant_items_available ON merchant_items(is_available) WHERE is_available = TRUE;

ALTER TABLE merchant_items ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Anyone can view available items" ON merchant_items FOR SELECT USING (is_available = TRUE);
CREATE POLICY "Merchants can manage own items" ON merchant_items FOR ALL USING (merchant_id IN (SELECT id FROM food_merchants WHERE user_id = auth.uid()));

-- ============================================================================
-- ITEM_OPTION_GROUPS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS item_option_groups (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  merchant_id UUID NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  selection_type VARCHAR(20) DEFAULT 'SINGLE',
  is_required BOOLEAN DEFAULT FALSE,
  max_choices INTEGER DEFAULT 1,
  sequence_order INTEGER DEFAULT 0,
  is_active BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
  CONSTRAINT selection_type_valid CHECK (selection_type IN ('SINGLE', 'MULTIPLE')),
  CONSTRAINT max_choices_positive CHECK (max_choices >= 1),
  FOREIGN KEY (merchant_id) REFERENCES food_merchants(id) ON DELETE CASCADE,
  UNIQUE(merchant_id, name)
);

CREATE INDEX idx_option_groups_merchant ON item_option_groups(merchant_id);

ALTER TABLE item_option_groups ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Anyone can view active option groups" ON item_option_groups FOR SELECT USING (is_active = TRUE);
CREATE POLICY "Merchants can manage own option groups" ON item_option_groups FOR ALL USING (merchant_id IN (SELECT id FROM food_merchants WHERE user_id = auth.uid()));

-- ============================================================================
-- ITEM_OPTIONS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS item_options (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  option_group_id UUID NOT NULL,
  merchant_id UUID NOT NULL,
  name VARCHAR(255) NOT NULL,
  price_adjustment NUMERIC(10,2) DEFAULT 0,
  description TEXT,
  sequence_order INTEGER DEFAULT 0,
  is_available BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
  CONSTRAINT price_adjustment_non_negative CHECK (price_adjustment >= 0),
  FOREIGN KEY (option_group_id) REFERENCES item_option_groups(id) ON DELETE CASCADE,
  FOREIGN KEY (merchant_id) REFERENCES food_merchants(id) ON DELETE CASCADE
);

CREATE INDEX idx_options_group ON item_options(option_group_id);
CREATE INDEX idx_options_merchant ON item_options(merchant_id);

ALTER TABLE item_options ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Anyone can view available options" ON item_options FOR SELECT USING (is_available = TRUE);
CREATE POLICY "Merchants can manage own options" ON item_options FOR ALL USING (merchant_id IN (SELECT id FROM food_merchants WHERE user_id = auth.uid()));

-- ============================================================================
-- MERCHANT_ITEM_OPTION_GROUPS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS merchant_item_option_groups (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  item_id UUID NOT NULL,
  option_group_id UUID NOT NULL,
  sequence_order INTEGER DEFAULT 0,
  is_active BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
  FOREIGN KEY (item_id) REFERENCES merchant_items(id) ON DELETE CASCADE,
  FOREIGN KEY (option_group_id) REFERENCES item_option_groups(id) ON DELETE CASCADE,
  UNIQUE(item_id, option_group_id)
);

CREATE INDEX idx_item_option_groups_item ON merchant_item_option_groups(item_id);
CREATE INDEX idx_item_option_groups_group ON merchant_item_option_groups(option_group_id);

ALTER TABLE merchant_item_option_groups ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Anyone can view active item options" ON merchant_item_option_groups FOR SELECT USING (is_active = TRUE);
CREATE POLICY "Merchants can manage own item options" ON merchant_item_option_groups FOR ALL USING (EXISTS (SELECT 1 FROM merchant_items WHERE id = item_id AND merchant_id IN (SELECT id FROM food_merchants WHERE user_id = auth.uid())));

-- ============================================================================
-- FOOD_ORDERS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS food_orders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id UUID NOT NULL,
  merchant_id UUID NOT NULL,
  driver_id UUID,
  customer_wallet_id UUID,
  merchant_wallet_id UUID,
  driver_wallet_id UUID,
  delivery_address TEXT NOT NULL,
  delivery_lat NUMERIC(10,8),
  delivery_lng NUMERIC(11,8),
  special_instructions TEXT,
  item_subtotal NUMERIC(12,2) NOT NULL,
  delivery_fee NUMERIC(10,2) NOT NULL,
  platform_commission NUMERIC(10,2),
  driver_earning NUMERIC(12,2),
  discount_amount NUMERIC(10,2) DEFAULT 0,
  voucher_id UUID,
  payment_method payment_method_enum DEFAULT 'WALLET',
  cutlery_included BOOLEAN DEFAULT FALSE,
  total_amount NUMERIC(12,2) NOT NULL,
  status food_order_status_enum DEFAULT 'CREATED',
  merchant_status VARCHAR(50) DEFAULT 'WAITING',
  merchant_notes TEXT,
  created_at TIMESTAMP DEFAULT NOW(),
  confirmed_at TIMESTAMP,
  pickup_at TIMESTAMP,
  delivered_at TIMESTAMP,
  settled_at TIMESTAMP,
  is_settled BOOLEAN DEFAULT FALSE,
  is_refunded BOOLEAN DEFAULT FALSE,
  CONSTRAINT amount_positive CHECK (total_amount > 0),
  CONSTRAINT discount_non_negative CHECK (discount_amount >= 0),
  CONSTRAINT driver_earning_non_negative CHECK (driver_earning >= 0),
  FOREIGN KEY (customer_id) REFERENCES users(id),
  FOREIGN KEY (merchant_id) REFERENCES food_merchants(id),
  FOREIGN KEY (driver_id) REFERENCES users(id),
  FOREIGN KEY (customer_wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (merchant_wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (driver_wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (voucher_id) REFERENCES vouchers(id) ON DELETE SET NULL
);

CREATE OR REPLACE FUNCTION calc_food_driver_earning() RETURNS TRIGGER AS $$
BEGIN NEW.driver_earning := NEW.delivery_fee * 0.9; RETURN NEW; END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER food_driver_earning_calc BEFORE INSERT ON food_orders FOR EACH ROW EXECUTE FUNCTION calc_food_driver_earning();

CREATE INDEX idx_food_orders_customer ON food_orders(customer_id);
CREATE INDEX idx_food_orders_merchant ON food_orders(merchant_id);
CREATE INDEX idx_food_orders_driver ON food_orders(driver_id);
CREATE INDEX idx_food_orders_status ON food_orders(status);
CREATE INDEX idx_food_orders_created ON food_orders(created_at DESC);
CREATE INDEX idx_food_orders_settled ON food_orders(is_settled);
CREATE INDEX idx_food_orders_payment ON food_orders(payment_method);

ALTER TABLE food_orders ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Users can view own food orders" ON food_orders FOR SELECT USING (customer_id = auth.uid() OR merchant_id = auth.uid() OR driver_id = auth.uid());
CREATE POLICY "Users can insert food orders" ON food_orders FOR INSERT WITH CHECK (customer_id = auth.uid());
CREATE POLICY "Users can update own food orders" ON food_orders FOR UPDATE USING (customer_id = auth.uid() OR merchant_id = auth.uid() OR driver_id = auth.uid());

CREATE TRIGGER food_order_rollback_voucher AFTER UPDATE OF status ON food_orders FOR EACH ROW EXECUTE FUNCTION rollback_voucher_soft_delete();
CREATE TRIGGER food_order_cash_settlement AFTER UPDATE OF status ON food_orders FOR EACH ROW EXECUTE FUNCTION process_cash_settlement();

-- ============================================================================
-- FOOD_ORDER_ITEMS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS food_order_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id UUID NOT NULL,
  item_id UUID NOT NULL,
  item_name VARCHAR(255) NOT NULL,
  item_price NUMERIC(10,2) NOT NULL,
  quantity INTEGER NOT NULL,
  subtotal NUMERIC(12,2) NOT NULL,
  options JSONB DEFAULT '{}'::jsonb,
  options_total NUMERIC(10,2) DEFAULT 0,
  special_instructions TEXT,
  created_at TIMESTAMP DEFAULT NOW(),
  CONSTRAINT quantity_positive CHECK (quantity > 0),
  CONSTRAINT options_total_non_negative CHECK (options_total >= 0),
  FOREIGN KEY (order_id) REFERENCES food_orders(id) ON DELETE CASCADE,
  FOREIGN KEY (item_id) REFERENCES merchant_items(id) ON DELETE RESTRICT
);

CREATE INDEX idx_food_order_items_order ON food_order_items(order_id);
CREATE INDEX idx_food_order_items_options ON food_order_items USING GIN (options);

ALTER TABLE food_order_items ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Customer can view own order items" ON food_order_items FOR SELECT USING (EXISTS (SELECT 1 FROM food_orders WHERE id = order_id AND customer_id = auth.uid()));
CREATE POLICY "Merchant can view order items for own store" ON food_order_items FOR SELECT USING (EXISTS (SELECT 1 FROM food_orders fo JOIN food_merchants fm ON fm.id = fo.merchant_id WHERE fo.id = order_id AND fm.user_id = auth.uid()));
CREATE POLICY "Driver can view order items for assigned orders" ON food_order_items FOR SELECT USING (EXISTS (SELECT 1 FROM food_orders WHERE id = order_id AND driver_id = auth.uid()));
CREATE POLICY "Customer can insert own order items" ON food_order_items FOR INSERT WITH CHECK (EXISTS (SELECT 1 FROM food_orders WHERE id = order_id AND customer_id = auth.uid()));

-- ============================================================================
-- SEND_ORDERS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS send_orders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sender_id UUID NOT NULL,
  driver_id UUID,
  sender_wallet_id UUID,
  driver_wallet_id UUID,
  package_weight_kg NUMERIC(6,2),
  package_dimensions_cm TEXT,
  package_description TEXT,
  pickup_lat NUMERIC(10,8) NOT NULL,
  pickup_lng NUMERIC(11,8) NOT NULL,
  pickup_address TEXT NOT NULL,
  base_fare NUMERIC(12,2) NOT NULL,
  distance_km NUMERIC(8,3),
  weight_surcharge NUMERIC(10,2) DEFAULT 0,
  total_fare NUMERIC(12,2) NOT NULL,
  declared_value NUMERIC(12,2) DEFAULT 0,
  package_type package_type_enum DEFAULT 'STANDARD',
  insurance_fee NUMERIC(10,2) DEFAULT 0,
  discount_amount NUMERIC(10,2) DEFAULT 0,
  voucher_id UUID,
  payment_method payment_method_enum DEFAULT 'WALLET',
  platform_commission NUMERIC(10,2),
  driver_earning NUMERIC(12,2),
  status send_order_status_enum DEFAULT 'CREATED',
  delivery_photo_url TEXT,
  recipient_signature BYTEA,
  created_at TIMESTAMP DEFAULT NOW(),
  assigned_at TIMESTAMP,
  pickup_at TIMESTAMP,
  delivered_at TIMESTAMP,
  settled_at TIMESTAMP,
  is_settled BOOLEAN DEFAULT FALSE,
  CONSTRAINT fare_positive CHECK (total_fare > 0),
  CONSTRAINT discount_non_negative CHECK (discount_amount >= 0),
  CONSTRAINT declared_value_non_negative CHECK (declared_value >= 0),
  CONSTRAINT insurance_fee_non_negative CHECK (insurance_fee >= 0),
  FOREIGN KEY (sender_id) REFERENCES users(id),
  FOREIGN KEY (driver_id) REFERENCES users(id),
  FOREIGN KEY (sender_wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (driver_wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (voucher_id) REFERENCES vouchers(id) ON DELETE SET NULL
);

CREATE INDEX idx_send_orders_sender ON send_orders(sender_id);
CREATE INDEX idx_send_orders_driver ON send_orders(driver_id);
CREATE INDEX idx_send_orders_status ON send_orders(status);
CREATE INDEX idx_send_orders_created ON send_orders(created_at DESC);
CREATE INDEX idx_send_orders_settled ON send_orders(is_settled);
CREATE INDEX idx_send_orders_driver_active ON send_orders(driver_id, status) WHERE status IN ('DRIVER_ASSIGNED', 'PICKED_UP');
CREATE INDEX idx_send_orders_payment ON send_orders(payment_method);

ALTER TABLE send_orders ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Users can view own send orders" ON send_orders FOR SELECT USING (sender_id = auth.uid() OR driver_id = auth.uid());
CREATE POLICY "Users can insert send orders" ON send_orders FOR INSERT WITH CHECK (sender_id = auth.uid());
CREATE POLICY "Users can update own send orders" ON send_orders FOR UPDATE USING (sender_id = auth.uid() OR driver_id = auth.uid());

CREATE TRIGGER send_order_rollback_voucher AFTER UPDATE OF status ON send_orders FOR EACH ROW EXECUTE FUNCTION rollback_voucher_soft_delete();
CREATE TRIGGER send_order_cash_settlement AFTER UPDATE OF status ON send_orders FOR EACH ROW EXECUTE FUNCTION process_cash_settlement();

-- ============================================================================
-- SEND_ORDER_STOPS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS send_order_stops (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id UUID NOT NULL,
  stop_number INTEGER NOT NULL,
  recipient_name VARCHAR(255),
  recipient_phone VARCHAR(20),
  dropoff_lat NUMERIC(10,8) NOT NULL,
  dropoff_lng NUMERIC(11,8) NOT NULL,
  dropoff_address TEXT NOT NULL,
  distance_km NUMERIC(8,3),
  allocated_fare NUMERIC(12,2),
  status VARCHAR(50) DEFAULT 'PENDING',
  delivery_photo_url TEXT,
  recipient_signature BYTEA,
  arrived_at TIMESTAMP,
  completed_at TIMESTAMP,
  notes TEXT,
  CONSTRAINT stop_number_positive CHECK (stop_number > 0),
  CONSTRAINT status_valid CHECK (status IN ('PENDING', 'ARRIVED', 'COMPLETED', 'SKIPPED')),
  CONSTRAINT allocated_fare_non_negative CHECK (allocated_fare >= 0),
  FOREIGN KEY (order_id) REFERENCES send_orders(id) ON DELETE CASCADE,
  UNIQUE(order_id, stop_number)
);

CREATE INDEX idx_send_stops_order ON send_order_stops(order_id);
CREATE INDEX idx_send_stops_status ON send_order_stops(status);
CREATE INDEX idx_send_stops_created ON send_order_stops(completed_at DESC);

ALTER TABLE send_order_stops ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Sender can view own stops" ON send_order_stops FOR SELECT USING (EXISTS (SELECT 1 FROM send_orders WHERE id = order_id AND sender_id = auth.uid()));
CREATE POLICY "Driver can view assigned stops" ON send_order_stops FOR SELECT USING (EXISTS (SELECT 1 FROM send_orders WHERE id = order_id AND driver_id = auth.uid()));
CREATE POLICY "Sender can insert stops" ON send_order_stops FOR INSERT WITH CHECK (EXISTS (SELECT 1 FROM send_orders WHERE id = order_id AND sender_id = auth.uid()));
CREATE POLICY "Sender can update own stops" ON send_order_stops FOR UPDATE USING (EXISTS (SELECT 1 FROM send_orders WHERE id = order_id AND sender_id = auth.uid()));
CREATE POLICY "Driver can update assigned stops" ON send_order_stops FOR UPDATE USING (EXISTS (SELECT 1 FROM send_orders WHERE id = order_id AND driver_id = auth.uid()));

-- ============================================================================
-- SEND_ORDER_EVENTS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS send_order_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id UUID NOT NULL,
  from_status send_order_status_enum,
  to_status send_order_status_enum NOT NULL,
  reason TEXT,
  triggered_by UUID,
  created_at TIMESTAMP DEFAULT NOW(),
  metadata JSONB,
  FOREIGN KEY (order_id) REFERENCES send_orders(id) ON DELETE CASCADE,
  FOREIGN KEY (triggered_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_send_events_order ON send_order_events(order_id);
CREATE INDEX idx_send_events_created ON send_order_events(created_at DESC);

ALTER TABLE send_order_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY "System can insert send events" ON send_order_events FOR INSERT WITH CHECK (TRUE);

-- ============================================================================
-- ADMIN_ACTIONS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS admin_actions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_id UUID NOT NULL,
  action_type VARCHAR(100),
  target_user_id UUID,
  target_resource_type VARCHAR(50),
  target_resource_id UUID,
  before_state JSONB,
  after_state JSONB,
  reason TEXT,
  ip_address INET,
  created_at TIMESTAMP DEFAULT NOW(),
  CONSTRAINT action_type_valid CHECK (action_type IN ('SUSPEND_USER', 'FREEZE_WALLET', 'UNFREEZE_WALLET', 'REFUND_ORDER', 'REVERSE_TRANSACTION', 'ADJUST_BALANCE', 'VERIFY_MERCHANT', 'CLOSE_ORDER')),
  FOREIGN KEY (admin_id) REFERENCES users(id),
  FOREIGN KEY (target_user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_admin_actions_admin ON admin_actions(admin_id);
CREATE INDEX idx_admin_actions_created ON admin_actions(created_at DESC);
CREATE INDEX idx_admin_actions_target ON admin_actions(target_resource_type, target_resource_id);

ALTER TABLE admin_actions ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Admins can view all actions" ON admin_actions FOR ALL USING ((auth.jwt() ->> 'user_type') = 'admin');

-- ============================================================================
-- RATINGS_REVIEWS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS ratings_reviews (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  rater_id UUID NOT NULL,
  ratee_id UUID NOT NULL,
  rating_type rating_type_enum,
  reference_order_id UUID,
  stars INTEGER NOT NULL,
  review_text TEXT,
  created_at TIMESTAMP DEFAULT NOW(),
  CONSTRAINT stars_valid CHECK (stars >= 1 AND stars <= 5),
  CONSTRAINT unique_order_rating UNIQUE (rater_id, reference_order_id, rating_type),
  FOREIGN KEY (rater_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (ratee_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_ratings_ratee ON ratings_reviews(ratee_id);
CREATE INDEX idx_ratings_type ON ratings_reviews(rating_type);
CREATE INDEX idx_ratings_created ON ratings_reviews(created_at DESC);

ALTER TABLE ratings_reviews ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Anyone can view ratings" ON ratings_reviews FOR SELECT USING (TRUE);
CREATE POLICY "Users can insert own ratings" ON ratings_reviews FOR INSERT WITH CHECK (rater_id = auth.uid());

CREATE OR REPLACE FUNCTION update_merchant_rating() RETURNS TRIGGER AS $$
DECLARE avg_rating_val NUMERIC(3,2); total_reviews_val INTEGER;
BEGIN
  IF NEW.rating_type = 'FOOD_MERCHANT' THEN
    SELECT AVG(stars)::NUMERIC(3,2), COUNT(*) INTO avg_rating_val, total_reviews_val FROM ratings_reviews WHERE ratee_id = NEW.ratee_id AND rating_type = 'FOOD_MERCHANT';
    UPDATE food_merchants SET avg_rating = COALESCE(avg_rating_val, 5.0), total_reviews = COALESCE(total_reviews_val, 0), updated_at = NOW() WHERE user_id = NEW.ratee_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER merchant_rating_update AFTER INSERT OR UPDATE OF stars ON ratings_reviews FOR EACH ROW EXECUTE FUNCTION update_merchant_rating();

-- ============================================================================
-- VIEWS
-- ============================================================================

CREATE OR REPLACE VIEW ledger_balance_check AS
SELECT 
  SUM(CASE WHEN entry_type = 'DEBIT' AND is_reversed = FALSE THEN amount ELSE 0 END) as total_debits,
  SUM(CASE WHEN entry_type = 'CREDIT' AND is_reversed = FALSE THEN amount ELSE 0 END) as total_credits,
  (SUM(CASE WHEN entry_type = 'DEBIT' AND is_reversed = FALSE THEN amount ELSE 0 END) = SUM(CASE WHEN entry_type = 'CREDIT' AND is_reversed = FALSE THEN amount ELSE 0 END)) as is_balanced
FROM ledger_entries;

CREATE MATERIALIZED VIEW wallet_ledger_reconcile AS
SELECT 
  w.id as wallet_id,
  w.balance as wallet_balance,
  COALESCE(SUM(CASE WHEN le.entry_type = 'CREDIT' AND le.is_reversed = FALSE THEN le.amount ELSE 0 END), 0) -
  COALESCE(SUM(CASE WHEN le.entry_type = 'DEBIT' AND le.is_reversed = FALSE THEN le.amount ELSE 0 END), 0) as ledger_balance,
  (w.balance = (COALESCE(SUM(CASE WHEN le.entry_type = 'CREDIT' AND le.is_reversed = FALSE THEN le.amount ELSE 0 END), 0) - COALESCE(SUM(CASE WHEN le.entry_type = 'DEBIT' AND le.is_reversed = FALSE THEN le.amount ELSE 0 END), 0))) as is_reconciled
FROM wallets w
LEFT JOIN ledger_entries le ON w.id = le.wallet_id
GROUP BY w.id, w.balance;

CREATE INDEX idx_reconcile_wallet ON wallet_ledger_reconcile(wallet_id);