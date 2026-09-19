-- MIGRATION 001: INITIAL SCHEMA — G-Flow Super-App (Phase 1: Wallet & Auth)
-- Version : 001
-- Phase   : 1 (Wallet & Authentication)
-- Doc ref : DATABASE_SCHEMA v10.4-FINAL, LOGIC_FLOW v6.1-FINAL,
--           TDD v2.0-FINAL, ROADMAP_01 v1.1-FINAL
--
-- DESIGN DECISIONS (disepakati bersama sebelum menulis migration):
--   1. CONVENTION: Primary keys menggunakan UUIDv4 `gen_random_uuid()`.
--      (SESUAI DATABASE_SCHEMA v10.4-FINAL yang LOCKED. Bukan UUIDv7/serial.)
--   2. DEADLOCK LOCK ORDER: Semua operasi multi-wallet (Transfer, Top-Up,
--      Settlement) WAJIB menggunakan `ORDER BY id ASC FOR UPDATE` di layer
--      aplikasi (Go).
--   3. TRIGGER sync_wallet_balance: HANYA mengupdate kolom `balance_after`
--      pada ledger_entries dan `balance` pada wallets/*wallets. Trigger ini
--      TIDAK melakukan `SELECT ... FOR UPDATE` (locking ditangani aplikasi).
--      Ini PENTING agar tidak bentrok dengan lock `ORDER BY id` di aplikasi.
--   4. IDEMPOTENCY: Redis L1 (TTL 5 menit utk status PROCESSING, 24 jam utk
--      COMPLETED) + PostgreSQL L2 (idempotency_cache, unique constraint).
--   5. SYSTEM USERS/WALLETS: Menggunakan UUID fixed deterministik agar mudah
--      direferensikan (dikenali) dari log dan kode:
--        SYSTEM_ESCROW      -> user  ...0001 / wallet ...0001
--        SYSTEM_PLATFORM    -> user  ...0002 / wallet ...0002
--        SYSTEM_BANK_GATEWAY-> user  ...0003 / wallet ...0003
--
-- NOTE: Migration ini hanya mencakup 5 tabel CORE Phase 1 (per ROADMAP 1.1):
--   users, wallets, ledger_entries, idempotency_cache, topup_transactions.
-- Tabel lain (ride/food/send/voucher/dll.) akan dibuat pada migration
-- berikutnya (Phase 2+).

-- EXTENSIONS
-- pg_uuidv7 OPSIONAL (tidak dipakai; kita pakai gen_random_uuid).
-- cube & earthdistance disertakan untuk kompatibilitas penuh schema v10.4
-- (dibutuhkan oleh table driver_locations / spatial index nanti, dan aman
-- untuk dibuat lebih awal).
CREATE EXTENSION IF NOT EXISTS pgcrypto;        -- menyediakan gen_random_uuid()
-- CREATE EXTENSION IF NOT EXISTS pg_uuidv7;     -- OPSIONAL (tidak dipakai di PK)
CREATE EXTENSION IF NOT EXISTS cube;
CREATE EXTENSION IF NOT EXISTS earthdistance;

-- ENUMS (strict type safety — subset yang dibutuhkan Phase 1)
CREATE TYPE user_type_enum AS ENUM
  ('customer', 'driver', 'merchant', 'admin', 'system');

-- SYSTEM_BANK_GATEWAY ditambahkan: diperlukan sebagai ASSET account untuk
-- alur Top-Up (per LOGIC_FLOW 1.2). DATABASE_SCHEMA v10.4 tidak mencantumkannya
-- secara eksplisit di enum, namun LOGIC_FLOW memakainya; ditambahkan di sini
-- sebagai keputusan desain Phase 1 (asset bertambah saat dana masuk).
CREATE TYPE wallet_type_enum AS ENUM
  ('CUSTOMER', 'DRIVER', 'MERCHANT', 'SYSTEM_ESCROW', 'SYSTEM_PLATFORM', 'SYSTEM_BANK_GATEWAY');

CREATE TYPE entry_type_enum AS ENUM ('DEBIT', 'CREDIT');

-- TABLE: users (core identity)
CREATE TABLE IF NOT EXISTS users (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  email             VARCHAR(255) UNIQUE NOT NULL,
  phone             VARCHAR(20),
  name              VARCHAR(255) NOT NULL,

  user_type         user_type_enum NOT NULL,
  status            VARCHAR(50) DEFAULT 'ACTIVE',

  is_online         BOOLEAN DEFAULT FALSE,
  working_status    VARCHAR(20) DEFAULT 'IDLE',
  last_online_at    TIMESTAMP,
  last_status_update_at TIMESTAMP,

  password_hash     VARCHAR(255) NOT NULL,
  last_login_at     TIMESTAMP,

  profile_photo_url TEXT,
  national_id       VARCHAR(50),
  kyc_status        VARCHAR(20) DEFAULT 'UNVERIFIED',  -- UNVERIFIED|PENDING|VERIFIED

  primary_address   TEXT,

  license_number    VARCHAR(50),
  license_expiry    DATE,
  vehicle_type      VARCHAR(50),
  vehicle_plate     VARCHAR(20),

  min_balance_threshold NUMERIC(15,2) DEFAULT 0,

  bank_name         VARCHAR(100),
  bank_account_number VARCHAR(50),
  bank_account_name VARCHAR(255),

  created_at        TIMESTAMP DEFAULT NOW(),
  updated_at        TIMESTAMP DEFAULT NOW(),
  deleted_at        TIMESTAMP,

  CONSTRAINT email_valid CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}$'),
  CONSTRAINT phone_valid CHECK (phone IS NULL OR phone ~* '^\+?[0-9]{10,15}$'),
  CONSTRAINT status_valid CHECK (status IN ('ACTIVE', 'SUSPENDED', 'FROZEN', 'DELETED')),
  CONSTRAINT working_status_valid CHECK (working_status IN ('IDLE', 'BUSY')),
  CONSTRAINT min_balance_non_negative CHECK (min_balance_threshold >= 0),
  CONSTRAINT driver_fields_required CHECK (
    user_type != 'driver' OR
    (vehicle_plate IS NOT NULL AND license_number IS NOT NULL)
  )
);

CREATE INDEX idx_users_email   ON users(email);
CREATE INDEX idx_users_phone   ON users(phone);
CREATE INDEX idx_users_type    ON users(user_type);
CREATE INDEX idx_users_status  ON users(status);
CREATE INDEX idx_users_created ON users(created_at);

-- TABLE: wallets (PayPulse core — single source of truth for balance)
CREATE TABLE IF NOT EXISTS wallets (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  user_id       UUID,
  wallet_type   wallet_type_enum NOT NULL DEFAULT 'CUSTOMER',

  balance       NUMERIC(15,2) NOT NULL DEFAULT 0,
  status        VARCHAR(50) DEFAULT 'ACTIVE',

  currency      VARCHAR(3) DEFAULT 'IDR',
  created_at    TIMESTAMP DEFAULT NOW(),
  updated_at    TIMESTAMP DEFAULT NOW(),

  CONSTRAINT wallet_user_type_unique UNIQUE (user_id, wallet_type),
  CONSTRAINT balance_non_negative CHECK (balance >= -1000000),  -- negatif diizinkan utk cash settlement (ceiling -1jt)
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_wallets_user   ON wallets(user_id);
CREATE INDEX idx_wallets_type   ON wallets(wallet_type);
CREATE INDEX idx_wallets_status ON wallets(status);

-- Hanya satu system wallet per wallet_type (user_id IS NULL)
CREATE UNIQUE INDEX idx_system_wallet_unique ON wallets(wallet_type)
  WHERE user_id IS NULL;

-- TABLE: ledger_entries (double-entry bookkeeping — immutable)
CREATE TABLE IF NOT EXISTS ledger_entries (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  wallet_id       UUID NOT NULL,

  entry_type      entry_type_enum NOT NULL,
  amount          NUMERIC(15,2) NOT NULL,

  reference_type  VARCHAR(100),
  reference_id    UUID,
  description     TEXT,

  created_at      TIMESTAMP DEFAULT NOW(),
  created_by      UUID,

  balance_after   NUMERIC(15,2),

  is_reversed     BOOLEAN DEFAULT FALSE,
  reversal_of_id  UUID,

  CONSTRAINT amount_positive CHECK (amount > 0),
  FOREIGN KEY (wallet_id) REFERENCES wallets(id) ON DELETE CASCADE,
  FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
  FOREIGN KEY (reversal_of_id) REFERENCES ledger_entries(id) ON DELETE SET NULL
);

CREATE INDEX idx_ledger_wallet    ON ledger_entries(wallet_id);
CREATE INDEX idx_ledger_reference ON ledger_entries(reference_type, reference_id);
CREATE INDEX idx_ledger_created   ON ledger_entries(created_at DESC);
CREATE INDEX idx_ledger_type      ON ledger_entries(entry_type);
CREATE INDEX idx_ledger_reversal  ON ledger_entries(is_reversed) WHERE is_reversed = TRUE;

-- CONSTRAINT TRIGGER (DEFERRABLE): validasi double-entry di akhir transaksi
-- reference_id WAJIB untuk semua transaksi finansial (kecuali system adjustment).
-- SUM(DEBIT) harus = SUM(CREDIT) per reference_id.
CREATE OR REPLACE FUNCTION validate_ledger_balance()
RETURNS TRIGGER AS $$
DECLARE
  total_debit  NUMERIC(15,2);
  total_credit NUMERIC(15,2);
BEGIN
  -- System adjustment diizinkan tanpa reference_id asalkan tidak finansial
  IF NEW.reference_id IS NULL AND (NEW.reference_type IS NULL OR NEW.reference_type = '') THEN
    RAISE EXCEPTION 'reference_id wajib diisi untuk transaksi finansial (kecuali system adjustment)';
  END IF;

  IF NEW.reference_id IS NULL THEN
    RETURN NEW;
  END IF;

  SELECT
    COALESCE(SUM(CASE WHEN entry_type = 'DEBIT'  AND is_reversed = FALSE THEN amount ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN entry_type = 'CREDIT' AND is_reversed = FALSE THEN amount ELSE 0 END), 0)
  INTO total_debit, total_credit
  FROM ledger_entries
  WHERE reference_id = NEW.reference_id AND is_reversed = FALSE;

  IF total_debit != total_credit THEN
    RAISE EXCEPTION 'Ledger imbalance: DEBIT (%) != CREDIT (%) for reference_id %',
      total_debit, total_credit, NEW.reference_id;
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER ledger_balance_validation
AFTER INSERT ON ledger_entries
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_ledger_balance();

-- IMMUTABILITY: ledger entries tidak bisa dimodifikasi (kecuali is_reversed)
CREATE OR REPLACE FUNCTION raise_immutability_error()
RETURNS TRIGGER AS $$
BEGIN
  IF (TG_OP = 'UPDATE') THEN
    IF (OLD.wallet_id != NEW.wallet_id OR
        OLD.entry_type != NEW.entry_type OR
        OLD.amount != NEW.amount OR
        OLD.reference_type != NEW.reference_type OR
        OLD.reference_id != NEW.reference_id OR
        OLD.description != NEW.description) THEN
      RAISE EXCEPTION 'Ledger entries are immutable except is_reversed flag';
    END IF;
  ELSIF (TG_OP = 'DELETE') THEN
    RAISE EXCEPTION 'Ledger entries cannot be deleted';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ledger_immutable BEFORE UPDATE OR DELETE ON ledger_entries
FOR EACH ROW EXECUTE FUNCTION raise_immutability_error();

-- TRIGGER sync_wallet_balance
-- DESIGN DECISION #3 (PENTING):
--   Trigger ini hanya mengupdate `ledger_entries.balance_after` dan
--   `wallets.balance`. Trigger TIDAK melakukan `SELECT ... FOR UPDATE`
--   pada wallets. Locking wallet ditangani sepenuhnya di layer aplikasi (Go)
--   dengan `ORDER BY id ASC FOR UPDATE` untuk mencegah deadlock.
--
--   Konsekuensi: kode aplikasi HARUS selalu mengunci semua wallet terkait
--   dalam satu transaksi SEBELUM memasukkan ledger entries. Insert ke
--   ledger_entries di luar transaksi yang sudah mengunci wallet berisiko
--   balapan terhadap `UPDATE wallets SET balance = balance +/- ...` di sini.
--
--   UPDATE is_reversed=true -> reversal (entry lama di-kompensasi).
CREATE OR REPLACE FUNCTION sync_wallet_balance()
RETURNS TRIGGER AS $$
BEGIN
  IF (TG_OP = 'INSERT') THEN
    -- Aplikasi sudah mengunci wallet via FOR UPDATE; kita langsung update.
    UPDATE wallets
    SET balance = balance + CASE
          WHEN NEW.entry_type = 'CREDIT' THEN NEW.amount
          ELSE -NEW.amount
        END,
        updated_at = NOW()
    WHERE id = NEW.wallet_id
    RETURNING balance INTO NEW.balance_after;

    RETURN NEW;

  ELSIF (TG_OP = 'UPDATE') THEN
    -- Reversal: hanya mengembalikan balance ke nilai sebelum entry,
    -- hanya jika is_reversed berubah dari FALSE menjadi TRUE.
    IF (NEW.is_reversed = TRUE AND OLD.is_reversed = FALSE) THEN
      UPDATE wallets
      SET balance = balance - CASE
            WHEN OLD.entry_type = 'CREDIT' THEN OLD.amount
            ELSE -OLD.amount
          END,
          updated_at = NOW()
      WHERE id = OLD.wallet_id;
    END IF;
    RETURN NEW;
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ledger_sync_balance
BEFORE INSERT OR UPDATE OF is_reversed ON ledger_entries
FOR EACH ROW EXECUTE FUNCTION sync_wallet_balance();

-- TABLE: idempotency_cache (duplicate prevention — L2 PostgreSQL)
CREATE TABLE IF NOT EXISTS idempotency_cache (
  key           VARCHAR(255) PRIMARY KEY,
  user_id       UUID NOT NULL,

  request_hash  VARCHAR(64),
  request_body  JSONB,

  response_body JSONB NOT NULL,
  status_code   INTEGER NOT NULL,

  state        VARCHAR(20) DEFAULT 'COMPLETED',  -- PROCESSING | COMPLETED
  debounce_at   TIMESTAMP,                         -- ditetapkan saat PROCESSING (5 menit TTL)

  created_at    TIMESTAMP DEFAULT NOW(),
  expires_at    TIMESTAMP NOT NULL,

  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_idempotency_user    ON idempotency_cache(user_id);
CREATE INDEX idx_idempotency_expires ON idempotency_cache(expires_at);
CREATE INDEX idx_idempotency_state   ON idempotency_cache(state) WHERE state = 'PROCESSING';

-- TABLE: topup_transactions (intermediary — PENDING -> COMPLETED/FAILED/EXPIRED)
CREATE TABLE IF NOT EXISTS topup_transactions (
  id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id                UUID NOT NULL,
  wallet_id              UUID NOT NULL,

  amount                 NUMERIC(15,2) NOT NULL,
  fee                    NUMERIC(10,2) DEFAULT 0,
  net_amount             NUMERIC(15,2) NOT NULL,

  payment_method         VARCHAR(50) DEFAULT 'SIMULATED',
  status                 VARCHAR(50) DEFAULT 'PENDING',

  external_transaction_id VARCHAR(255),
  gateway_response       JSONB,

  is_late_settlement     BOOLEAN DEFAULT FALSE,   -- Bug #53/#56: SUCCESS setelah EXPIRED

  requested_at           TIMESTAMP DEFAULT NOW(),
  completed_at           TIMESTAMP,
  expired_at             TIMESTAMP,

  notes                  TEXT,
  ledger_entry_id        UUID,

  CONSTRAINT amount_positive CHECK (amount > 0),
  CONSTRAINT status_valid CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED', 'EXPIRED')),
  CONSTRAINT payment_method_valid CHECK (payment_method IN ('SIMULATED', 'BANK_TRANSFER', 'QRIS', 'CREDIT_CARD')),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (ledger_entry_id) REFERENCES ledger_entries(id) ON DELETE SET NULL
);

CREATE INDEX idx_topup_user    ON topup_transactions(user_id);
CREATE INDEX idx_topup_wallet  ON topup_transactions(wallet_id);
CREATE INDEX idx_topup_status  ON topup_transactions(status);
CREATE INDEX idx_topup_created ON topup_transactions(requested_at DESC);
CREATE INDEX idx_topup_external ON topup_transactions(external_transaction_id);

-- SEED: SYSTEM USERS & SYSTEM WALLETS
-- Menggunakan UUID fixed deterministik:
--   SYSTEM_ESCROW       = ...0001
--   SYSTEM_PLATFORM     = ...0002
--   SYSTEM_BANK_GATEWAY = ...0003
-- password_hash dikosong-kan ("SYSTEM_USER" sentinel) karena akun sistem
-- tidak melakukan login password.
INSERT INTO users (id, email, name, user_type, status, password_hash, is_online) VALUES
  ('00000000-0000-0000-0000-000000000001', 'system.escrow@g-flow.system',     'SYSTEM_ESCROW',       'system', 'ACTIVE', 'SYSTEM_USER', FALSE),
  ('00000000-0000-0000-0000-000000000002', 'system.platform@g-flow.system',   'SYSTEM_PLATFORM',     'system', 'ACTIVE', 'SYSTEM_USER', FALSE),
  ('00000000-0000-0000-0000-000000000003', 'system.bankgateway@g-flow.system','SYSTEM_BANK_GATEWAY', 'system', 'ACTIVE', 'SYSTEM_USER', FALSE)
ON CONFLICT (email) DO NOTHING;

-- NOTE: wallet_type untuk SYSTEM_BANK_GATEWAY diperjelas di bawah.
-- Karena DATABASE_SCHEMA v10.4 hanya mendefinisikan 2 wallet_type system
-- (SYSTEM_ESCROW, SYSTEM_PLATFORM), sedangkan LOGIC_FLOW memakai
-- SYSTEM_BANK_GATEWAY sebagai asset account utk top-up, kita menambahkan
-- enum 'SYSTEM_BANK_GATEWAY' (lihat TOP OF FILE / enum di atas) — lihat
-- keputusan reviewer sebelum menerapkan seed ini.
INSERT INTO wallets (id, user_id, wallet_type, balance, status) VALUES
  ('00000000-0000-0000-0000-000000000001', NULL, 'SYSTEM_ESCROW',       0, 'ACTIVE'),
  ('00000000-0000-0000-0000-000000000002', NULL, 'SYSTEM_PLATFORM',     0, 'ACTIVE'),
  ('00000000-0000-0000-0000-000000000003', NULL, 'SYSTEM_BANK_GATEWAY', 0, 'ACTIVE')
ON CONFLICT DO NOTHING;

-- ROW-LEVEL SECURITY (RLS) — Phase 1 core tables
-- Catatan: Dalam konteks Supabase, auth.uid() & auth.jwt() tersedia.
-- Untuk deployment non-Supabase (pure PostgreSQL + Go), RLS policies ini
-- tetap dibuat agar schema konsisten, namun aplikasi bergantung pada
-- layering RBAC di Go dan koneksi DB dengan role yang aman.
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Users can view own data" ON users
--   FOR SELECT USING (id = auth.uid());
-- CREATE POLICY "Users can update own data" ON users
--   FOR UPDATE USING (id = auth.uid());
-- CREATE POLICY "Admins can view all users" ON users
--   FOR SELECT USING ((auth.jwt() ->> 'user_type') = 'admin');

ALTER TABLE wallets ENABLE ROW LEVEL SECURITY;
-- Include SYSTEM_BANK_GATEWAY utk visibility & admin/system management
-- CREATE POLICY "Users can view own wallet" ON wallets
--   FOR SELECT USING (user_id = auth.uid() OR wallet_type IN ('SYSTEM_ESCROW', 'SYSTEM_PLATFORM', 'SYSTEM_BANK_GATEWAY'));
-- CREATE POLICY "Users can insert own wallet" ON wallets
--   FOR INSERT WITH CHECK (
--     user_id = auth.uid() AND wallet_type IN ('CUSTOMER', 'DRIVER', 'MERCHANT')
--   );
-- CREATE POLICY "Admin can insert system wallets" ON wallets
--   FOR INSERT WITH CHECK (
--     wallet_type IN ('SYSTEM_ESCROW', 'SYSTEM_PLATFORM', 'SYSTEM_BANK_GATEWAY') AND
--     (auth.jwt() ->> 'user_type') IN ('admin', 'system')
--   );
-- CREATE POLICY "Users can update own wallet" ON wallets
--   FOR UPDATE USING (user_id = auth.uid());
-- CREATE POLICY "Admin can update system wallets" ON wallets
--   FOR UPDATE USING ((auth.jwt() ->> 'user_type') IN ('admin', 'system'));

ALTER TABLE ledger_entries ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY "System can manage ledger entries" ON ledger_entries
--   FOR ALL USING ((auth.jwt() ->> 'user_type') IN ('admin', 'system'));

ALTER TABLE idempotency_cache DISABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Users can view own idempotency cache" ON idempotency_cache
--   FOR SELECT USING (user_id = auth.uid());
-- CREATE POLICY "Users can insert own idempotency cache" ON idempotency_cache
--   FOR INSERT WITH CHECK (user_id = auth.uid());

ALTER TABLE topup_transactions DISABLE ROW LEVEL SECURITY;
-- CREATE POLICY "Users can view own topups" ON topup_transactions
--   FOR SELECT USING (user_id = auth.uid());
-- CREATE POLICY "Users can insert topup" ON topup_transactions
--   FOR INSERT WITH CHECK (user_id = auth.uid());

-- POST-MIGRATION SANITY CHECKS (informational — bisa dijalankan manual)
-- SELECT id, email, user_type FROM users WHERE user_type = 'system';
-- SELECT id, user_id, wallet_type, balance FROM wallets ORDER BY id;

-- END OF MIGRATION 001.
