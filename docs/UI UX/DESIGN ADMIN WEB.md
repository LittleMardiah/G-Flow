# DENTFLOW v10.0 - DESIGN ADMIN WEB.md
## Admin Portal (Desktop-First)

**Versi:** 1.0  
**Tanggal:** 2026-07-07  
**Device:** Web Desktop (Responsive untuk tablet)  
**Target:** Stitch With Google / Figma Hi-Fi Prototype  
**Status:** Sesuai 100% dengan PRD v10.0 & HALAMAN.txt

---

## 📋 TABLE OF CONTENTS

1. [Global Design System](#global-design-system)
2. [Architecture Overview](#architecture-overview)
3. [Admin Portal Design](#admin-portal-design)
4. [Page Details](#page-details)
5. [Shared Components & Patterns](#shared-components--patterns)
6. [Responsive Breakpoints](#responsive-breakpoints)

---

## 🎨 GLOBAL DESIGN SYSTEM

### Color Palette

| Color Name | Hex Code | Usage | Notes |
|---|---|---|---|
| Primary (Deep Teal) | `#0F766E` | Tombol utama, menu aktif, text headings | Memancarkan higiene & trust |
| Primary Light | `#CCFBF1` | Hover states, row highlight, badge | Soft highlight untuk interaksi |
| Alert / Danger | `#D08080` | Status negatif, Low Stock alert | Hindari di text murni (gunakan badge) |
| Neutral Dark (Slate 800) | `#1E293B` | Body text utama, labels | Keterbacaan tinggi |
| Neutral Light (Slate 50) | `#F8FAFC` | Background dasar aplikasi | Minimal eye-strain |
| Surface White | `#FFFFFF` | Background card, modal, tabel | Clean & airy |
| Gray 400 | `#9CA3AF` | Border, disabled state, secondary text | Visual hierarchy |
| Success (Green) | `#10B981` | Status success, badge "Selesai", "Siap" | Psychological positive |
| Info (Blue) | `#3B82F6` | Status info, badge "Draft", "Pending" | Informatif, netral |
| Warning (Amber) | `#F59E0B` | Status warning, "Draft" alternative | Attention-grabbing |

### Typography

```
Heading 1 (H1): 32px, Bold (700), Line-height 1.2
Heading 2 (H2): 24px, Bold (700), Line-height 1.3
Heading 3 (H3): 20px, Semi-bold (600), Line-height 1.4
Body Text (p): 14px, Regular (400), Line-height 1.5
Small Text: 12px, Regular (400), Line-height 1.4
Label (form): 13px, Medium (500), Line-height 1.4
```

**Font Family:** Inter, -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, sans-serif

### Spacing & Layout

- Base grid unit: 4px (multiples: 4px, 8px, 12px, 16px, 20px, 24px, 32px, 40px, 48px)
- Desktop container width: 1440px (max width)
- Sidebar width: 260px (fixed)
- Main content area: Full width minus sidebar
- Card padding: 20px
- Page padding: 24px

### Accessibility Rules

- **WCAG AA Compliance:** Semua warna text harus lolos contrast test
- Alert/Danger (#D08080) untuk text: Jangan gunakan langsung di white background
  - **Solution:** Gunakan sebagai badge background dengan opacity 15% + icon, atau text di dark background
- Focus states: Outline 2px solid `#0F766E` dengan offset 2px
- Interactive elements: Min height 44px untuk mobile accessibility

---

## 🏗️ ARCHITECTURE OVERVIEW

### Main Layout Structure (Web Admin)

```
┌─────────────────────────────────────────────────────────────┐
│ TOP NAVBAR (Fixed, Height: 64px)                            │
│ Logo | Breadcrumb | Search | User Menu (Avatar + Logout)   │
├──────────────┬──────────────────────────────────────────────┤
│              │                                              │
│ LEFT SIDEBAR │ MAIN CONTENT AREA                            │
│ (260px,      │ (Responsive, fullwidth - 260px)             │
│ Fixed)       │                                              │
│              │                                              │
│ Navigation   │ Page Title + Breadcrumb                      │
│ Menu Items   │                                              │
│              │ ┌──────────────────────────────────────────┐ │
│ - Antrian    │ │ PAGE CONTENT                             │ │
│ - Dokter     │ │ (Cards, Tables, Forms, Modals, etc)     │ │
│ - Pasien     │ │                                          │ │
│ - Keuangan   │ │                                          │ │
│ - Inventaris │ │                                          │ │
│ - Audit Log  │ │                                          │ │
│ - Pengaturan │ │                                          │ │
│              │ └──────────────────────────────────────────┘ │
│              │                                              │
└──────────────┴──────────────────────────────────────────────┘
```

### Navigation Strategy

**Left Sidebar (Fixed):**
- Fixed width: 260px
- Background: `#FFFFFF` (white)
- Border-right: 1px solid `#E5E7EB` (gray-200)
- Scroll: Only sidebar content scrolls, always visible
- Display branch info: "Cabang [A/B/C]" (small text di bawah logo)

**Top Navbar (Fixed):**
- Height: 64px
- Background: `#FFFFFF`
- Border-bottom: 1px solid `#E5E7EB`
- Sticky on scroll
- Content: Logo, Breadcrumb, Search, User menu

---

## 👨‍💼 ADMIN PORTAL DESIGN

### Admin Role Menu (Sidebar)

**NOTE: TIDAK ADA DASHBOARD MENU (sesuai PRD v10.0)**

```
┌─────────────────────────────────────┐
│ DENTFLOW ADMIN                      │ (20px padding top)
│ [Logo] Klinik Gigi                  │
├─────────────────────────────────────┤
│ Cabang Medan                        │ (small text, info cabang)
├─────────────────────────────────────┤
│ OPERASIONAL                         │ (section header)
│ [icon] Antrian                      │ (Primary if active: bg #CCFBF1, text #0F766E)
│ [icon] Dokter                       │
│ [icon] Pasien                       │
├─────────────────────────────────────┤
│ KEUANGAN & OPERASI                  │ (section header)
│ [icon] Keuangan                     │
│ [icon] Inventaris                   │
├─────────────────────────────────────┤
│ SISTEM                              │ (section header)
│ [icon] Audit Log                    │
│ [icon] Pengaturan                   │
├─────────────────────────────────────┤
│ [Avatar]                            │
│ Admin Name                          │
│ [Logout button]                     │
└─────────────────────────────────────┘
```

**Menu Items (7 total):**
1. **Antrian** - `/admin/queue` - Live queue monitoring & check-in
2. **Dokter** - `/admin/doctors` - Doctor management & schedule override
3. **Pasien** - `/admin/patients` - Patient list & history
4. **Keuangan** - `/admin/finance` - Invoice management & payment tracking
5. **Inventaris** - `/admin/inventory` - Stock management (admin only)
6. **Audit Log** - `/admin/audit-logs` - System activity logs (immutable)
7. **Pengaturan** - `/admin/settings` - Branch settings, backup, profile

---

## 📄 PAGE DETAILS

### 1. ANTRIAN (QUEUE MANAGEMENT) → `/admin/queue`

**Purpose:** Live queue monitoring & patient check-in management

**Layout:**

```
┌─ Page Header ──────────────────────────────────────┐
│ Antrian > Cabang Medan                             │
│ Real-time updates (setiap 2 detik)                 │
│ [Refresh button] [Export CSV]                      │
└────────────────────────────────────────────────────┘

┌─ Check-in Form (Card) ─────────────────────────────┐
│ FORM CHECK-IN PASIEN                               │
│ ┌────────────────────────────────────────────────┐ │
│ │ Masukkan Kode Booking (6-digit)                │ │
│ │ [Input field: XXXXXX] [Check-in button]        │ │
│ │ ← atau → WALK-IN (untuk pasien tanpa booking)  │ │
│ │ [Pilih Dokter] [Masukkan ke Antrian]           │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘

┌─ Real-time Queue Status ───────────────────────────┐
│ ANTRIAN REAL-TIME (Auto-refresh: 2 detik)          │
│ [Clear Completed Numbers] [Manual Refresh]         │
│                                                    │
│ ┌─ Dr. Budi | Sp.KG | Sesi 09:00-12:00 ──────┐   │
│ │ NOW: 5 (dipanggil) | QUEUE: 1, 2, 3, 4, 6, 7│   │
│ └────────────────────────────────────────────────┘   │
│                                                    │
│ ┌─ Dr. Siti | Sp.BM | Sesi 09:00-12:00 ──────┐   │
│ │ NOW: 3 (dipanggil) | QUEUE: 2, 4, 5, 8     │   │
│ └────────────────────────────────────────────────┘   │
│                                                    │
│ ... (remaining dokter cards)                       │
└────────────────────────────────────────────────────┘

┌─ Queue Summary Table ──────────────────────────────┐
│ RINGKASAN ANTRIAN HARI INI                         │
│ ┌────────────────────────────────────────────────┐ │
│ │ Dokter │ Spesialis │ Total │ Selesai │ Sisa   │ │
│ │─────────────────────────────────────────────────│ │
│ │ Dr. Budi │ Sp.KG  │ 8     │ 2       │ 6      │ │
│ │ Dr. Siti │ Sp.BM  │ 5     │ 1       │ 4      │ │
│ │ ... (other doctors)                            │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘
```

**Components:**

- **Check-in Form Card:**
  - Kode booking input (6-digit alphanumerik)
  - Validation: Check kode format, validasi kode exist, status cek
  - Success: Nomor antrian generated & displayed
  - Walk-in alternative: Pilih dokter directly, tanpa kode
  - Error handling: Toast notification untuk invalid kode

- **Real-time Queue Cards (per dokter):**
  - Doctor info: Name, Specialist, Session time
  - NOW section: Current calling number (large, RED highlight)
  - QUEUE section: List of waiting numbers (BLUE, normal color)
  - Real-time update via polling (2-3 detik) atau WebSocket
  - Auto-delete completed numbers

- **Queue Summary Table:**
  - Columns: Dokter, Spesialis, Total Antrian, Selesai, Sisa
  - Pagination or load more

**Interaction:**
- Enter 6-digit kode → Click "Check-in" → Validate → Generate nomor antrian → Toast success
- Walk-in: Select dokter → Input ke antrian → Nomor generated
- Clear completed numbers → Remove selesai numbers dari display
- Manual refresh → Sync latest queue state

**Logic Flow (Backend Integration):**
1. Admin input kode booking → POST `/api/admin/checkin` dengan { code, admin_id, branch_id }
2. Backend validate:
   - Kode exist? Kode sudah dipakai? Booking status PAYMENT_CONFIRMED? Session timing valid?
3. If valid:
   - Generate nomor antrian via Redis atomic counter
   - Update booking status → CHECKED_IN
   - Create queue entry dengan status WAITING
   - Return nomor antrian
4. If invalid → Return error message
5. Real-time sync via polling GET `/api/admin/queue/status?branch_id=X` setiap 2 detik

---

### 2. DOKTER (DOCTOR MANAGEMENT) → `/admin/doctors`

**Purpose:** Manage doctors & their schedules

**Layout:**

```
┌─ Page Header ──────────────────────────────────────┐
│ Dokter > Cabang Medan                              │
│ Total: 8 dokter                                    │
│ [Filter by Specialist] [Add Doctor] [Export]       │
└────────────────────────────────────────────────────┘

┌─ Doctor List Table ────────────────────────────────┐
│ ┌────────────────────────────────────────────────┐ │
│ │ No │ Nama │ Spesialis │ Jadwal │ Status │ Aksi │ │
│ │─────────────────────────────────────────────────│ │
│ │ 1 │ Dr. Budi │ Sp.KG │ Sen,Rab,Jum │ Aktif │ Edit │
│ │ 2 │ Dr. Siti │ Sp.BM │ Sel,Kam,Sab │ Aktif │ Edit │
│ │ 3 │ Dr. Hasan │ Sp.Ort │ (custom) │ Cuti │ Edit │
│ │ ... (remaining doctors)                        │ │
│ └────────────────────────────────────────────────┘ │
│ [Pagination]                                      │
└────────────────────────────────────────────────────┘

┌─ Doctor Detail Modal (on edit) ────────────────────┐
│ DETAIL DOKTER: Dr. Budi                            │
│ ┌────────────────────────────────────────────────┐ │
│ │ Nama: Dr. Budi Santoso (read-only)             │ │
│ │ Spesialis: Sp.KG (read-only)                   │ │
│ │ No. Lisensi: 123456789 (read-only)             │ │
│ │ Email: budi@klinik.com (read-only)             │ │
│ │ No. Telepon: 08123456789 (editable)            │ │
│ │ Cabang: Cabang Medan (read-only)               │ │
│ │                                                 │ │
│ │ JADWAL KERJA:                                   │ │
│ │ ☑ Senin       ☑ Rabu       ☑ Jumat             │ │
│ │ ☐ Selasa      ☐ Kamis      ☐ Sabtu             │ │
│ │                                                 │ │
│ │ JAM PRAKTEK (override):                         │ │
│ │ [Input: 09:00] - [Input: 17:00]                │ │
│ │ ℹ️ Leave default atau override per hari        │ │
│ │                                                 │ │
│ │ [Simpan Perubahan] [Batal]                     │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘
```

**Components:**

- **Doctor List Table:**
  - Columns: No, Nama, Spesialis, Jadwal, Status, Aksi
  - Filter dropdown: Filter by specialist
  - Search: Search by doctor name
  - Status badge: "Aktif" (green), "Cuti" (amber), "Resign" (red)
  - Action buttons: "Edit" (modal), "View Schedule"

- **Doctor Detail Modal (edit):**
  - Basic info (read-only): Nama, Spesialis, Lisensi, Email, Cabang
  - Editable fields: Nomor telepon, Bio
  - Schedule checkboxes: Senin-Sabtu (default dari hardcoded schedule di PRD)
  - Override time: JAM PRAKTEK custom input (opsional)
  - Validation: Min 1 hari dipilih, jam valid format
  - Save button dengan audit log trigger (SCHEDULE_OVERRIDE)

**Interaction:**
- Click "Edit" → Open modal
- Select/deselect jadwal hari
- Edit jam praktek (opsional)
- Click "Simpan" → Validate → POST `/api/admin/doctors/:doctor_id/schedule` → Toast → Close modal
- Filter by specialist → Reload table

**Logic Flow (Backend Integration):**
1. Load: GET `/api/admin/doctors?branch_id=X` dengan filter & search
2. Display doctor list
3. Click "Edit" → Load doctor detail
4. Edit schedule → Validate → PUT `/api/admin/doctors/:doctor_id/schedule`
5. Backend: Update schedule, log SCHEDULE_OVERRIDE audit event
6. Return success → Toast "Jadwal berhasil diperbarui"

---

### 3. PASIEN (PATIENT MANAGEMENT) → `/admin/patients`

**Purpose:** View patient list, history, & contact info

**Layout:**

```
┌─ Page Header ──────────────────────────────────────┐
│ Pasien > Cabang Medan                              │
│ Total Pasien: 245                                  │
│ [Search] [Filter by Status] [Export CSV]           │
└────────────────────────────────────────────────────┘

┌─ Filter & Search ──────────────────────────────────┐
│ [Search by name/email/phone] [Status: All/Active/Suspended] │
│ [Date Range: from date - to date] [Apply]          │
└────────────────────────────────────────────────────┘

┌─ Patient List Table ───────────────────────────────┐
│ ┌────────────────────────────────────────────────┐ │
│ │ No │ Nama │ Email │ HP │ Status │ Kunjungan │ Aksi │
│ │──────────────────────────────────────────────────│ │
│ │ 1 │ Ahmad │ ahmad@mail.com │ 0812... │ Aktif │ 5 │ View │
│ │ 2 │ Siti │ siti@mail.com │ 0818... │ Aktif │ 2 │ View │
│ │ 3 │ Budi │ budi@mail.com │ 0899... │ Inactive │ 1 │ View │
│ │ ... (more patients)                            │ │
│ └────────────────────────────────────────────────┘ │
│ [Pagination]                                      │
└────────────────────────────────────────────────────┘

┌─ Patient Detail Modal (on click View) ─────────────┐
│ DETAIL PASIEN: Ahmad                               │
│ ┌────────────────────────────────────────────────┐ │
│ │ Informasi Dasar:                                │ │
│ │ Nama: Ahmad Sudibyo                             │ │
│ │ Email: ahmad@mail.com                           │ │
│ │ No. HP: 0812-3456-789                           │ │
│ │ Tanggal Lahir: 15 Januari 1990                  │ │
│ │ Alamat: Jl. Merdeka No. 123, Medan              │ │
│ │ Alergi: Amoxicillin [BADGE: ALERT]             │ │
│ │                                                 │ │
│ │ Riwayat Kunjungan:                              │ │
│ │ • 7 Jul 2026 - Dr. Budi (Sp.KG) - Scaling      │ │
│ │ • 5 Jul 2026 - Dr. Siti (Sp.BM) - Cabut Gigi   │ │
│ │ • 2 Jul 2026 - Dr. Hasan (Sp.Ort) - Konsultasi │ │
│ │                                                 │ │
│ │ [Lihat EMR Lengkap] [Close]                    │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘
```

**Components:**

- **Patient List Table:**
  - Columns: No, Nama, Email, HP, Status, Kunjungan (visit count), Aksi
  - Search: By name, email, phone
  - Filter: Status (All, Aktif, Inactive)
  - Filter: Date range (registration date)
  - Sort: By name, by registration date
  - Pagination

- **Patient Detail Modal:**
  - Basic info: Nama, Email, HP, Tanggal Lahir, Alamat
  - Alergi: Display sebagai badge dengan alert color (`#D08080`)
  - Riwayat Kunjungan: List 5 latest visits (date, doctor, procedure)
  - Link: "Lihat EMR Lengkap" → Navigate to EMR detail page
  - Close button

**Interaction:**
- Search/filter → Reload table
- Click patient row → Open detail modal
- Click "Lihat EMR Lengkap" → Navigate to EMR read-only view
- Export CSV → Download patient list

**Logic Flow (Backend Integration):**
1. Load: GET `/api/admin/patients?branch_id=X` dengan search, filter, sort
2. Display patient list
3. Click row → GET `/api/admin/patients/:patient_id` → Open modal
4. Display visit history (latest 5): GET `/api/admin/patients/:patient_id/visits`
5. Click "Export CSV" → POST `/api/admin/patients/export` → Download

---

### 4. KEUANGAN (INVOICE & PAYMENT) → `/admin/finance`

**Purpose:** Manage invoices, track payments, export reports

**Layout:**

```
┌─ Page Header ──────────────────────────────────────┐
│ Keuangan > Cabang Medan                            │
│ [Filter: All/Unpaid/Paid] [Date Range] [Export CSV]│
└────────────────────────────────────────────────────┘

┌─ Summary Cards (3x) ───────────────────────────────┐
│ ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │
│ │ Total Revenue│  │ Unpaid       │  │ Pending DP   │ │
│ │ Hari Ini     │  │ Invoices     │  │ Confirmation │ │
│ │ Rp 2.1M      │  │ Rp 500K (3)  │  │ Rp 150K (2)  │ │
│ └──────────────┘  └──────────────┘  └──────────────┘ │
└────────────────────────────────────────────────────┘

┌─ Invoice List Table ───────────────────────────────┐
│ ┌────────────────────────────────────────────────┐ │
│ │ No │ Invoice ID │ Pasien │ Tgl │ DP │ Tindakan │ │
│ │    │            │        │     │    │ Cost     │ │
│ │─────────────────────────────────────────────────│ │
│ │    │ INV-A-20260707-001 │ Ahmad │ 7/7 │ 50K │ 200K │ │
│ │    │ Status: UNPAID [badge] │ [Input Pembayaran] │ │
│ │─────────────────────────────────────────────────│ │
│ │ ... (more invoices)                            │ │
│ └────────────────────────────────────────────────┘ │
│ [Pagination]                                      │
└────────────────────────────────────────────────────┘

┌─ Payment Input Modal (on click Input Pembayaran) ──┐
│ INPUT PEMBAYARAN                                   │
│ ┌────────────────────────────────────────────────┐ │
│ │ Invoice: INV-A-20260707-001 (read-only)        │ │
│ │ Pasien: Ahmad Sudibyo (read-only)              │ │
│ │ Total: Rp 250.000 (read-only)                  │ │
│ │                                                 │ │
│ │ Nominal Bayar: [Rp __________] (editable)      │ │
│ │ Metode Pembayaran:                              │ │
│ │   ○ Cash                                        │ │
│ │   ○ Transfer                                    │ │
│ │ Waktu Pembayaran: [Date/Time picker] (auto now) │
│ │ Catatan: [textarea, optional]                  │ │
│ │                                                 │ │
│ │ [Simpan Pembayaran] [Batal]                    │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘
```

**Components:**

- **Summary Cards (3x):**
  - Total Revenue (Hari Ini): Sum all paid invoices today
  - Unpaid Invoices: Count & total amount pending
  - Pending DP Confirmation: Count & total awaiting payment gateway confirmation

- **Invoice List Table:**
  - Columns: No, Invoice ID, Pasien, Tanggal, DP, Tindakan Cost, Total, Status, Aksi
  - Filter: All, Unpaid, Paid
  - Filter: Date range
  - Search: By invoice ID, patient name
  - Status badge: UNPAID (red), PAID (green)
  - Action: "Input Pembayaran" button (only for UNPAID)

- **Payment Input Modal:**
  - Invoice info (read-only): INV-ID, Pasien, Total amount
  - Nominal input: Amount paid (should match total, but allow partial)
  - Metode: Cash or Transfer (radio buttons)
  - Waktu Pembayaran: Auto-filled with current datetime, editable
  - Catatan: Optional notes
  - Validation: Nominal > 0, metode selected
  - Save button: POST `/api/admin/invoices/:invoice_id/payment`

**Interaction:**
- Filter/search → Reload table
- Click "Input Pembayaran" → Open payment modal
- Fill form → Validate → Click "Simpan" → Update invoice status to PAID → Toast success
- Export CSV → Download payment report

**Logic Flow (Backend Integration):**
1. Load: GET `/api/admin/invoices?branch_id=X` dengan status filter, date range, search
2. Display invoice list
3. Click "Input Pembayaran" → Open modal dengan pre-filled invoice data
4. Submit payment → PUT `/api/admin/invoices/:invoice_id/payment` dengan { nominal, method, timestamp, notes }
5. Backend: Update invoice status → PAID, log PAYMENT_INPUT audit event
6. Return success → Update table, Toast "Pembayaran berhasil dicatat"

---

### 5. INVENTARIS (INVENTORY MANAGEMENT) → `/admin/inventory`

**Purpose:** Manage stock of materials & supplies (admin only, per PRD v10)

**Layout:**

```
┌─ Page Header ──────────────────────────────────────┐
│ Inventaris > Cabang Medan                          │
│ [Filter: All/Low Stock] [Search] [Add Item] [Export]│
└────────────────────────────────────────────────────┘

┌─ Summary Card ─────────────────────────────────────┐
│ Total Items: 45 | Low Stock Alert: 8 items        │
│ [Show Low Stock Only]                             │
└────────────────────────────────────────────────────┘

┌─ Inventory Table ──────────────────────────────────┐
│ ┌────────────────────────────────────────────────┐ │
│ │ No │ Item │ Kategori │ Stok │ Min │ Unit │ Aksi │
│ │─────────────────────────────────────────────────│ │
│ │ 1 │ Composite A2 │ Material │ 5 │ 10 │ pcs │ Edit │
│ │   │ ⚠️ LOW STOCK │ (red bg highlight)         │ Del │
│ │─────────────────────────────────────────────────│ │
│ │ 2 │ IRM MD Plus │ Material │ 20 │ 5 │ tube │ Edit │
│ │   │ Status: OK (green)                         │ Del │
│ │─────────────────────────────────────────────────│ │
│ │ ... (more items)                               │ │
│ └────────────────────────────────────────────────┘ │
│ [Pagination]                                      │
└────────────────────────────────────────────────────┘

┌─ Add/Edit Item Modal ──────────────────────────────┐
│ TAMBAH ITEM INVENTARIS                             │
│ ┌────────────────────────────────────────────────┐ │
│ │ Nama Item: [Input]                              │ │
│ │ Kategori: [Dropdown: Material, Tools, Other]   │ │
│ │ Stok Saat Ini: [Number input]                   │ │
│ │ Stok Minimum: [Number input] (default: 5)      │ │
│ │ Unit: [Dropdown: pcs, tube, box, botol, dll]  │ │
│ │ Supplier: [Input, optional]                    │ │
│ │ Harga Unit: [Rp ____] (optional, untuk tracking) │
│ │ Catatan: [Textarea, optional]                  │ │
│ │                                                 │ │
│ │ [Simpan] [Batal]                               │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘

┌─ Delete Confirmation Modal ────────────────────────┐
│ HAPUS ITEM: Composite A2?                          │
│ Anda yakin ingin menghapus item ini?              │
│ [Hapus] [Batal]                                    │
└────────────────────────────────────────────────────┘
```

**Components:**

- **Inventory Summary:**
  - Total items count
  - Low stock alert count
  - Filter checkbox: "Show Low Stock Only"

- **Inventory Table:**
  - Columns: No, Nama Item, Kategori, Stok Saat Ini, Stok Minimum, Unit, Aksi
  - Low stock items: Highlight background (#D08080 with opacity), warning icon
  - Search: By item name
  - Filter: Category (Material, Tools, Other)
  - Sort: By stock level (ascending)
  - Action buttons: "Edit", "Delete"

- **Add/Edit Item Modal:**
  - Nama Item: Text input
  - Kategori: Dropdown
  - Stok Saat Ini: Number input
  - Stok Minimum: Number input (threshold untuk low stock alert)
  - Unit: Dropdown (pcs, tube, box, botol, etc)
  - Supplier: Optional text input
  - Harga Unit: Optional currency input (for cost tracking)
  - Catatan: Optional textarea
  - Validation: Nama & kategori required, stok >= 0
  - Save button: POST `/api/admin/inventory` (create) atau PUT `/api/admin/inventory/:item_id` (update)

- **Delete Confirmation Modal:**
  - Warning message: "Anda yakin ingin menghapus item ini?"
  - Two buttons: "Hapus" (danger), "Batal"

**Interaction:**
- Click "Add Item" → Open add modal
- Click "Edit" → Open edit modal with pre-filled data
- Click "Delete" → Confirm modal → DELETE `/api/admin/inventory/:item_id`
- Filter/search → Reload table

**Logic Flow (Backend Integration):**
1. Load: GET `/api/admin/inventory?branch_id=X` dengan filter, search, sort
2. Display inventory list dengan low stock highlighting
3. Click "Add Item" → Open modal
4. Submit form → POST `/api/admin/inventory` → Validate → Create item
5. Audit log: INVENTORY_CREATE
6. Toast success → Reload table

---

### 6. AUDIT LOG (SYSTEM ACTIVITY) → `/admin/audit-logs`

**Purpose:** View immutable audit logs (read-only, no edit/delete)

**Layout:**

```
┌─ Page Header ──────────────────────────────────────┐
│ Audit Log > Cabang Medan                           │
│ [Filter: Event Type] [Date Range] [Search] [Export]│
└────────────────────────────────────────────────────┘

┌─ Filters ──────────────────────────────────────────┐
│ Event Type: [Dropdown: All, Login, Booking, Payment,│
│             Queue, EMR, Inventory, Schedule]        │
│ Date Range: [From date] - [To date]                │
│ User Name: [Search input]                          │
│ [Apply Filters] [Reset]                            │
└────────────────────────────────────────────────────┘

┌─ Audit Log Table ──────────────────────────────────┐
│ ┌────────────────────────────────────────────────┐ │
│ │ Timestamp │ Event │ User │ Affected │ Detail  │ │
│ │           │ Type  │ Name │ Resource │ (expand)│ │
│ │─────────────────────────────────────────────────│ │
│ │ 7/7/26 │ PAYMENT_INPUT │ Admin │ INV-A-... │ ✓ │
│ │ 14:30  │               │ Budi  │           │   │
│ │        │ (Pembayaran cash, Rp 250K)        │   │
│ │─────────────────────────────────────────────────│ │
│ │ 7/7/26 │ CHECKIN │ Admin │ Booking │ ✓ │
│ │ 14:15  │         │ Budi  │ ABC123  │   │
│ │        │ (Kode: ABCD12, Nomor antrian: 5)  │   │
│ │─────────────────────────────────────────────────│ │
│ │ ... (more logs)                                │ │
│ └────────────────────────────────────────────────┘ │
│ [Pagination]                                      │
└────────────────────────────────────────────────────┘

┌─ Log Detail Modal (on expand) ─────────────────────┐
│ DETAIL AUDIT LOG                                   │
│ ┌────────────────────────────────────────────────┐ │
│ │ Timestamp: 7 Juli 2026, 14:30:45               │ │
│ │ Event Type: PAYMENT_INPUT                      │ │
│ │ User: Admin Budi (admin_id: 123)               │ │
│ │ Affected Resource: Invoice INV-A-20260707-001 │ │
│ │ Description: Pembayaran cash sebesar Rp 250K   │ │
│ │                                                 │ │
│ │ Metadata (JSON):                                │ │
│ │ {                                               │ │
│ │   "invoice_id": "INV-A-20260707-001",          │ │
│ │   "amount": 250000,                            │ │
│ │   "method": "cash",                            │ │
│ │   "timestamp": "2026-07-07T14:30:45Z"          │ │
│ │ }                                               │ │
│ │                                                 │ │
│ │ [Close]                                         │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘
```

**Components:**

- **Filter Section:**
  - Event Type dropdown: All, LOGIN, LOGOUT, FAILED_LOGIN, CHANGE_PASSWORD, BOOKING_CREATED, BOOKING_CANCELLED, BOOKING_CONFIRMED, PAYMENT_REDIRECT, PAYMENT_CONFIRMED, PAYMENT_INPUT, CHECKIN, QUEUE_CALL, SERVICE_START, SERVICE_COMPLETE, NO_SHOW, EMR_CREATED, EMR_UPDATED, EMR_COMPLETED, INVENTORY_CREATE, INVENTORY_UPDATE, INVENTORY_DELETE, SCHEDULE_OVERRIDE, RECONCILIATION_COMPLETED
  - Date range picker: From & To dates
  - User name search: Filter by who performed action
  - Apply & Reset buttons

- **Audit Log Table:**
  - Columns: Timestamp, Event Type (badge-style), User Name, Affected Resource, Detail (expand icon)
  - Immutable: No edit/delete buttons (read-only)
  - Expand icon → Open modal dengan full detail
  - Pagination: 20 items per page default
  - Sort: By timestamp (latest first)

- **Log Detail Modal:**
  - Timestamp: Full ISO format
  - Event Type: Dengan deskripsi Bahasa Indonesia
  - User info: Nama & ID
  - Affected Resource: ID/reference affected object
  - Description: Human-readable summary (Bahasa Indonesia)
  - Metadata: Full JSON data untuk transparency
  - Close button only (no edit/delete)

**Interaction:**
- Filter/search → Reload logs
- Click expand icon → Open detail modal
- Export CSV → Download filtered logs (timestamp, event_type, user, resource_id, description)

**Logic Flow (Backend Integration):**
1. Load: GET `/api/admin/audit-logs?branch_id=X` dengan filters, date range, search, sort
2. Display logs dengan pagination
3. Click expand → Open modal dengan full metadata
4. Export CSV → POST `/api/admin/audit-logs/export` → Download

---

### 7. PENGATURAN (SETTINGS) → `/admin/settings`

**Purpose:** Branch settings, backup, profile & logout

**Layout:**

```
┌─ Page Header ──────────────────────────────────────┐
│ Pengaturan > Cabang Medan                          │
└────────────────────────────────────────────────────┘

┌─ TAB NAVIGATION ───────────────────────────────────┐
│ [Umum] [Profil Admin] [Keamanan] [Backup]         │
└────────────────────────────────────────────────────┘

TAB 1: UMUM (General Settings)
┌─────────────────────────────────────────────────────┐
│ PENGATURAN CABANG                                   │
│ ┌────────────────────────────────────────────────┐ │
│ │ Nama Cabang: Cabang Medan (read-only)          │ │
│ │ Alamat: Jl. Merdeka No. 123, Medan (read-only) │ │
│ │ Nomor Telepon: +62612-123-456 (editable)       │ │
│ │ Email Cabang: medan@klinik.com (editable)      │ │
│ │ Waktu Operasional:                              │ │
│ │   Senin - Jumat: [09:00] - [17:00]             │ │
│ │   Sabtu: [09:00] - [14:00]                     │ │
│ │   Minggu: [Tutup] [checkbox]                   │ │
│ │                                                 │ │
│ │ [Simpan Perubahan] [Reset]                     │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘

TAB 2: PROFIL ADMIN (Admin Profile)
┌─────────────────────────────────────────────────────┐
│ PROFIL ADMIN                                        │
│ ┌────────────────────────────────────────────────┐ │
│ │ Nama: Admin Budi (read-only)                   │ │
│ │ Email: budi_admin@klinik.com (read-only)       │ │
│ │ No. Telepon: 08123456789 (editable)            │ │
│ │ Bio: [Textarea, optional, editable]            │ │
│ │                                                 │ │
│ │ [Edit] [Ganti Password] [Logout]               │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘

TAB 3: KEAMANAN (Security)
┌─────────────────────────────────────────────────────┐
│ KEAMANAN AKUN                                       │
│ ┌────────────────────────────────────────────────┐ │
│ │ PASSWORD SAAT INI                               │ │
│ │ Password Lama: [Input, type=password]           │ │
│ │                                                 │ │
│ │ PASSWORD BARU                                   │ │
│ │ Password Baru: [Input, type=password]           │ │
│ │ Konfirmasi Password: [Input, type=password]     │ │
│ │ Persyaratan:                                    │ │
│ │   ✓ Minimal 8 karakter                         │ │
│ │   ✓ Mengandung huruf besar & kecil            │ │
│ │   ✓ Mengandung angka                          │ │
│ │   ✓ Mengandung simbol (!@#$%^&*)              │ │
│ │                                                 │ │
│ │ [Ganti Password] [Batal]                       │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘

TAB 4: BACKUP (Backup & Restore)
┌─────────────────────────────────────────────────────┐
│ DATABASE BACKUP                                     │
│ ┌────────────────────────────────────────────────┐ │
│ │ Backup Otomatis:                                │ │
│ │ ✓ Enabled (setiap hari jam 02:00 WIB)          │ │
│ │                                                 │ │
│ │ Backup Terakhir:                                │ │
│ │ Tanggal: 7 Juli 2026 | Waktu: 02:15 WIB        │ │
│ │ Size: 12.5 MB                                  │ │
│ │ Status: ✓ Sukses                               │ │
│ │ Location: Local + Supabase auto-backup         │ │
│ │                                                 │ │
│ │ BACKUP MANUAL:                                  │ │
│ │ [Backup Sekarang] [Download Backup] [Restore]  │ │
│ │                                                 │ │
│ │ ℹ️ Restore procedure: Contact IT support       │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘
```

**Components:**

- **Tab Navigation:** Umum, Profil Admin, Keamanan, Backup

- **Tab: Umum (General)**
  - Branch name (read-only)
  - Alamat (read-only)
  - Nomor Telepon: Editable
  - Email Cabang: Editable
  - Waktu Operasional: Time pickers (Sen-Jum, Sabtu, Minggu checkbox)
  - Save button

- **Tab: Profil Admin**
  - Nama (read-only)
  - Email (read-only)
  - Nomor Telepon: Editable
  - Bio: Textarea editable
  - Buttons: Edit, Ganti Password, Logout

- **Tab: Keamanan**
  - Password lama input (required untuk security)
  - Password baru input
  - Konfirmasi password input
  - Password requirements checklist (dynamic: 8 chars, uppercase, lowercase, number, symbol)
  - Validation: Semua requirement terpenuhi sebelum enable save button
  - Ganti Password button

- **Tab: Backup**
  - Backup status display: Last backup time, size, status badge
  - Auto-backup enabled/disabled toggle
  - Manual backup button: "Backup Sekarang"
  - Download backup button
  - Restore button (dengan warning: "Hubungi IT support")
  - Info text: Backup strategy & locations

**Interaction:**
- Edit fields → Validate → Save → Toast success
- Ganti Password → Validate requirements → POST `/api/admin/password-change` → Logout & redirect to login
- Backup Sekarang → POST `/api/admin/backup` → Toast "Backup sedang dijalankan..." → Polling status
- Download Backup → GET `/api/admin/backup/download` → Trigger file download
- Logout → Show confirmation modal → Clear session → Redirect to login

**Logic Flow (Backend Integration):**
1. Load settings: GET `/api/admin/settings?branch_id=X`
2. Edit general settings → PUT `/api/admin/settings` → Audit log: SYSTEM_CONFIG_UPDATE
3. Change password → POST `/api/admin/password-change` dengan { old_password, new_password } → Validate → Update → Logout
4. Trigger backup → POST `/api/admin/backup` → Async job → Return status
5. Download backup → GET `/api/admin/backup/download` → File stream

---

## 🔧 SHARED COMPONENTS & PATTERNS

### Global Components Used Across Admin

#### **1. Top Navbar (Fixed)**
- Height: 64px
- Background: White
- Border-bottom: 1px solid `#E5E7EB`
- Sticky on scroll
- Content:
  - Left: Logo + Breadcrumb
  - Right: Search (optional), User menu (Avatar + Dropdown)
  - User dropdown: Profile, Settings, Logout

#### **2. Sidebar Navigation (Fixed)**
- Width: 260px
- Background: White
- Border-right: 1px solid `#E5E7EB`
- Scroll: Only sidebar content scrolls
- Menu items: Icons + Labels (7 items for admin)
- Active state: Background `#CCFBF1` + text `#0F766E`
- Branch info display: "Cabang [A/B/C]" (small text)

#### **3. Modals & Dialogs**
- No browser `alert()` - use custom modals
- Background overlay: `rgba(0, 0, 0, 0.5)` (50% opacity)
- Modal width: 400-600px (responsive)
- Border-radius: 8px
- Close button (X) top-right
- Buttons: [Primary button] [Secondary button]

#### **4. Toast Notifications**
- Position: Top-right corner
- Auto-dismiss: 3-5 seconds
- Types: Success (green), Error (red), Info (blue), Warning (amber)
- Background opacity: 95%
- Text: White or dark
- Icon + text message
- Example: "✓ Pembayaran berhasil dicatat"

#### **5. Status Badges**
- Inline badges for status
- Background color + rounded corners (border-radius: 20px)
- Padding: 4px 8px
- Font size: 12px
- Examples:
  - "Selesai" → Green (`#10B981`)
  - "Unpaid" → Red (`#D08080`)
  - "Draft" → Blue (`#3B82F6`)
  - "Aktif" → Green
  - "Inactive" → Gray
  - "Low Stock" → Red with warning icon

#### **6. Loading States**
- Skeleton loading (gray placeholder bars) during data fetch
- Spinner icon (rotating, centered)
- Prevent layout shift during load

#### **7. Pagination**
- "Load More" button OR numbered pagination
- Default: 10-20 items per page
- Responsive: Single column on mobile

#### **8. Form Inputs**
- Input field: Border 1px `#E5E7EB`, focus border `#0F766E`
- Focus outline: 2px solid `#0F766E`
- Error state: Border `#D08080`, error message below in red
- Disabled state: Background `#F3F4F6`, cursor not-allowed
- Label: 13px, Medium (500), Neutral Dark (`#1E293B`)
- Helper text: 12px, Gray (`#9CA3AF`)

#### **9. Buttons**
- Primary: Background `#0F766E`, text white, 44px height (min)
- Secondary: Border `#0F766E`, text `#0F766E`, white background
- Danger: Background `#D08080`, text white (for destructive actions)
- Disabled: Background `#D1D5DB`, cursor not-allowed
- Hover: Slightly darker shade
- Border-radius: 6px
- Font: 14px, Semi-bold (600)

#### **10. Tables**
- Header: Background `#F9FAFB`, text `#374151` (Neutral Dark)
- Rows: Alternate white & `#F9FAFB` (subtle striping)
- Row hover: Background `#CCFBF1` (Primary Light)
- Border: 1px solid `#E5E7EB`
- Cell padding: 12px
- Low stock rows: Highlight background with alert color & warning icon

---

## 📱 RESPONSIVE BREAKPOINTS

| Breakpoint | Width | Usage |
|---|---|---|
| Mobile | < 640px | (Out of scope for Web portal - desktop only) |
| Tablet | 640px - 1024px | Sidebar collapses to hamburger menu, full width content |
| Desktop | > 1024px | Full layout (sidebar + content) |
| Large Desktop | > 1440px | Max content width 1440px (centered with margins) |

**For Web Portal (Admin):**
- **Desktop-first design**
- **Tablet adaptation:** Sidebar → Hamburger menu toggle, content full-width
- **Mobile:** Not primary target (but can work on tablets)

---

## 🎯 DESIGN HANDOFF CHECKLIST

- [ ] All pages use consistent color palette (#0F766E primary, #D08080 alerts, etc.)
- [ ] All text contrast passes WCAG AA (18px for large text, 14px normal)
- [ ] No browser alerts - use custom modals only
- [ ] Loading states (skeleton/spinner) implemented on data-heavy pages
- [ ] Toast notifications for all success/error actions
- [ ] Responsive sidebar (collapse on tablet)
- [ ] Real-time updates for queue (polling indicator or WebSocket indicator)
- [ ] Low stock items highlighted in red (#D08080) in inventory table
- [ ] Immutable audit log (no edit/delete buttons)
- [ ] Button minimum height 44px for accessibility
- [ ] Focus states visible (2px outline)
- [ ] Form validation errors clear & actionable (Bahasa Indonesia)
- [ ] No Dashboard menu for admin (sesuai PRD v10.0)
- [ ] 7 menu items only: Antrian, Dokter, Pasien, Keuangan, Inventaris, Audit Log, Pengaturan

---

## 📝 NOTES FOR IMPLEMENTATION

- Use React for component structure (if Next.js frontend)
- Implement CSS-in-JS or Tailwind for styling (with design tokens matching palette)
- Use context/state management for auth & branch selection
- Implement real-time updates with polling (2-3 sec) or WebSocket
- All form submissions: client-side validation + server-side validation
- Toast library: React Toastify or similar
- Modal library: Headless UI or Radix UI
- Table library: TanStack React Table (for large tables)
- Date picker: react-datepicker or similar
- File upload: react-dropzone
- Loading states: React Skeleton or custom spinner
- Error handling: Try-catch + toast notification with error message
- Logging: Console logs + analytics tracking
- Real-time queue: Polling every 2-3 seconds, or WebSocket for live updates

---

## 🔗 PAGE ROUTING SUMMARY

| Page | Route | Purpose |
|---|---|---|
| Login | `/admin/login` | Authentication (shared) |
| Queue | `/admin/queue` | Queue management & check-in |
| Doctors | `/admin/doctors` | Doctor management & schedule |
| Patients | `/admin/patients` | Patient list & history |
| Finance | `/admin/finance` | Invoice & payment tracking |
| Inventory | `/admin/inventory` | Stock management |
| Audit Logs | `/admin/audit-logs` | System activity logs |
| Settings | `/admin/settings` | Branch settings & backup |

---

## ✅ VALIDATION CHECKLIST (vs PRD v10.0 & HALAMAN.txt)

- ✅ Admin menu: 7 items only (NO Dashboard)
- ✅ Menu items: Antrian, Dokter, Pasien, Keuangan, Inventaris, Audit Log, Pengaturan
- ✅ Queue: Live monitoring, check-in form, walk-in support, real-time updates
- ✅ Doctor: Manage schedule (override), FIFO system
- ✅ Patient: List, search, filter, visit history, EMR access (read-only for admin)
- ✅ Finance: Invoice list, payment input, export CSV
- ✅ Inventory: Stock management (admin only, NOT for doctor)
- ✅ Audit Log: Immutable, append-only, no edit/delete
- ✅ Settings: Branch config, backup, admin profile, password change, logout
- ✅ Design style: Same flat 2.0 SaaS as doctor portal (consistent)
- ✅ Responsive: Desktop-first, tablet & mobile support
- ✅ Accessibility: WCAG AA compliant
- ✅ Real-time queue updates: Polling 2-3 detik atau WebSocket
- ✅ Auto-delete completed queue numbers: TV display + admin queue page
- ✅ Role-based menu: Stok hanya admin, bukan dokter ✓

---

**END OF DESIGN ADMIN WEB.md**

Version: 1.0 (Final - 100% sesuai PRD v10.0 & HALAMAN.txt)  
Tanggal: 2026-07-07  
Status: Ready for Hi-Fi Prototype & Frontend Implementation  
Menu Items: 7 (NO Dashboard) | Pages: 7 | Validation: 100% Sesuai PRD
