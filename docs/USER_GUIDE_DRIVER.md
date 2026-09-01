# User Guide — Driver (G-Flow)

Panduan penggunaan aplikasi mobile **G-Flow** untuk driver.
Sumber wireframe: [`docs/HALAMAN.txt`](./HALAMAN.txt).

---

## 1. Daftar & Verifikasi (KYC)

1. **Registrasi** sebagai **Driver** — isi data + bidang kendaraan:
   - Vehicle type (motorcycle / car / van)
   - **Nomor SIM** + tanggal **berlaku SIM** (harus > hari ini)
   - **Plat nomor** kendaraan
2. Setelah mendaftar, status akun = `PENDING_VERIFICATION` sampai KYC disetujui admin.
3. Unggah dokumen KYC (SIM, lisensi/STNK, foto kendaraan) dari menu *Vehicle Management*.

> Syarat: status driver `ACTIVE` + saldo wallet ≥ `min_balance_threshold` untuk menerima order.

---

## 2. Online / Offline

- Gunakan toggle **ONLINE / OFFLINE** di Dashboard.
- Saat ONLINE, driver terlihat oleh sistem untuk menerima order baru (G-Ride, G-Food, G-Send).

---

## 3. Menerima Order (60 detik)

1. Order tersedia muncul di **Daftar Order** (berdasarkan lokasi Anda).
2. Klik **Accept** — Anda punya **60 detik countdown**.
   - Jika tidak di-accept dalam 60 detik, order otomatis dibatalkan/dilepas ke driver lain.
3. **Kapasitas**: maksimal **3 order aktif** sekaligus. Tidak bisa menerima lebih.

> Detail: POST `/rides/{id}/accept`, GET `/drivers/available-orders`, GET `/drivers/{id}/orders`.

---

## 4. Navigasi & Penyelesaian

- Gunakan **map/navigasi** menuju pickup / merchant, lalu ke dropoff / tujuan.
- **G-Ride**: mulai perjalanan (TRIP_STARTED) → selesai (COMPLETED) → sistem settle.
- **G-Food**: ambil pesanan di merchant → antarkan ke pelanggan → perbarui status.
- **G-Send (multi-stop)**: antarkan ke tiap stop berurutan; update status tiap stop
  (DELIVERED / SKIPPED).
  - Jika penerima tidak ada di suatu stop → tandai **RETURN_REQUIRED** → kembali ke
    pengirim (RETURNED_TO_SENDER).
- **Proof of Delivery**: lampirkan foto bila diperlukan.

> Detail: PATCH `/food-orders/{id}`, PATCH `/send-orders/{id}/stops/{stop_id}`,
> PATCH `/send-orders/{id}`.

---

## 5. Penghasilan (Earnings)

- Lihat ringkasan harian/mingguan/bulanan di menu **Earnings**.
- Settlement otomatis masuk ke **wallet** Anda setelah order selesai:
  - G-Ride: 80% tarif (20% platform).
  - G-Food: 4-way settlement (customer/merchant/driver/platform) zero-discrepancy.
  - G-Send: 3-way settlement sesuai alokasi per stop.
- Saldo bisa bertambah saat **auto-sweep** dari overdraft/escrow — Anda akan dapat
  notifikasi.

> Detail: GET ledger via `/ledger/audit` (admin) / endpoint earnings.

---

## 6. Withdrawal

1. Buka Wallet → **Withdraw**.
2. Masukkan nominal & rekening bank.
3. Ajukan → status `PENDING_APPROVAL` → disetujui admin → masuk rekening.

> Butuh role driver/merchant; minimal Rp10.000, maksimal Rp10.000.000 per penarikan.

---

## 7. Kendaraan & Pengaturan

- Kelola info kendaraan (SIM, plat, asuransi).
- Pantau indikator saldo minimal (`min_balance_threshold`) — pastikan cukup untuk tetap
  bisa menerima order.
- Rating performa Anda tampil di profil.
