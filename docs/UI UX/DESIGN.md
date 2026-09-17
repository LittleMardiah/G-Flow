# G-Flow Marketing Website — Design Specification

## 1. Brand Overview
G-Flow adalah super-app yang menggabungkan ride-hailing, food delivery, kirim paket, dan dompet digital. Target audiens: pengguna urban Indonesia (18–45 tahun) yang mencari kemudahan, kecepatan, dan keamanan.

**Tagline:** "Bayar • Pesan • Kirim — Satu Aplikasi untuk Semua"

**Brand Personality:** Modern, terpercaya, hangat, inspiratif. Bukan terlalu formal (perbankan), bukan terlalu kasual (startup berisik). Posisinya di antara keduanya—premium tapi accessible.

---

## 2. Visual Identity

### Color Palette
| Role | Hex | Usage |
|------|-----|-------|
| Background | `#09090b` | Dark base, seluruh halaman |
| Card / Surface | `#121215` | Kartu, mockup frame |
| Border | `#27272a` | Divider, border kartu, input |
| Text Primary | `#fafafa` | Judul, teks penting |
| Text Secondary | `#a1a1aa` (zinc-400) | Deskripsi, subtext |
| Text Muted | `#71717a` (zinc-500) | Footer, label kecil |
| Accent (Primary CTA) | `#ff9500` | Tombol utama, hover state, badge, link |
| Accent Soft | `rgba(255,149,0,0.08)` | Background ikon layanan |

### Typography
| Role | Font | Weight | Size (Mobile → Desktop) |
|------|------|--------|--------------------------|
| Heading (H1) | Space Grotesk | 700 | 36px → 56px |
| Heading (H2) | Space Grotesk | 700 | 28px → 40px |
| Heading (H3) | Space Grotesk | 600 | 20px → 24px |
| Body | DM Sans | 400 | 16px → 18px |
| Label / Badge | DM Sans | 500 | 11px → 12px |

**Tracking:** Heading pakai `tracking-tight` (-0.02em) untuk kesan premium dan rapat.

### Spacing System (Tailwind-based)
- Container max-width: `1240px`
- Gutter mobile: `1rem (16px)`
- Gutter desktop: `2rem (32px)`
- Section padding: `5rem (80px)` mobile, `7rem (112px)` desktop
- Card padding: `1.5rem (24px)`

---

## 3. Layout & Page Structure (3 Halaman)

### 3.1. Navbar (Global)
**Posisi:** Sticky, fixed di atas.

**Komponen:**
- Kiri: Logo "G-Flow" + badge "Ecosystem" (border tipis)
- Tengah (desktop): 3 link navigasi: Layanan, Mitra, Kontak
- Kanan: CTA "Unduh Aplikasi" (tombol oranye)

**Interaksi:** Navbar transparan dengan backdrop blur (`backdrop-blur-sm`) dan border bawah tipis.

---

### 3.2. Beranda (Hero)
**Layout:** Asimetris 7:5 (kiri copy, kanan mockup staggered).

**Kiri (7 kolom):**
- Badge: "Layanan 24/7 • 2,4 Juta Pengguna" (pill dengan dot hijau)
- Headline: "Bayar • Pesan • Kirim" (baris pertama) + "Satu Aplikasi untuk Semua." (baris kedua, warna zinc-400)
- Subtext: Benefit-driven, maks 2-3 kalimat.
- CTA: Tombol "Mulai Sekarang" (oranye) + "Lihat Layanan" (outline)
- Social Proof: Rating (4.9 ★), jumlah pengguna (2,4 Juta), tingkat keberhasilan (99,9%)

**Kanan (5 kolom):**
- 3 mockup HP staggered:
  - Back-left: rotated -6deg (G-Food)
  - Back-right: rotated +6deg (G-Send)
  - Front-center: dominant, shadow dalam (G-Flow App)
- Efek hover: mockup berotasi ke 0deg (kecuali front-center yang naik sedikit).

**Background:** Ambient glow halus dari zinc-800/20 dan zinc-800/10 (bukan neon).

---

### 3.3. Layanan (Services) — Bento Grid
**Layout:** Bento asimetris (2-1, 1-2), bukan grid 4 kolom kaku.

**Susunan:**
- Baris 1: G-Ride (lebar 2 kolom) + G-Food (1 kolom)
- Baris 2: G-Send (1 kolom) + PayPulse (lebar 2 kolom)

**Setiap kartu layanan:**
- Ikon (lingkaran dengan background oranye soft)
- Label (misal "G-Ride" dengan warna oranye)
- Judul (H3, putih)
- Deskripsi (zinc-400, 1-2 kalimat)
- 3 bullet poin benefit (zinc-400, dengan tanda "•")

