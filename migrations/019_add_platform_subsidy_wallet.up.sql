-- ============================================================================
-- MIGRATION 019: Add SYSTEM_PLATFORM_SUBSIDY system wallet (TD-132)
-- ============================================================================
-- Version : 019
-- Phase   : 2 (Ride Hailing / G-Ride) — schema fix
-- Doc ref : TECHNICAL DEBT.txt — TD-132
--
-- Latar belakang:
--   LOGIC FLOW.txt:351 (FARE ESCROW SHORTFALL & SURPLUS §2.2) menulis:
--
--     IF actual_fare > estimated_fare:
--       -> DEBIT customer by delta (actual - estimated) -> ESCROW
--       -> IF customer balance insufficient:
--          -> System covers delta from SYSTEM_PLATFORM_SUBSIDY
--          -> Record as OVERDUE_DEBT on customer
--
--   Tapi wallet_type_enum (migrations/001_initial_schema.up.sql:48-49)
--   hanya punya CUSTOMER / DRIVER / MERCHANT / SYSTEM_ESCROW /
--   SYSTEM_PLATFORM / SYSTEM_BANK_GATEWAY, dan migration lanjutan hanya
--   menambah SYSTEM_RECEIVABLE_OVERDRAFT (migrations/010:42-52). Tidak ada
--   SYSTEM_PLATFORM_SUBSIDY di DB maupun di kode.
--
--   Akibatnya collectShortfallDelta (internal/ride/service.go, TD-069)
--   memakai wallet SYSTEM_PLATFORM sebagai sumber dana subsidi shortfall
--   (fallback sementara). Dampak: saldo subsidi tercampur dengan komisi
--   platform — rekonsiliasi "komisi platform" vs "subsidi shortfall" mustahil
--   dari sisi saldo wallet, dan audit trail dana subsidi tidak transparan.
--
-- Fix:
--   1. Tambah nilai enum 'SYSTEM_PLATFORM_SUBSIDY' ke wallet_type_enum.
--   2. Seed system user + system wallet (user_id NULL) untuk tipe tersebut,
--      mengikuti pola deterministik migration 010 (SYSTEM_RECEIVABLE_OVERDRAFT
--      memakai ...0004; wallet ini memakai ...0005).
--   3. Code collectShortfallDelta diarahkan ke wallet ini (lihat
--      internal/ride/service.go — konstanta WalletTypeSystemPlatformSubsidy).
--
-- Konsistensi dengan schema:
--   - idx_system_wallet_unique (001:134-135) UNIQUE (wallet_type)
--     WHERE user_id IS NULL -> hanya boleh ADA SATU system wallet per
--     wallet_type. `ON CONFLICT DO NOTHING` membuat seed idempotent walau
--     wallet sudah ada dengan id lain.
--   - wallets.user_id NULL WAJIB karena repository SystemWalletID
--     (internal/ride/repository.go:276-288) mencari
--     `WHERE wallet_type = $1 AND user_id IS NULL`.
--   - CHECK balance_non_negative (001:125) mengizinkan saldo negatif sampai
--     -1.000.000; wallet subsidi di-DEBIT saat menutup shortfall, jadi
--     ceiling yang sama seperti SYSTEM_PLATFORM berlaku (tidak diubah di sini).
--
-- ⚠️ JANGAN bungkus file ini dengan BEGIN/COMMIT atau jalankan psql dengan
--    --single-transaction / -1. PostgreSQL (termasuk PG 15) menolak pemakaian
--    nilai enum baru di transaksi yang sama dengan ALTER TYPE ADD VALUE:
--      ERROR: unsafe use of new value "..." of enum type wallet_type_enum
--    Semua migration di repo ini berjalan per-statement (psql -f tanpa -1),
--    lihat scripts/migrate.sh & scripts/migrate.bat. Ikuti pola yang sama.
--
-- Idempotent: aman di-re-run (ADD VALUE IF NOT EXISTS + ON CONFLICT DO NOTHING).
--
-- PATCH: psql -h localhost -p 15432 -U postgres -d g_flow_dev \
--          -f 019_add_platform_subsidy_wallet.up.sql
--
-- Sanity check:
--   \dT wallet_type_enum        -- harus ada SYSTEM_PLATFORM_SUBSIDY
--   SELECT id, wallet_type, status FROM wallets
--     WHERE wallet_type = 'SYSTEM_PLATFORM_SUBSIDY';
-- ============================================================================

-- 1) Perluasan enum wallet_type_enum.
ALTER TYPE wallet_type_enum ADD VALUE IF NOT EXISTS 'SYSTEM_PLATFORM_SUBSIDY';

-- 2) Seed system user untuk wallet subsidi (email unik, user_type 'system',
--    password_hash sentinel 'SYSTEM_USER' — akun sistem tidak login).
INSERT INTO users (id, email, name, user_type, status, password_hash, is_online) VALUES
  ('00000000-0000-0000-0000-000000000005', 'system.subsidy@g-flow.system', 'SYSTEM_PLATFORM_SUBSIDY', 'system', 'ACTIVE', 'SYSTEM_USER', FALSE)
ON CONFLICT (email) DO NOTHING;

-- 3) Seed system wallet (user_id NULL agar lolos filter SystemWalletID dan
--    unik per wallet_type sesuai idx_system_wallet_unique).
INSERT INTO wallets (id, user_id, wallet_type, balance, status) VALUES
  ('00000000-0000-0000-0000-000000000005', NULL, 'SYSTEM_PLATFORM_SUBSIDY', 0, 'ACTIVE')
ON CONFLICT DO NOTHING;

-- END OF MIGRATION 019.
