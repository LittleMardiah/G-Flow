-- ============================================================================
-- MIGRATION 014: SEED ADMIN USER (TD-024)
-- ============================================================================
-- Version : 014
-- Task    : STEP 1C — Admin Auth
-- Doc ref : TECHNICAL DEBT.txt — TD-024
--
-- Membuat akun admin default untuk development/testing.
--
-- KREDENSIAL DEFAULT: TIDAK LAGI DIDOKUMENTASIKASI DI SINI (TD-030, 2026-09-27).
-- Nilai aslinya pernah ter-publish di repo, jadi sudah dianggap bocor dan
-- DIHAPUS oleh migrations/017_remove_default_admin.up.sql. Statement INSERT di
-- bawah sengaja dibiarkan utuh (migration historis & immutable) supaya lingkungan
-- yang belum menjalankan 017 tetap punya admin untuk dev; yang bocor sudah
-- dihapus, dan untuk environment baru password datang dari env, bukan repo:
--
--   ADMIN_PASSWORD='<strong-password>' bash scripts/seed_admin.sh
--
-- Hanya hash bcrypt (bukan passwordnya) yang masih tertulis di file ini, dan
-- hash itu bukan rahasia — yang dipakai untuk DELETE di migration 017.
-- UUID deterministik pola 9xxxxxxxx-... (pola unik, hindari bentrok
-- dengan seed existing 0000.../1000...). Nilai exact ada di INSERT.
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