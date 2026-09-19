-- MIGRATION 008: Perbaiki idempotency_cache (TD-002)
-- Masalah: idempotency_cache.key adalah PRIMARY KEY sehingga key yang sama
-- dilarang untuk seluruh user (global). Dengan idempotency scope per-user
-- (Redis L1 & service idempotency_cache sudah memakai user_id sebagai scope),
-- key seharusnya UNIK per (key, user_id), bukan global.
--
-- Perbaikan:
--   1. Hapus PRIMARY KEY constraint dari key.
--   2. Tambahkan UNIQUE constraint (key, user_id).
--   3. Pastikan index idx_idempotency_expires tetap ada (untuk purge worker
--      yang menghapus baris expires_at < NOW()).

-- 1) Hapus PRIMARY KEY dari key.
ALTER TABLE idempotency_cache DROP CONSTRAINT idempotency_cache_pkey;

-- 2) UNIQUE (key, user_id) — idempotency key hanya harus unik per user.
ALTER TABLE idempotency_cache ADD CONSTRAINT uq_idempotency_key_user UNIQUE (key, user_id);

-- 3) Index purge worker — pastikan tetap ada (idempoten bila sudah dibuat).
CREATE INDEX IF NOT EXISTS idx_idempotency_expires ON idempotency_cache(expires_at);

-- END OF MIGRATION 008
