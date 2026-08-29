-- ============================================================================
-- MIGRATION 002: KYC status, idempotency key VARCHAR, disable RLS (Phase 1 dev)
-- ============================================================================
-- 1. Tambahkan kyc_status ke users (Opsi A untuk KYC limit di service).
-- 2. Ubah idempotency_cache.key dari UUID -> VARCHAR(255) agar fleksibel
--    dengan idempotency key client (API_CONTRACT: string bebas).
-- 3. Disable RLS sementara di idempotency_cache & topup_transactions karena
--    service Phase 1 belum mengatur auth.uid(); akan diaktifkan ulang pada
--    Phase 4 (hardening) dengan mekanisme yang benar.
-- ============================================================================

-- 1) KYC status untuk penentuan limit saldo (TOPUP).
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS kyc_status VARCHAR(20) DEFAULT 'UNVERIFIED';

-- Nilai yang dipakai: 'UNVERIFIED', 'PENDING', 'VERIFIED'.
-- (biarkan tanpa CHECK constraint pada migration ini agar tidak memblokir
--  fase transisi; bisa ditambah di migration hardening)

-- 2) Idempotency key fleksibel (string bebas).
ALTER TABLE idempotency_cache
  ALTER COLUMN key TYPE VARCHAR(255);

-- 3) Disable RLS sementara (Phase 1 development).
ALTER TABLE idempotency_cache DISABLE ROW LEVEL SECURITY;
ALTER TABLE topup_transactions DISABLE ROW LEVEL SECURITY;

-- ============================================================================
-- END OF MIGRATION 002
-- ============================================================================
