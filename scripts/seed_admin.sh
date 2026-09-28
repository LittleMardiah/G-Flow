#!/usr/bin/env bash
# ============================================================================
# scripts/seed_admin.sh — Seed akun admin G-Flow dari environment (TD-030)
# ============================================================================
# Replacement path untuk hardcoded password di migrations/014_seed_admin.up.sql.
# Password admin TIDAK PERNAH disimpan di repo: dibaca dari env
# ADMIN_PASSWORD, di-hash bcrypt on-the-fly oleh scripts/hashgen, lalu di-upsert
# ke tabel users.
#
# kenapa script, bukan migration:
#   Migration bersifat historis & immutable (sudah pernah jalan di semua env).
#   Password adalah rahasia per-environment, bukan bagian dari schema. Menghapus
#   seed dari migration tanpa menyediakan jalur env-driven = admin tidak bisa
#   dibuat sama sekali, karena role 'admin' TIDAK bisa didaftarkan lewat
#   public API (internal/auth/handler.go:30 allowedUserTypes hanya
#   customer/driver/merchant).
#
# PEMAKAIAN (production / staging):
#   ADMIN_PASSWORD='<strong-password>' bash scripts/seed_admin.sh
#   ADMIN_EMAIL='ops@g-flow.id' ADMIN_PASSWORD='<strong>' bash scripts/seed_admin.sh
#
# Rotasi password admin: jalankan ulang script dengan ADMIN_PASSWORD baru.
# Script bersifat upsert, jadi aman dipanggil berulang.
#
# ENV:
#   ADMIN_EMAIL     Email admin. Default: admin@g-flow.local
#   ADMIN_PASSWORD  WAJIB. Kosong/tidak di-set -> fail-fast, exit 1.
#                   Tidak ada password default yang bisa "terlupa" (that's the point).
#   ADMIN_NAME      Nama tampilan. Default: "Admin G-Flow"
#   DATABASE_URL    Connection string. Kalau kosong, script mencoba baca .env
#                   di root repo (mengikuti pola scripts/migrate.sh).
#
# EXIT CODE: 0 = sukses, 1 = gagal (see pesan di stderr).
# ============================================================================

set -euo pipefail

readonly SCRIPT_NAME="$(basename "$0")"

die() {
  echo "$SCRIPT_NAME: $*" >&2
  exit 1
}

# --- lokasi repo (script dipanggil dari mana saja) -----------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
readonly REPO_ROOT

# --- env dari .env (opsional, hanya kalau variabel belum di-set) ---------------
if [ -f "$REPO_ROOT/.env" ]; then
  while IFS= read -r line || [ -n "$line" ]; do
    case "$line" in
      ''|'#'*) continue ;;
    esac
    key="${line%%=*}"
    val="${line#*=}"
    # buang tanda kutip bila ada
    val="${val%\"}"; val="${val#\"}"
    val="${val%\'}"; val="${val#\'}"
    if [ -n "$key" ] && [ -z "${!key:-}" ]; then
      export "$key=$val"
    fi
  done < "$REPO_ROOT/.env"
fi

# --- DATABASE_URL --------------------------------------------------------------
if [ -z "${DATABASE_URL:-}" ]; then
  die "DATABASE_URL wajib diisi (atau taruh di .env root repo). Lihat .env.example"
fi

# --- ADMIN_PASSWORD: WAJIB, fail-fast -----------------------------------------
# Password default TIDAK ada. Kosong = error, bukan "pakai default".
if [ -z "${ADMIN_PASSWORD:-}" ]; then
  echo "$SCRIPT_NAME: ADMIN_PASSWORD wajib" >&2
  exit 1
fi

readonly ADMIN_EMAIL="${ADMIN_EMAIL:-admin@g-flow.local}"
readonly ADMIN_NAME="${ADMIN_NAME:-Admin G-Flow}"

# Sanity check password sebelum dibakar jadi hash.
if [ "${#ADMIN_PASSWORD}" -lt 8 ]; then
  die "ADMIN_PASSWORD minimal 8 karakter (sekarang ${#ADMIN_PASSWORD})"
