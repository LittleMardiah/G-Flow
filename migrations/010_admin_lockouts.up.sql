-- MIGRATION 010: TRANSACTION REVERSAL (Task 4.2.4)
-- Version : 010
-- Task    : 4.2 Transaction Reversal
-- Doc ref : ROADMAP 04 HARDENING DEPLOY.txt — 4.2.4
--
-- Menambahkan:
--   1. Tabel `admin_lockouts` — ritel percobaan 2FA admin yang gagal
--      (failed_attempts, locked_until). Redis (TTL) sebagai cache L1,
--      PostgreSQL sebagai penyimpanan persisten (fallback & audit).
--   2. Tabel `admin_action_logs` — audit trail aksi admin (reversal,
--      clampdown, dst.) untuk keperluan kepatuhan & traceability.
--   3. Perluasan enum `wallet_type_enum` dengan SYSTEM_RECEIVABLE_OVERDRAFT
--      (piutang platform) untuk mencatat shortfall saat clawback reversal
--      melebihi saldo merchant/driver/platform.

-- 1) admin_lockouts
CREATE TABLE IF NOT EXISTS admin_lockouts (
  admin_id        UUID PRIMARY KEY,
  failed_attempts INTEGER DEFAULT 0,
  locked_until    TIMESTAMPTZ,
  created_at      TIMESTAMPTZ DEFAULT NOW(),
  updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- 2) admin_action_logs
CREATE TABLE IF NOT EXISTS admin_action_logs (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_id      UUID NOT NULL,
  action        VARCHAR(100) NOT NULL,
  entity_type   VARCHAR(50),
  entity_id     UUID,
  reference_id  UUID,
  details       JSONB,
  created_at    TIMESTAMPTZ DEFAULT NOW(),
  FOREIGN KEY (admin_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_admin_action_logs_admin ON admin_action_logs(admin_id);
CREATE INDEX idx_admin_action_logs_entity ON admin_action_logs(entity_type, entity_id);
CREATE INDEX idx_admin_action_logs_created ON admin_action_logs(created_at DESC);

-- 3) Perluasan enum wallet untuk SYSTEM_RECEIVABLE_OVERDRAFT
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_type t
    JOIN pg_enum e ON e.enumtypid = t.oid
    WHERE t.typname = 'wallet_type_enum' AND e.enumlabel = 'SYSTEM_RECEIVABLE_OVERDRAFT'
  ) THEN
    ALTER TYPE wallet_type_enum ADD VALUE IF NOT EXISTS 'SYSTEM_RECEIVABLE_OVERDRAFT';
  END IF;
END $$;

-- Seed system user + wallet untuk SYSTEM_RECEIVABLE_OVERDRAFT (piutang).
-- Mengikuti pola seed deterministik migration 001 (SYSTEM...0001/0002/0003),
-- sistem ini memakai id ...0004. Wallet user_id NULL karena wallet sistem.
INSERT INTO users (id, email, name, user_type, status, password_hash, is_online) VALUES
  ('00000000-0000-0000-0000-000000000004', 'system.receivable@g-flow.system', 'SYSTEM_RECEIVABLE_OVERDRAFT', 'system', 'ACTIVE', 'SYSTEM_USER', FALSE)
ON CONFLICT (email) DO NOTHING;

INSERT INTO wallets (id, user_id, wallet_type, balance, status) VALUES
  ('00000000-0000-0000-0000-000000000004', NULL, 'SYSTEM_RECEIVABLE_OVERDRAFT', 0, 'ACTIVE')
ON CONFLICT DO NOTHING;

