-- ============================================================================
-- MIGRATION 017: HAPUS AKUN ADMIN DEFAULT (TD-030)
-- ============================================================================
-- Version : 017
-- Doc ref : TECHNICAL DEBT.txt — TD-030
--
-- Latar belakang:
--   Migration 014 membuat akun admin dengan password hardcoded. Password itu
--   pernah ter-publish di repo (014 + DEPLOYMENT_GUIDE + CI), jadi sudah
--   dianggap bocor: siapa pun yang punya repo bisa login. Migration ini menghapus
--   akun default tersebut.
--
-- Password default TIDAK ditulis di file ini. Yang dicocokkan adalah bcrypt HASH
-- yang sama persis dengan yang di-seed migration 014:32, jadi file ini tidak
-- menambah kebocoran baru.
--
-- SETELAH migration ini:
--   Fresh deploy = TIDAK ADA akun admin. Role 'admin' tidak bisa didaftarkan
--   lewat public API (internal/auth/handler.go:30 hanya menerima
--   customer/driver/merchant), jadi admin WAJIB dibuat lewat:
--     ADMIN_PASSWORD='<strong-password>' bash scripts/seed_admin.sh
--   (lihat docs/DEPLOYMENT_GUIDE.md §11).
--
-- Idempotent: aman di-re-run. Kalau akun sudah dihapus (atau password-nya
-- sudah dirotasi sehingga hash tidak cocok), statement ini tidak-match apa pun
-- dan tidak mengubah apa pun.
--
-- PENTING — akun yang password-nya sudah dirotasi TIDAK ikut terhapus: hash-nya
-- sudah berbeda dari hash di 014:32, jadi kondisi di bawah tidak terpenuhi.
-- Itu memang niatnya: hapus yang bocor saja, jangan sentuh kredensial yang
-- sudah diganti.
--
-- CATATAN AUDIT: tabel admin_action_logs punya FK ke users(id) ON DELETE CASCADE
-- (lihat migrations/010_admin_lockouts.up.sql). Kalau admin default ternyata
-- pernah dipakai (login + aksi), log aksinya ikut terhapus saat migration ini
-- jalan. Itu konsekuensi yang tidak bisa dihindari tanpa mengubah FK, dan
-- memang kredensial yang sudah bocor tidak boleh dipakai lagi. Bila audit trail
-- untuk incident terkait dibutuhkan, backup dulu sebelum menjalankan migration.
-- ============================================================================

BEGIN;

DO $$
DECLARE
    v_deleted integer;
BEGIN
    DELETE FROM users
    WHERE email = 'admin@g-flow.local'
      AND password_hash = '$2a$12$LHVbld8VCwJJ1hINQuPeZOPu7FCeSLUoMdZ8P6bqptlWwi2OUJ4/y';

    GET DIAGNOSTICS v_deleted = ROW_COUNT;

    RAISE NOTICE '017: % baris admin default (hash dari migration 014) dihapus', v_deleted;

    IF v_deleted = 0 THEN
        RAISE NOTICE '017: tidak ada admin default — wajar kalau fresh deploy '
                     'atau password admin sudah dirotasi. Jalankan '
                     'scripts/seed_admin.sh untuk membuat admin.';
    END IF;
END $$;

COMMIT;
