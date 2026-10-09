-- Migration 020: withdrawal_requests table (TD-183)
-- Sync with DATABASE_SCHEMA.txt section 7 withdrawal_requests + API CONTRACT
-- section 6.4 (user request) + 10.4 (admin approve).
-- Enum status: PENDING/PROCESSING/COMPLETED/FAILED/REJECTED (DB source of truth).
-- API DTO maps PENDING -> PENDING_APPROVAL for response readability.
-- Idempotency: handled at application layer (body idempotency_key), not schema.

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

  requested_at TIMESTAMPTZ DEFAULT NOW(),
  processed_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,

  ledger_entry_id UUID,
  admin_notes TEXT,
  rejection_reason TEXT,

  CONSTRAINT amount_positive CHECK (amount > 0),
  CONSTRAINT status_valid CHECK (status IN ('PENDING','PROCESSING','COMPLETED','FAILED','REJECTED')),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (wallet_id) REFERENCES wallets(id),
  FOREIGN KEY (ledger_entry_id) REFERENCES ledger_entries(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_withdrawal_user ON withdrawal_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_withdrawal_wallet ON withdrawal_requests(wallet_id);
CREATE INDEX IF NOT EXISTS idx_withdrawal_status ON withdrawal_requests(status);
CREATE INDEX IF NOT EXISTS idx_withdrawal_created ON withdrawal_requests(requested_at DESC);
