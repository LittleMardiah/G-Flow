# User Guide — Customer (G-Flow)

Panduan penggunaan aplikasi mobile **G-Flow** untuk pelanggan (customer).
Sumber wireframe: [`docs/HALAMAN.txt`](./HALAMAN.txt).

---

## 1. Mulai

1. **Install** aplikasi customer_app (Flutter).
2. **Registrasi / Login** — buat akun atau masuk dengan email + password.
3. **Lengkapi profil** setelah verifikasi OTP.

---

## 2. Wallet (PayPulse)

- **Cek saldo** di tab **Wallet**: saldo tersedia ditampilkan besar di atas.
- **Top-Up**:
  1. Masukkan nominal (atau pilih quick: Rp50K/100K/200K/500K/1M).
  2. Pilih metode (Simulasi Internal untuk demo).
  3. Klik **Top-Up Now** — top-up instan, tanpa biaya.
- **Riwayat transaksi**: tab Wallet → lihat daftar transaksi terakhir (top-up, pembayaran).
- Status akun (Verified/KYC, limit) tampil di bagian Wallet.

> Detail: POST `/wallets/{id}/topup` (idempotent), GET `/wallets/{id}/balance`.

---

## 3. Ride (G-Ride)

1. Buka tab **Ride** → **Book a Ride**.
2. Pilih **pickup** (auto lokasi saat ini) & **dropoff**.
3. Pilih tipe (Economy / Premium), lihat **estimasi tarif** (jarak × rate).
4. Klik **Confirm Booking** → escrow dipotong dari wallet.
5. **Tracking**: lihat status (SEARCHING_DRIVER → DRIVER_ASSIGNED → DRIVER_ARRIVED →
   TRIP_STARTED → COMPLETED).
6. Setelah selesai, **beri rating** ke driver.

> Detail: POST `/rides/book`, GET `/rides/{id}`, PATCH `/rides/{id}/status`.

---

## 4. Food (G-Food)

1. Buka tab **Food** → pilih merchant (map/list) atau cari.
2. Buka **menu merchant**, pilih item & varian (level pedas, topping, dsb).
3. Tambahkan ke **Cart** → atur qty/catatan.
4. **Checkout**: pilih metode bayar, konfirmasi → order dibuat.
5. **Tracking**: pantau status (CONFIRMED → PREPARING → READY → DELIVERED).

> Detail: GET `/merchants`, GET `/merchants/{id}/items`, POST `/food-orders`.

---

## 5. Send (G-Send)

Pengiriman paket **multi-stop** (beberapa tujuan dalam satu order):

1. Buka tab **Send** → **Create Send**.
2. Isi detail paket (nama, berat, tipe).
3. Atur **pickup** & **delivery stops** (tambah beberapa tujuan).
4. Lihat **fare breakdown** per stop + total, pilih metode bayar.
5. **Confirm** → order dikirim.
6. **Tracking**: lihat status tiap stop (PICKED_UP → IN_TRANSIT → DELIVERED).
   - Jika penerima tidak ada → status `RETURN_REQUIRED` → driver kembali ke pengirim
     (`RETURNED_TO_SENDER`), refund stop dilewati.

> Detail: POST `/send-orders`, GET `/send-orders/{id}`.

---

## 6. Akun & Pengaturan

- Edit data pribadi, alamat tersimpan, metode pembayaran.
- **Hak untuk dilupakan (UU PDP)**: hapus akun — hanya bisa jika tidak ada order aktif
  & escrow pending.

---

## 7. Bantuan

- Hubungi *Help & Support* di aplikasi untuk keluhan/pertanyaan.
- FAQ umum: proses pembatalan ride, refund stop G-Send, status verifikasi.