**Interaksi:** Hover → border berubah ke oranye/40, kartu naik -4px.

**Sisi Mitra (di bawah bento):**
- 2 kartu: "Driver Partner" dan "Merchant Hub"
- Masing-masing: ikon, judul, deskripsi singkat, link "Daftar Sekarang" (oranye)

---

### 3.4. Kontak (Contact)
**Layout:** 2 kolom (kiri informasi, kanan form).

**Kiri:**
- Headline: "Mari Bangun Ekosistem Bersama G-Flow"
- Deskripsi singkat
- Kontak: email, telepon, alamat (dengan ikon oranye)

**Kanan:**
- Form: Nama, Email/WA, Kategori (dropdown), Pesan (textarea)
- Tombol "Kirim Pengajuan" (oranye)

---

### 3.5. Footer
**Layout:** Minimalis, 2 baris.

**Baris 1 (kiri-kanan):**
- Kiri: Logo "G-Flow" + "© 2025 • Ekosistem Indonesia"
- Kanan: Kebijakan Privasi, Syarat & Ketentuan, **Developer Docs** (link khusus), **GitHub** (link khusus)

**Detail penting:** Link Developer Docs dan GitHub adalah **penanda portofolio engineering**—cukup berupa teks kecil dengan ikon, tidak mengganggu estetika marketing.

---

## 4. Desain Sistem & Interaksi

### 4.1. Komponen Dasar
| Komponen | Spesifikasi |
|----------|-------------|
| **Tombol Primer** | Background `#ff9500`, text `#09090b`, rounded-full, shadow, hover scale/translate |
| **Tombol Sekunder** | Border `#27272a`, text `#a1a1aa`, hover bg `#18181b` |
| **Kartu** | Background `#121215`, border `#27272a`, rounded-2xl, padding 1.5rem, hover border oranye |
| **Form Input** | Background `#18181b`, border `#27272a`, focus border oranye |
| **Badge** | Background `#18181b`, border `#27272a`, text `#a1a1aa`, rounded-full |

### 4.2. Interaksi
| Elemen | Interaksi |
|--------|-----------|
| Navbar link | Hover → text putih |
| CTA utama | Hover → bg lebih gelap, translateY(-1px) |
| Kartu layanan | Hover → border oranye, translateY(-4px) |
| Mockup HP | Hover → rotasi ke 0deg, front-center → translateY(-8px) |
| Link mitra | Hover → text oranye lebih gelap |
| Form input | Focus → border oranye |

### 4.3. Responsif
| Breakpoint | Perubahan |
|------------|-----------|
| **Mobile (< 640px)** | 1 kolom semua, teks lebih kecil, mockup HP diatur ulang (stack) |
| **Tablet (640–1024px)** | Grid 2 kolom, bento tetap asimetris |
| **Desktop (> 1024px)** | Full bento 2-1, 1-2 + mockup staggered |

---

## 5. Anti-AI-Slop Rules (Wajib)

### ❌ FORBIDDEN
- Neon gradients (cyan/purple/pink glow)
- Glassmorphism (`backdrop-blur` berlebihan)
- Floating metric widgets / fake live stats
- Buzzwords: "Revolusioner", "Seamless", "Empower", "Transformasi"
- Tech stack badges (Go, Flutter, Supabase, Redis)

### ✅ REQUIRED
- Layout asimetris / staggered (bukan simetris)
- Tipografi dan whitespace sebagai elemen utama
- Warna monokromatik zinc dengan 1 aksen kuat
- Copywriting benefit-driven (bukan feature list)
- Link developer di footer (GitHub / Docs) sebagai subtle signal

---

## 6. Catatan Implementasi

1. **Mockup HP:** Gunakan placeholder atau screenshot asli di area yang ditandai `[ G-Food Mockup ]`, `[ G-Send Mockup ]`, `[ G-Flow App Mockup ]`.
2. **Ikon:** Gunakan ikon sederhana (SVG atau Unicode) untuk layanan (ride, food, send, wallet).
3. **Form:** Validasi tidak perlu diimplementasikan di prototype.
4. **Navigasi:** Link antar halaman cukup berupa `#` atau anchor ke section yang sama (untuk prototype satu halaman).
5. **Asset:** Tidak perlu gambar latar belakang yang kompleks—cukup gradien halus atau ambient glow sederhana.

---

## 7. Referensi Visual (Vibe)
- **Vercel / Linear.app** → clean typography, dark mode, asimetris
- **Stripe** → premium, trust-building, spacing generous
- **Gojek** → friendly, energetic, tapi dengan sentuhan premium

---

**Status: FINAL — Siap dieksekusi oleh Stitch With Google.**