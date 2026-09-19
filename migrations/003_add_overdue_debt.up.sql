-- MIGRATION 003: Overdue debt column (Bug #47)
-- Menambahkan kolom overdue_debt ke tabel users untuk mencatat hutang
-- customer (misal: shortfall pembayaran ride) yang otomatis dilunasi dari
-- top-up berikutnya (LOGIC_FLOW 1.2, Bug #47).
--
-- Semantik:
--   - overdue_debt = 0          -> tidak ada hutang
--   - overdue_debt > 0          -> hutang yang akan didahulukan dari top-up
--   - overdue_debt >= 0 selalu  -> tidak pernah negatif (CHECK constraint)

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS overdue_debt NUMERIC(15,2) NOT NULL DEFAULT 0;

-- Hutang tidak boleh negatif.
ALTER TABLE users
  ADD CONSTRAINT overdue_debt_non_negative CHECK (overdue_debt >= 0);

-- END OF MIGRATION 003