fi

# Validasi email LONGGAR di sini supaya pesan error jelas; whitelist penuh tetap
# dijamin CHECK constraint email_valid (migrations/001_initial_schema.up.sql:93).
case "$ADMIN_EMAIL" in
  *@*.*) ;;
  *) die "ADMIN_EMAIL tidak valid: '$ADMIN_EMAIL'" ;;
esac

# --- dependency check ----------------------------------------------------------
command -v psql >/dev/null 2>&1 || die "psql tidak ditemukan. Install postgresql-client."
command -v go   >/dev/null 2>&1 || die "go tidak ditemukan. Diperlukan untuk generate bcrypt hash."

# --- generate bcrypt hash ------------------------------------------------------
# Password dikirim lewat env HASHGEN_PASSWORD, bukan sebagai argumen, supaya tidak
# muncul di process list.
PASSWORD_HASH="$(
  HASHGEN_PASSWORD="$ADMIN_PASSWORD" \
    go run "$SCRIPT_DIR/hashgen" 2>&1
)" || die "gagal generate bcrypt hash (lihat output di atas)"

case "$PASSWORD_HASH" in
  '$2a$'*|'$2b$'*|'$2y$'*) ;;
  *) die "output hashgen bukan bcrypt hash yang valid: '$PASSWORD_HASH'" ;;
esac

# Jangan pernah print password; print hash-nya saja (aman, sudah one-way).
echo "$SCRIPT_NAME: email=$ADMIN_EMAIL  hash=${PASSWORD_HASH:0:7}...  cost=${PASSWORD_HASH:4:2}"

# --- upsert ke tabel users -----------------------------------------------------
# Idempotent: ON CONFLICT (email) DO UPDATE. Dipakai juga untuk rotasi password.
#
# id TIDAK ikut di-insert: primary key pakai DEFAULT gen_random_uuid() sesuai
# konvensi project (migrations/001_initial_schema.up.sql:8 — "Primary keys
# menggunakan UUIDv4 gen_random_uuid()"). Kalau id dipaksa konstan, seed admin
# kedua akan bentrok dengan users_pkey milik admin pertama.
#
# CATATAN wallet: admin TIDAK dibuatkan wallet. Ini konsisten dengan
# migrations/014_seed_admin.up.sql (admin panel hanya baca ledger, reversal,
# freeze user — tidak ada transaksi wallet) dan dengan kode: createWalletsForUser
# hanya dipanggil dari Register, yang menolak user_type='admin'.
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -q \
  -v admin_email="$ADMIN_EMAIL" \
  -v admin_name="$ADMIN_NAME" \
  -v admin_hash="$PASSWORD_HASH" <<'SQL'
INSERT INTO users (
    email, name, user_type, status, password_hash,
    kyc_status, is_online, created_at, updated_at
) VALUES (
    :'admin_email',
    :'admin_name',
    'admin',
    'ACTIVE',
    :'admin_hash',
    'VERIFIED',
    FALSE,
    NOW(),
    NOW()
)
ON CONFLICT (email) DO UPDATE SET
    name           = EXCLUDED.name,
    user_type      = EXCLUDED.user_type,
    status         = EXCLUDED.status,
    password_hash  = EXCLUDED.password_hash,
    kyc_status     = EXCLUDED.kyc_status,
    updated_at     = NOW();
SQL

ADMIN_ID="$(
  # Catatan: interpolasi variabel psql (:'nama') hanya jalan untuk input via
  # stdin/heredoc, BUKAN untuk -c. Karena itu query verifikasi lewat heredoc.
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -tA -v admin_email="$ADMIN_EMAIL" <<'SQL'
SELECT id FROM users WHERE email = :'admin_email';
SQL
)"
[ -n "$ADMIN_ID" ] || die "verifikasi gagal: admin '${ADMIN_EMAIL}' tidak ditemukan setelah upsert"

echo "$SCRIPT_NAME: OK — admin '${ADMIN_EMAIL}' terseed (id=${ADMIN_ID})"
