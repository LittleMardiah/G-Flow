-- ============================================================================
-- MIGRATION 014: SEED ADMIN USER (TD-024)
-- ============================================================================
-- Version : 014
-- Task    : STEP 1C — Admin Auth
-- Doc ref : TECHNICAL DEBT.txt — TD-024
--
-- Membuat akun admin default untuk development/testing.
--
-- KREDENSIAL DEFAULT (WAJIB GANTI DI PRODUCTION):
--   Email    : akun admin domain @g-flow.local (lihat INSERT di bawah)
--   Password : AdminP@ssw0rd!2026
--
-- UUID deterministik pola 9xxxxxxxx-... (pola unik, hindari bentrok
-- dengan seed existing 0000.../1000...). Nilai exact ada di INSERT.
--
-- TODO (TD-030): Admin WAJIB ganti password setelah login pertama
--                di production.
-- ============================================================================

BEGIN;

INSERT INTO users (
    id, email, name, user_type, status, password_hash,
    kyc_status, is_online, created_at, updated_at
) VALUES (
    '90000000-0000-0000-0000-000000000001',
    'admin@g-flow.local',
    'Admin G-Flow',
    'admin',
    'ACTIVE',
    '$2a$12$LHVbld8VCwJJ1hINQuPeZOPu7FCeSLUoMdZ8P6bqptlWwi2OUJ4/y',
    'VERIFIED',
    FALSE,
    NOW(),
    NOW()
)
ON CONFLICT (email) DO NOTHING;

-- Admin tidak butuh wallet (admin panel cuma baca ledger, reversal,
-- freeze user — tidak ada transaksi wallet).
-- Konfirmasi: TIDAK INSERT wallet untuk admin.

COMMIT;