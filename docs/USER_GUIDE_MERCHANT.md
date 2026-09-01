# User Guide — Merchant (G-Flow)

Panduan penggunaan aplikasi mobile **G-Flow Merchant** untuk pemilik usaha makanan.
Sumber wireframe: [`docs/HALAMAN.txt`](./HALAMAN.txt).

---

## 1. Onboarding & Verifikasi

1. **Registrasi** sebagai **Merchant** → buat akun.
2. Lengkapi **profil toko** (nama, alamat, jam operasional, kategori, foto banner).
3. Status toko = `PENDING_VERIFICATION` sampai admin **memverifikasi dokumen**.
4. Setelah disetujui, status = `ACTIVE` dan toko tampil untuk customer.

> Endpoint: POST `/merchants/register`, GET/PATCH `/merchants/{id}`.

---

## 2. Manajemen Menu (Katalog)

Kelola di menu **Catalog Management**:

- **Section / Menu**: buat grup menu (mis. "Nasi & Mie", "Minuman").
- **Item**: tambah/edit item dengan harga, deskripsi, gambar, status *available*.
- **Multi-Group Varian** (opsional): tambahkan beberapa grup varian pada satu item,
  misalnya grup "Level Pedas" + grup "Ukuran", masing-masing dengan penyesuaian harga:
  - selection_type `SINGLE` (radio) atau `MULTIPLE` (checkbox).
- **Availability**: toggle untuk menampilkan/menyembunyikan item.

> Endpoint: POST `/merchants/{id}/menus`, PATCH/DELETE menu, POST/PATCH/DELETE
> `/merchants/{id}/items`.

---

## 3. Manajemen Order

- **Incoming Orders**: pesanan masuk muncul di dashboard (real-time notifikasi).
- **Order Details**: lihat item, catatan, alamat, total.
- **Update Status** (alur wajib):
  - `CONFIRMED` → `PREPARING` → `READY`
- **Rejected Orders**: tolak pesanan yang tidak bisa dipenuhi (dengan alasan).
- **Order History**: riwayat pesanan yang sudah diproses.

> Endpoint: GET/PATCH `/food-orders/{id}`.

---

## 4. Analitik & Penghasilan

- **Daily Sales**: penjualan hari ini.
- **Revenue Report**: laporan pendapatan bulanan.
- **Top Items**: item terlaris.
- **Withdrawal**: tarik saldo ke rekening bank (role merchant).

> Settlement **4-way** (zero-discrepancy): pembayaran customer didistribusikan ke
> merchant, driver, dan platform secara otomatis setelah order selesai.

---

## 5. Profil Toko

- Perbarui **info toko** (lokasi, jam, kategori).
- Kelola **pengaturan** & metode pembayaran.

---

## 6. Tips

- Pastikan menu & harga selalu up-to-date; saat harga berubah, aplikasi customer
  menampilkan **peringatan version mismatch** sehingga tidak ada harga kadaluarsa.
- Pantau notifikasi order real-time agar tidak melewatkan pesanan.
