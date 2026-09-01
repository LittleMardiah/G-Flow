# User Guide — Admin (G-Flow)

Panduan penggunaan **Admin Web Panel** G-Flow untuk operator/platform.
Sumber: [`docs/HALAMAN.txt`](./HALAMAN.txt) & [`docs/ROADMAP 04 HARDENING DEPLOY.txt`](./ROADMAP%2004%20HARDENING%20DEPLOY.txt).

---

## 1. Dashboard

- **KPI utama**: Active Orders, Total Transaction Volume, Avg Fare, Revenue Today, Error Rate.
- **Order Status Breakdown** (pie chart): distribusi status order.
- **Error Rate** (line chart): tren error.
- **Latest Transactions** (tabel): transaksi terbaru.

> GET `/admin/dashboard/kpis`, GET `/admin/dashboard/transactions`.

---

## 2. Ledger & Finance

- **Ledger Audit Trail**: tabel semua entri ledger (wallet, entry type, amount, reference, created_at).
  - Filter: rentang tanggal, tipe wallet, tipe entry, pencarian.
  - **Export**: unduh CSV / JSON.
- **Balance Verification**: masukkan wallet ID → cek total debit vs kredit, dan status
  `BALANCED`/discrepancy.
- **Settlement Reports** & **Dispute Management**.

> GET `/admin/ledger`, GET `/admin/ledger/verify/{wallet_id}`, POST `/admin/ledger/export`.

---

## 3. User Management

- **User List**: filter by role & status; kolom (ID, email, role, status, balance, actions).
- **User Detail**: profil lengkap, riwayat transaksi, toggle freeze/suspend/ban.
- **KYC Verification**: verifikasi merchant & driver (dokumen).
- **Blacklist Management**.

> GET `/admin/users`, PATCH `/admin/users/{id}/freeze|suspend|ban`.

---

## 4. Transaction Reversal (2FA)

Membatalkan / membalik transaksi dengan keamanan tinggi:

1. Buka transaksi terkait → **Reverse**.
2. **Harus memasukkan Token 2FA** (header `X-Admin-2FA-Token`).
3. Iisi **reason** & **notes**.
4. Sistem melakukan **Proportional Clawback**:
   - Refund penuh ke customer.
   - Klaim kembali dari Merchant, Driver, dan Platform secara proporsional.
   - Jika dana tidak cukup → shortfall dicatat ke `SYSTEM_RECEIVABLE_OVERDRAFT`.
5. **Auto-Sweep Journal**: DEBIT `RECIPIENT_PAYOUT_ACCOUNT`, CREDIT
   `SYSTEM_RECEIVABLE_OVERDRAFT`, plus notifikasi partial sweep.

**Keamanan 2FA / Lockout:**
- Maksimum **3 percobaan gagal** → **lockout 15 menit** (Redis + PostgreSQL dual-write).
- Check lockout dilakukan via Redis eval; jika Redis gagal → fallback ke PostgreSQL.

> POST `/admin/transactions/{id}/reverse` — status: 401 (2FA invalid), 403, 422, 429 (lockout).

---

## 5. Dispute Management

- Kelola dispute (mis. order tidak sampai, refund).
- **Timeout 30 hari** untuk resolve otomatis; **admin override** butuh 2FA.
- Pantau **overdraft** (shortfall) melalui ledger.

---

## 6. Operasional Harian

- Pantau **saldo sistem** & escrow agar tetap seimbang (ledger zero-discrepancy).
- Verifikasi **withdrawal** (driver/merchant) → status `PENDING_APPROVAL` → `COMPLETED`.
- Tangani **PII / UU PDP**: data user di-anonimkan secara detach (email & phone hash
  bigint-safe) saat penghapusan akun.

---

## Contoh Alur Reversal

```
Admin masuk → Dashboard → Ledger → temukan transaksi
  → klik Reverse → masukkan TOTP 2FA → isi reason
  → sistem: refund customer + clawback merchant/driver/platform
  → shortfall → SYSTEM_RECEIVABLE_OVERDRAFT + auto-sweep journal
  → notifikasi partial sweep dikirim
```
