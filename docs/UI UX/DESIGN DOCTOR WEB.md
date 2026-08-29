# DENTFLOW v10.0 - DESIGN DOCTOR WEB.md
## Doctor Portal (Desktop-First)

**Versi:** 1.0 (REVISED - Dashboard Removed)  
**Tanggal:** 2026-07-07  
**Device:** Web Desktop (Responsive untuk tablet)  
**Target:** Stitch With Google / Figma Hi-Fi Prototype  
**Status:** 100% Sesuai HALAMAN.txt (section 14: DOCTOR WEB)

---

## 📋 TABLE OF CONTENTS

1. [Global Design System](#global-design-system)
2. [Architecture Overview](#architecture-overview)
3. [Doctor Portal Navigation](#doctor-portal-navigation)
4. [Page Details](#page-details)
5. [Shared Components & Patterns](#shared-components--patterns)
6. [Responsive Breakpoints](#responsive-breakpoints)

---

## 🎨 GLOBAL DESIGN SYSTEM

### Color Palette (Sama dengan Admin Portal)

| Color Name | Hex Code | Usage | Notes |
|---|---|---|---|
| Primary (Deep Teal) | `#0F766E` | Tombol utama, menu aktif, text headings | Memancarkan higiene & trust |
| Primary Light | `#CCFBF1` | Hover states, row highlight, badge | Soft highlight untuk interaksi |
| Alert / Danger | `#D08080` | Status negatif, Allergy alert | Hindari di text murni (gunakan badge) |
| Neutral Dark (Slate 800) | `#1E293B` | Body text utama, labels | Keterbacaan tinggi |
| Neutral Light (Slate 50) | `#F8FAFC` | Background dasar aplikasi | Minimal eye-strain |
| Surface White | `#FFFFFF` | Background card, modal, tabel | Clean & airy |
| Gray 400 | `#9CA3AF` | Border, disabled state, secondary text | Visual hierarchy |
| Success (Green) | `#10B981` | Status success, "Selesai", "Ready" | Psychological positive |
| Info (Blue) | `#3B82F6` | Status info, "Waiting", "Draft" | Informatif, netral |
| Warning (Amber) | `#F59E0B` | Status warning, "Called", "In Service" | Attention-grabbing |

### Typography (Sama dengan Admin)

```
Heading 1 (H1): 32px, Bold (700), Line-height 1.2
Heading 2 (H2): 24px, Bold (700), Line-height 1.3
Heading 3 (H3): 20px, Semi-bold (600), Line-height 1.4
Body Text (p): 14px, Regular (400), Line-height 1.5
Small Text: 12px, Regular (400), Line-height 1.4
Label (form): 13px, Medium (500), Line-height 1.4
```

**Font Family:** Inter, -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, sans-serif

### Spacing & Layout (Sama dengan Admin)

- Base grid unit: 4px (multiples: 4px, 8px, 12px, 16px, 20px, 24px, 32px, 40px, 48px)
- Desktop container width: 1440px (max width)
- Sidebar width: 260px (fixed)
- Main content area: Full width minus sidebar
- Card padding: 20px
- Page padding: 24px

### Accessibility Rules (Sama dengan Admin)

- **WCAG AA Compliance:** Semua warna text harus lolos contrast test
- Alert/Danger (#D08080) untuk text: Gunakan dengan opacity atau di dark background
- Focus states: Outline 2px solid `#0F766E` dengan offset 2px
- Interactive elements: Min height 44px untuk mobile accessibility

---

## 🏗️ ARCHITECTURE OVERVIEW

### Main Layout Structure (Web Doctor)

```
┌─────────────────────────────────────────────────────────────┐
│ TOP NAVBAR (Fixed, Height: 64px)                            │
│ Logo | Doctor Name Dropdown | Logout                        │
├──────────────┬──────────────────────────────────────────────┤
│              │                                              │
│ LEFT SIDEBAR │ MAIN CONTENT AREA                            │
│ (260px,      │ (Responsive, fullwidth - 260px)             │
│ Fixed)       │                                              │
│              │                                              │
│ Navigation   │ Page Title + Context Info                    │
│ Menu Items   │                                              │
│              │ ┌──────────────────────────────────────────┐ │
│ - Antrian    │ │ PAGE CONTENT                             │ │
│ - Pasien     │ │ (Tables, Forms, Modals, etc)            │ │
│ - EMR        │ │                                          │ │
│ - Profil     │ │                                          │ │
│              │ │                                          │ │
│              │ │                                          │ │
│              │ └──────────────────────────────────────────┘ │
└──────────────┴──────────────────────────────────────────────┘
```

### Navigation Strategy

**Left Sidebar (Fixed):**
- Fixed width: 260px
- Background: `#FFFFFF` (white)
- Border-right: 1px solid `#E5E7EB` (gray-200)
- Scroll: Only sidebar content scrolls, always visible

**Top Navbar (Fixed):**
- Height: 64px
- Background: `#FFFFFF`
- Border-bottom: 1px solid `#E5E7EB`
- Sticky on scroll
- Content: Logo, Doctor Name + Dropdown, Logout

---

## 👨‍⚕️ DOCTOR PORTAL NAVIGATION

### Doctor Role Menu (Sidebar)

**Struktur Sidebar (per HALAMAN.txt section B):**

```
┌─────────────────────────┐
│ DENTFLOW DOCTOR         │ (20px padding top)
│ [Logo] Klinik Gigi      │
├─────────────────────────┤
│ MANAGEMENT              │
│ [icon] Antrian          │ (Primary if active: bg #CCFBF1, text #0F766E)
│ [icon] Pasien (optional)│
│ [icon] EMR              │
├─────────────────────────┤
│ ACCOUNT                 │
│ [icon] Profil           │
│                         │
│ [Avatar]                │
│ Dr. Nama                │
│ Sp.Spesialisasi         │
│ [Logout button]         │
└─────────────────────────┘
```

**Menu Items (4 items - per HALAMAN.txt section B):**

1. **Antrian** - `/doctor/queue`
   - Live queue display for the day
   - Primary entry point after login
   - Status-based patient management (WAITING, CALLED, IN_SERVICE, COMPLETED)

2. **Pasien** - `/doctor/patients`
   - Patient list treated by this doctor (optional per HALAMAN.txt)
   - Search, filter, visit history
   - Optional feature - can be implemented in V1.1

3. **EMR** - `/doctor/emr`
   - Medical records archive (encounters list)
   - Filter by date, status, patient name
   - View, edit, finalize encounters

4. **Profil** - `/doctor/profile`
   - Doctor profile & settings
   - Edit email, phone, password
   - Logout

**Key Notes:**
- **NO DASHBOARD PAGE** - Doctor redirects directly to `/doctor/queue` after login
- Minimal menu structure (only 4 items)
- Same design style as admin portal (flat 2.0 SaaS)
- "Stok/Inventaris" NOT available for doctor (role-based)

---

## 📄 PAGE DETAILS

### 1. ANTRIAN (DOCTOR'S QUEUE) → `/doctor/queue`

**Purpose (per HALAMAN.txt C):** Live queue monitoring, patient service start workflow

**URL:** `/doctor/queue`  
**Role:** DOCTOR (authenticated)  
**Device:** Web  
**Real-time:** Polling 3 detik

**Layout:**

```
┌─ Page Header ──────────────────────────────────────┐
│ Antrian Pasien Saya Hari Ini                       │
│ Dr. Budi | Sp.KG | Cabang Medan                    │
└────────────────────────────────────────────────────┘

┌─ Summary (Optional) ───────────────────────────────┐
│ Total antrian: 8 | Sedang dilayani: 1 | Selesai: 3│
└────────────────────────────────────────────────────┘

┌─ Queue Table (Real-time, polling 3 detik) ────────┐
│ ┌────────────────────────────────────────────────┐ │
│ │Nomor│ Nama Pasien │ Status │ Check-in │ Aksi   │ │
│ │─────────────────────────────────────────────────│ │
│ │ 1   │ Ahmad B.    │WAITING│ 10:00    │ [Panggil]
│ │     │             │       │          │ [Mulai] │
│ │─────────────────────────────────────────────────│ │
│ │ 2   │ Siti M.     │WAITING│ 10:15    │ [Panggil]
│ │     │             │       │          │ [Mulai] │
│ │─────────────────────────────────────────────────│ │
│ │ 3   │ Budi H.     │CALLED │ 10:30    │ [Mulai]  │
│ │     │             │       │          │ [Ulang] │
│ │─────────────────────────────────────────────────│ │
│ │ 4   │ Hasan A.    │WAITING│ 10:45    │ [Panggil]
│ │     │             │       │          │ [Mulai] │
│ │─────────────────────────────────────────────────│ │
│ │ 5   │ Rudi S.     │IN_SRV │ 11:00    │ [Selesai] 
│ │     │             │       │          │ [Input] │
│ │─────────────────────────────────────────────────│ │
│ │ ... │ ...         │  ...  │ ...      │ ...     │ │
│ └────────────────────────────────────────────────┘ │
│ [Load More] or Pagination                          │
└────────────────────────────────────────────────────┘

┌─ Patient Detail Modal (Click row) ─────────────────┐
│ DETAIL PASIEN                                      │
│ ┌──────────────────────────────────────────────┐  │
│ │ Nama: Ahmad B.                               │  │
│ │ Nomor Antrian: 1                             │  │
│ │ Status: WAITING / CALLED / IN_SERVICE        │  │
│ │ Check-in: 10:00                              │  │
│ │                                              │  │
│ │ ⚠️ ALERGI OBAT: Penisilan, Amoksisilin       │  │ (if applicable)
│ │ (Red badge, #D08080 background)              │  │
│ │                                              │  │
│ │ Keluhan: Gigi berlubang                      │  │
│ │ Riwayat Kunjungan: 2x (Last: 3 bulan lalu)   │  │
│ │                                              │  │
│ │ [Panggil] [Mulai Layani] [Lihat EMR Lalu]    │  │
│ │ [Close]                                      │  │
│ └──────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────┘
```

**Components:**

- **Page Header:** Title, doctor info, branch name
- **Summary Card (Optional):** Total, In Service, Completed counts
- **Queue Table:**
  - Columns: Nomor, Nama Pasien, Status (badge), Waktu Check-in, Aksi (buttons)
  - Real-time polling 3 detik
  - Rows clickable to show patient detail
  - Pagination or load more

- **Patient Detail Modal:**
  - Patient name, queue number, status, check-in time
  - Allergy alert (if any)
  - Chief complaint summary
  - Visit history
  - Action buttons (status-dependent):
    - **If WAITING:** [Panggil], [Mulai Layani]
    - **If CALLED:** [Mulai Layani], [Panggil Ulang]
    - **If IN_SERVICE:** [Selesai Layani] or "Input EMR"
  - [Lihat EMR Lalu] link to previous encounter

**Status Badges (per HALAMAN.txt C):**

- WAITING → `#3B82F6` (blue), white text
- CALLED → `#F59E0B` (amber), white text
- IN_SERVICE → `#F59E0B` (amber), white text
- COMPLETED → `#10B981` (green), white text

**Interaction (per HALAMAN.txt C Logic Flow):**

1. **Load:** GET `/queues/doctor/:doctor_id?date=[today]` (polling 3 detik)
2. **[Panggil]:** POST `/queues/:queue_id/call` → status WAITING → CALLED → FCM notification
3. **[Mulai Layani]:** POST `/queues/:queue_id/start-service` → status CALLED → IN_SERVICE
4. **[Selesai Layani]:** Navigate to `/doctor/emr/new?queue_id=[queue_id]`
5. **[Panggil Ulang]:** Re-call patient if needed
6. **[Lihat EMR Lalu]:** Show previous encounter in read-only modal
7. **Real-time updates:** Queue refreshes every 3 seconds

---

### 2. EMR INPUT PAGE (NEW ENCOUNTER) → `/doctor/emr/new?queue_id=[queue_id]`

**Purpose (per HALAMAN.txt D):** Input clinical data after service completes, trigger auto-invoice

**URL:** `/doctor/emr/new?queue_id=[queue_id]` or `/doctor/emr/create`  
**Role:** DOCTOR (authenticated)  
**Device:** Web  

**Layout:**

```
┌─ Page Header ──────────────────────────────────────┐
│ Input Rekam Medis (EMR)                            │
│ Pasien: Ahmad B. | 7 Juli 2026                     │
│ [Simpan Draft] [Selesaikan EMR] [Batal]            │
└────────────────────────────────────────────────────┘

┌─ ALLERGY ALERT BANNER (Top, CRITICAL) ─────────────┐
│ ⚠️ ALERGI OBAT: Penisilan, Amoksisilin             │
│ Background: #D08080 | Text: White | Bold           │
└────────────────────────────────────────────────────┘

┌─ Patient Info (Read-only) ─────────────────────────┐
│ Nama: Ahmad B. | Tanggal: 7 Juli 2026 | Waktu: 10:00
└────────────────────────────────────────────────────┘

┌─ EMR Form (2-column layout) ───────────────────────┐
│                                                    │
│ LEFT COLUMN (60%):                                 │
│ ┌──────────────────────────────────────────────┐  │
│ │ KELUHAN / ANAMNESA (required)                │  │
│ │ [Textarea, min 10 chars]                     │  │
│ │                                              │  │
│ │ DIAGNOSIS (required)                         │  │
│ │ [Textarea, min 10 chars]                     │  │
│ │                                              │  │
│ │ TINDAKAN MEDIS (required, ≥1)                │  │
│ │ [Multi-select Checkboxes]:                   │  │
│ │ □ Scaling □ Filling □ Extraction             │  │
│ │ □ Cleaning □ Konsultasi □ [Custom]          │  │
│ │                                              │  │
│ │ RESEP & INSTRUKSI (optional)                 │  │
│ │ [Textarea]                                   │  │
│ │ [+ Add Medicine] (for structured Rx)         │  │
│ │ ┌────────────────────────────────────────┐  │  │
│ │ │ 1. Paracetamol 500mg × 3/hari (5 hari) │  │  │
│ │ │    [Edit] [Delete]                      │  │  │
│ │ │ 2. Amoxicillin 500mg (⚠️ ALERGI)        │  │  │
│ │ │    [Edit] [Delete]                      │  │  │
│ │ └────────────────────────────────────────┘  │  │
│ │                                              │  │
│ │ LAMPIRAN / FILE (optional)                   │  │
│ │ [Upload File] (max 5 files, 5MB each)       │  │
│ │ ┌────────────────────────────────────────┐  │  │
│ │ │ Uploaded files:                        │  │  │
│ │ │ • foto-rontgen.pdf [Download] [Delete] │  │  │
│ │ └────────────────────────────────────────┘  │  │
│ │                                              │  │
│ └──────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────┘

┌─ Form Actions (Bottom, Sticky) ─────────────────────┐
│ [Simpan Draft] [Selesaikan EMR] [Batal]            │
└────────────────────────────────────────────────────┘
```

**Components (per HALAMAN.txt D):**

- **Allergy Alert Banner** (top, non-dismissible)
- **Patient Info** (pre-filled from queue, read-only)
- **Form Fields:**
  1. Keluhan / Anamnesa (textarea, required, min 10 chars)
  2. Diagnosis (textarea, required, min 10 chars)
  3. Tindakan Medis (multi-select checkboxes, required ≥1)
  4. Resep & Instruksi (textarea, optional)
     - Free-text OR structured medicine list via [+ Add Medicine]
     - Allergy check on medicine selection
  5. Lampiran / File (file upload, optional, max 5 files)

**Validation:**

- Required: Keluhan, Diagnosis, Tindakan (≥1)
- Disable "Selesaikan EMR" until all required fields valid
- Show inline error messages

**Interaction (per HALAMAN.txt D Logic Flow):**

1. **Load:** GET `/queues/:queue_id` → Pre-fill patient info, allergies
2. **Fill form:** Real-time validation
3. **Add medicine:** Check allergy → Show warning if match
4. **Upload file:** Validate type & size
5. **[Simpan Draft]:** 
   - Validate required fields
   - POST `/encounters { patient_id, queue_id, complaint, diagnosis, actions, prescription, files }` status: DRAFT
   - Toast: "EMR disimpan sebagai draft"
   - Stay on form
6. **[Selesaikan EMR]:**
   - Validate all required fields
   - POST `/encounters/:encounter_id/complete`
   - Backend:
     - Update status → COMPLETED
     - Update queue status → COMPLETED
     - **Trigger auto-invoice** (per PRD v10)
   - Toast: "EMR selesai. Invoice otomatis dibuat"
   - Redirect to `/doctor/queue`
7. **[Batal]:** Confirm if unsaved → Back to queue

---

### 3. EMR LIST / HISTORY → `/doctor/emr`

**Purpose (per HALAMAN.txt E):** View archived medical records

**URL:** `/doctor/emr` or `/doctor/encounters`  
**Role:** DOCTOR  
**Device:** Web  

**Layout:**

```
┌─ Page Header ──────────────────────────────────────┐
│ Riwayat EMR Saya                                   │
│ [Filter] [Export CSV]                              │
└────────────────────────────────────────────────────┘

┌─ Filter Bar ───────────────────────────────────────┐
│ Status: [Semua ▼] [Draft ▼] [Completed ▼]          │
│ Date Range: [From] [To]                            │
│ Patient Name: [Search]                             │
│ [Reset Filters]                                    │
└────────────────────────────────────────────────────┘

┌─ EMR List Table ───────────────────────────────────┐
│ ┌────────────────────────────────────────────────┐ │
│ │ Tanggal │ Pasien │ Status │ Tindakan │ Aksi   │ │
│ │─────────────────────────────────────────────────│ │
│ │ 7 Jul   │ Ahmad  │✓DONE  │ Scaling  │ Lihat  │ │ (green)
│ │ 7 Jul   │ Siti   │DRAFT  │ Konsult  │ Lihat/E│ │ (blue/amber)
│ │ 6 Jul   │ Budi   │✓DONE  │ Pencab.  │ Lihat  │ │
│ │ ... (paginated)                                 │ │
│ └────────────────────────────────────────────────┘ │
│ [Load More] or Pagination                          │
└────────────────────────────────────────────────────┘
```

**Components:**

- **Filter Bar:** Status, date range, patient name, reset
- **EMR List Table:**
  - Columns: Tanggal, Pasien, Status (badge), Tindakan, Aksi
  - Status badges: DRAFT (blue/amber), COMPLETED (green)
  - Actions: [Lihat Detail] [Edit] (if DRAFT)

**Interaction (per HALAMAN.txt E Logic Flow):**

1. **Load:** GET `/encounters?doctor_id=[from_token]`, filters, date_range
2. **[Lihat Detail]:** Navigate to `/doctor/emr/:encounter_id` (read-only)
3. **[Edit]** (if DRAFT): Navigate to `/doctor/emr/:encounter_id/edit` (editable)
4. **Filter:** Reload table with filters
5. **Search:** Real-time patient name filter

---

### 4. DOCTOR PROFILE → `/doctor/profile`

**Purpose (per HALAMAN.txt F):** Doctor profile & settings

**URL:** `/doctor/profile`  
**Role:** DOCTOR  
**Device:** Web  

**Layout:**

```
┌─ Page Header ──────────────────────────────────────┐
│ Profil Saya                                        │
└────────────────────────────────────────────────────┘

┌─ TAB NAVIGATION (2 tabs) ──────────────────────────┐
│ [Profil Dokter] [Ganti Password]                   │
└────────────────────────────────────────────────────┘

┌─ TAB 1: PROFIL DOKTER ────────────────────────────┐
│ ┌──────────────────────────────────────────────┐  │
│ │ INFORMASI PROFIL                             │  │
│ │ Nama: Dr. Budi Santoso (read-only)           │  │
│ │ Spesialisasi: Sp.KG (read-only)              │  │
│ │ Email: [Input: budi@medan.com] (editable)    │  │
│ │ No. Telepon: [Input: 08123456789] (editable) │  │
│ │ Bio (optional): [Textarea]                    │  │
│ │                                              │  │
│ │ [Simpan Perubahan] [Reset]                   │  │
│ └──────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────┘

┌─ TAB 2: GANTI PASSWORD ────────────────────────────┐
│ ┌──────────────────────────────────────────────┐  │
│ │ GANTI PASSWORD                               │  │
│ │ Password Lama: [Input, type=password]         │  │
│ │ Password Baru: [Input, type=password]         │  │
│ │ Konfirmasi: [Input, type=password]            │  │
│ │ [Simpan] [Batal]                             │  │
│ │                                              │  │
│ │ Persyaratan:                                 │  │
│ │ • Min 8 karakter                             │  │
│ │ • Kombinasi huruf, angka, & simbol           │  │
│ └──────────────────────────────────────────────┘  │
│                                                    │
│ [Logout] (dengan confirm modal)                   │
└────────────────────────────────────────────────────┘
```

**Components:**

- **TAB 1:** Name, Specialist, Email (edit), Phone (edit), Bio (optional), Save/Reset buttons
- **TAB 2:** Old password, new password, confirm password, save/cancel buttons
- **Logout:** Separate button with confirm modal

**Interaction:**

- Edit profile → Validate → Save → Toast
- Change password → Validate old password → Save → Toast
- Logout → Confirm → Clear session → Redirect to login

---

## 🎨 SHARED COMPONENTS & PATTERNS

### Buttons

**Primary Button:**
- Background: `#0F766E` (deep teal)
- Text: White, padding: 10px 20px, border-radius: 6px
- Hover: Darker shade
- Disabled: `#9CA3AF` (gray)

**Secondary Button:**
- Background: White, border: 1px `#0F766E`
- Text: `#0F766E`, padding: 10px 20px, border-radius: 6px
- Hover: Background `#CCFBF1`
- Disabled: Border `#9CA3AF`, text `#9CA3AF`

**Danger Button:**
- Background: `#D08080` (soft red)
- Text: White, padding: 10px 20px
- Hover: Darker shade
- Used for: Delete, No Show, Cancel

### Status Badges

| Status | Background | Color | Text |
|---|---|---|---|
| WAITING | `#3B82F6` | Blue | White |
| CALLED | `#F59E0B` | Amber | Dark |
| IN_SERVICE | `#F59E0B` | Amber | Dark |
| COMPLETED | `#10B981` | Green | White |
| DRAFT | `#3B82F6` | Blue | White |
| ✓ DONE | `#10B981` | Green | White |

### Alert Badges

- Allergy: Background `#D08080`, white text, alert icon

### Form Components

**Input Fields:**
- Border: 1px solid `#E5E7EB`
- Padding: 10px 12px, border-radius: 6px
- Focus: Border `#0F766E`, outline 2px
- Error: Border `#D08080`

**Textarea:**
- Same as inputs, min height 100px

**Checkboxes:**
- Size: 20x20px
- Border: 1px solid `#9CA3AF`
- Checked: Background `#0F766E`, checkmark

**Dropdowns:**
- Border: 1px solid `#E5E7EB`
- Border-radius: 6px, padding: 10px 12px

**Date Picker:**
- Calendar widget
- Selected: Background `#0F766E`, white text
- Today: Border highlight

### Modals & Dialogs

**Modal Structure:**
- Overlay: Black 50% opacity
- Dialog: White, border-radius 8px
- Header: Title + close button
- Body: Content
- Footer: Action buttons

**Confirm Modal:**
- Warning icon, question message
- [Confirm] (danger color) + [Cancel] (secondary)

### Tables

**Header:** Background `#F8FAFC`, bold text `#1E293B`  
**Rows:** White, alternating light gray  
**Borders:** 1px solid `#E5E7EB`  
**Cell padding:** 12px 16px  
**Hover:** Background `#CCFBF1`

### Toast Notifications

**Position:** Bottom-right, auto-dismiss 4 sec  
**Variants:**
- Success: `#10B981` (green)
- Error: `#D08080` (red)
- Warning: `#F59E0B` (amber)
- Info: `#3B82F6` (blue)

---

## 📱 RESPONSIVE BREAKPOINTS

**Desktop (1440px):**
- Sidebar: 260px fixed
- Main: Full width - 260px
- 2-column layout (EMR): 60% / 40%

**Tablet (768px - 1023px):**
- Sidebar: 200px or collapse
- Main: Adjust
- 1-column layout (stack vertically)

**Mobile (<767px):**
- Sidebar: Hidden (hamburger menu)
- Main: Full width
- Tables: Horizontal scroll or card layout
- All layouts: 1-column stack

---

## 🔗 PAGE ROUTING SUMMARY

| Page | Route | Purpose |
|---|---|---|
| Login | `/doctor/login` | Authentication |
| Queue | `/doctor/queue` | Live queue (PRIMARY) |
| EMR Input (New) | `/doctor/emr/new?queue_id=X` | Create EMR |
| EMR Detail | `/doctor/emr/:encounter_id` | View EMR |
| EMR Edit | `/doctor/emr/:encounter_id/edit` | Edit draft EMR |
| EMR Archive | `/doctor/emr` | EMR list/history |
| Patients | `/doctor/patients` | Patient list (optional) |
| Profile | `/doctor/profile` | Profile & settings |

---

## ✅ VALIDATION CHECKLIST (vs HALAMAN.txt section 14)

- ✅ NO DASHBOARD PAGE (HALAMAN.txt doesn't mention it)
- ✅ Doctor redirects to `/doctor/queue` after login
- ✅ 4 menu items: Antrian, Pasien (optional), EMR, Profil
- ✅ Queue page: WAITING, CALLED, IN_SERVICE, COMPLETED statuses
- ✅ Queue actions: [Panggil], [Mulai Layani], [Selesai Layani]
- ✅ EMR input form: Keluhan, Diagnosis, Tindakan, Resep, Lampiran
- ✅ EMR form buttons: [Simpan Draft], [Selesaikan EMR], [Batal]
- ✅ EMR finalize: Trigger auto-invoice (per PRD v10)
- ✅ EMR list: Filter by status, date, patient name
- ✅ Profile: Edit email/phone, change password, logout
- ✅ Design: Flat 2.0 SaaS style (same as admin)
- ✅ Responsive: Desktop, tablet, mobile

---

**END OF DESIGN DOCTOR WEB.md**

Version: 1.0 REVISED (Dashboard Removed)  
Tanggal: 2026-07-07  
Status: 100% Sesuai HALAMAN.txt Section 14 (DOCTOR WEB)  
Ready for Hi-Fi Prototype & Frontend Implementation
