# DENTFLOW v4.0 - DESIGN MOBILE.md
## Panduan UI/UX Aplikasi Mobile Pasien (Patient-Facing App)
### "Soft UI Neumorphism + Glassmorphism" Aesthetic Edition

**Versi**: 4.0  
**Target Platform**: iOS & Android (Responsive Web App / PWA)  
**Design Style**: Soft UI Neumorphism + Glassmorphism  
**Desain Filosofi**: Emosional, Menenangkan, Interaktif, Mobile-First dengan Personality  
**Compliance**: WCAG AA Accessibility, Inklusif  

---

## [0] DESIGN AESTHETIC PHILOSOPHY: SOFT UI + GLASSMORPHISM

### 0.1 Core Visual Principles ✨
```
🎨 SOFT UI NEUMORPHISM (Soft Shadows, Gentle Depth)
├─ Shadows soft & blurred (8-12px blur, low opacity)
├─ NO sharp lines or hard borders
├─ Depth dari shadow play, bukan lines
├─ Elevation creates visual hierarchy
└─ Aesthetic: Embossed & elegant, tapi modern

🔮 GLASSMORPHISM (Semi-transparent Layers)
├─ Semi-transparent backgrounds (70-80% opacity)
├─ Backdrop blur effect (10-15px)
├─ Layered, modern feel
└─ Perfect untuk modals, popups, overlays

🌈 GENEROUS SPACING & BREATHING ROOM
├─ Cards dengan margin substantial (20-28px)
├─ Internal padding: 16-20px (relaxed)
├─ White space = calm, not cramped
└─ Visual clarity through negative space

🎪 ROUNDED CORNERS EVERYWHERE
├─ Button radius: 12-16px (soft, friendly)
├─ Card radius: 16-20px (generous)
├─ Input radius: 12px (consistent)
├─ Modal radius: 20-24px (premium feel)
└─ Even icons slightly rounded

🎨 PLAYFUL COLORS & GRADIENTS
├─ Primary: Soft Blue (#3B82F6) atau Teal (#0F766E)
├─ Gradients smooth (Blue→Cyan, Teal→Green)
├─ Warm accent (Orange #F97316) untuk highlights
├─ NO neon or harsh colors
└─ Pastel-ish tapi professional

🎭 MASCOT + PLAYFUL ILLUSTRATIONS
├─ Anthropomorphized tooth character (cute, friendly)
├─ Reduces medical anxiety
├─ Used in empty states, onboarding, success screens
└─ "Healthcare app tapi fun to use"

✨ MICRO-INTERACTIONS (Smooth & Polished)
├─ Smooth transitions (200-300ms ease-in-out)
├─ Hover states subtle (color shift, slight lift)
├─ Loading states with skeleton/shimmer
├─ Feedback immediate (toasts, haptic if mobile)
└─ No jarring or instant changes
```

### 0.2 Kesan Visual yang Diharapkan
- **Warm & Approachable**: Terasa friendly, bukan clinical/cold
- **Premium & Modern**: Glassmorphism = 2024+ trend
- **Calm & Safe**: Soft shadows & generous space = psychological comfort
- **Playful**: Mascot & illustrations = personality
- **Trustworthy**: Clean hierarchy & clear interactions = professional

---

## [1] GLOBAL COLOR PALETTE & ACCESSIBILITY

### 1.1 Warna Utama (Soft UI Optimized)
| Komponen | Hex Code | Penggunaan | Notes |
|----------|----------|-----------|-------|
| **Primary (Soft Blue)** | `#3B82F6` | CTA Buttons, Active Menu, Primary Actions | Warmer dari teal, friendly, modern |
| **Primary Dark** | `#2563EB` | Button Hover, Active States | Deeper, not harsh |
| **Primary Light** | `#DBEAFE` | Backgrounds, Hover States | Subtle, calming |
| **Primary Gradient** | `#3B82F6 → #06B6D4` | Hero Cards, Premium Elements | Smooth blue→cyan |
| **Neutral Dark** | `#334155` | Body Text, Labels | Soft dark (not black) |
| **Neutral Light** | `#F1F5F9` | Page Background | Slightly warmer than #F8FAFC |
| **Surface White** | `#FFFFFF` | Cards, Modals, Inputs | Clean white |
| **Surface Glass** | `rgba(255,255,255,0.8)` | Glassmorphic overlays | 80% opacity |
| **Success** | `#10B981` | Confirmed badges, Success actions | Soft green |
| **Warning** | `#F59E0B` | Pending badges, Warnings | Warm amber |
| **Alert/Danger** | `#EC4899` | Cancel actions, Errors | Soft pink (less scary than red) |
| **Accent** | `#F97316` | Highlights, Mascot, Important info | Warm orange, playful |
| **Border** | `#E2E8F0` | Subtle borders, dividers | Very light gray |
| **Shadow Color** | `rgba(0,0,0,0.08-0.12)` | Soft shadows | Always dark with low opacity |

### 1.2 Shadow System (Neumorphism Tiers)
```css
/* Elevation 1 (Subtle, Cards) */
--shadow-sm: 0 2px 8px rgba(0, 0, 0, 0.08);

/* Elevation 2 (Medium, Higher Cards) */
--shadow-md: 0 4px 16px rgba(0, 0, 0, 0.12);

/* Elevation 3 (Strong, Modals) */
--shadow-lg: 0 8px 24px rgba(0, 0, 0, 0.15);

/* Elevation 4 (Extra, Floating Elements) */
--shadow-xl: 0 12px 32px rgba(0, 0, 0, 0.18);

/* Glassmorphic Backdrop Blur */
--backdrop-blur: blur(10px);
```

### 1.3 Border Radius Scale
```
Button & Input: 12px (small, tight)
Card & Section: 16px (standard, friendly)
Modal & Hero: 20-24px (large, premium)
Pill Badge: 24px+ (round, emphasis)
Icon Wrapper: 8-10px (slightly rounded)
```

### 1.4 Spacing Scale (Soft UI = Generous)
```
4px  = tiny gaps
8px  = small spacing
12px = compact items
16px = standard padding (cards, inputs)
20px = card margins (generous)
24px = section spacing (breathing room)
32px = major section gaps
```

---

## [2] ARSITEKTUR MOBILE APP PASIEN

### 2.1 Struktur Navigasi Utama: PERSISTENT BOTTOM TAB BAR (4 Menu)

```
┌──────────────────────────────────────┐
│  Content Area (Soft UI Cards)        │
│  ├─ Generous spacing (20-24px)       │
│  ├─ Soft shadows (elevation aware)   │
│  ├─ Rounded corners everywhere       │
│  └─ Smooth interactions              │
│                                      │
│                                      │
├──────────────────────────────────────┤
│ [🏠 Home] [📅 Bookings] [📄 EMR] [👤 Profile]
│   (Fixed Bottom Bar with soft shadow)
└──────────────────────────────────────┘
```

**4 Menu Items (Bottom Tab Bar - Soft UI Style):**

| Icon | Label | Route | Style Notes |
|------|-------|-------|-------------|
| 🏠 | **Home** | `/patient/home` | Soft blue when active, gray when inactive |
| 📅 | **Bookings** | `/patient/bookings` | Smooth transition on tap (200ms) |
| 📄 | **EMR** | `/patient/encounters` | Consistent icon sizing (24px) |
| 👤 | **Profile** | `/patient/profile` | Rounded corners on tab bar (20px radius) |

**Bottom Bar Styling:**
- Background: White (#FFFFFF)
- Shadow: Soft elevation 3 (0 8px 24px rgba(0,0,0,0.12))
- Padding: 12px horizontal, 8px top, 8px bottom + safe area
- Active color: Primary Blue (#3B82F6)
- Inactive color: Gray (#94A3B8)
- Smooth transition: 200ms ease-in-out

---

## [3] HALAMAN 1: HOME (`/patient/home`)

### 3.1 Layout & Visual Hierarchy (Soft UI)

```
HEADER SECTION (Sticky, Soft Shadow)
├─ Background: White (#FFFFFF)
├─ Shadow: Elevation 1 (soft, subtle)
├─ Padding: 16px (top/sides)
├─ Greeting: "Halo, Floyd!" (Bold, 20px, #334155)
├─ Subtitle: "Senin, 5 Juli 2026" (Small, 12px, #94A3B8)
└─ Notification Bell: Top-right corner (icon with small badge if any)

CONTENT AREA (Full Width, Scrollable)
├─ Background: Light gray (#F1F5F9)
├─ Padding: 16px sides, 20px top/bottom per card
└─ Card spacing: 20px between cards (generous)

[SECTION 1] UPCOMING APPOINTMENT CARD
├─ Card Style:
│  ├─ Background: White (#FFFFFF)
│  ├─ Shadow: Elevation 2 (0 4px 16px rgba(0,0,0,0.12))
│  ├─ Radius: 16px
│  ├─ Padding: 16px
│  ├─ Border-Left: 4px solid #3B82F6 (primary accent)
│  └─ Hover: Slight lift (shadow increase), no color change
│
├─ Content Layout:
│  ├─ Heading: "Janji Temu Berikutnya" (#334155, 14px, bold)
│  ├─ Doctor Card (Embedded):
│  │  ├─ Avatar: 48x48px circular, soft shadow
│  │  ├─ Name: "Dr. Sarah Williams, DMD" (16px, bold)
│  │  ├─ Spec: "Cosmetic Dentistry" (12px, gray)
│  │  └─ Rating: "⭐ 4.9 (245 reviews)" (small)
│  │
│  ├─ Details Grid (2 columns):
│  │  ├─ Date: "5 Juli 2026"
│  │  ├─ Time: "10:30 AM"
│  │  ├─ Branch: "Jakarta Pusat"
│  │  └─ Countdown: "2 hari lagi" (accent color #F97316)
│  │
│  ├─ Status Badge: "Confirmed" (#10B981 soft)
│  └─ CTA Button: "Lihat Detail" (Ghost style, soft border)
│
└─ Interaction:
   ├─ Tap: Navigate to `/patient/bookings/:booking_id`
   └─ Hover: Soft lift + shadow increase (not jarring)

[SECTION 2] EMPTY STATE (If no upcoming bookings)
├─ Card Style:
│  ├─ Background: Gradient (#DBEAFE light blue)
│  ├─ Radius: 16px
│  ├─ Padding: 32px (generous, centered)
│  └─ Shadow: Elevation 1 (very subtle)
│
├─ Content:
│  ├─ Illustration: Playful tooth character (64x64px, SVG)
│  ├─ Heading: "Tidak Ada Janji Temu" (#334155, bold)
│  ├─ Sub-text: "Mulai dengan membuat janji temu baru" (#94A3B8)
│  └─ CTA Button: [Buat Janji Temu Baru] (Primary blue, round)
│
└─ Tone: Friendly, encouraging, not sad

[SECTION 3] RECENT APPOINTMENTS (Last 3)
├─ Header: "Riwayat Terakhir" (14px, bold, #334155)
├─ List Items (Soft Cards):
│  ├─ Card Style:
│  │  ├─ Background: White
│  │  ├─ Shadow: Elevation 1
│  │  ├─ Radius: 12px
│  │  ├─ Padding: 12px
│  │  └─ Margin-bottom: 12px
│  │
│  └─ Content (Horizontal):
│     ├─ Left (70%):
│     │  ├─ Doctor name + date (14px, bold)
│     │  └─ Status badge (small, soft)
│     │
│     └─ Right (30%):
│        └─ Chevron icon (gray, 24px)
│
├─ Link: "Lihat Semua" (Bottom, accent color #3B82F6)
└─ Tap: Navigate to `/patient/bookings`

[SECTION 4] FLOATING ACTION BUTTON (FAB)
├─ Position: Bottom-right, 20px from edges
├─ Size: 56px (iOS) / 64px (Android)
├─ Background: Primary Gradient (#3B82F6 → #06B6D4)
├─ Icon: "+" (White, 24px)
├─ Shadow: Elevation 4 (0 12px 32px rgba(0,0,0,0.18))
├─ Border-radius: Full circle (50%)
└─ Interaction:
   ├─ Tap: Navigate to `/patient/booking?step=1`
   ├─ Hover: Slight scale-up (1.1x), shadow increase
   └─ Animation: Smooth, 200ms ease-out
```

### 3.2 Component Details (Soft UI Styling)

#### 3.2.1 Doctor Card (Upcoming Appointment)
```
Layout: Horizontal card with avatar on left
┌─────────────────────────────────┐
│ [Avatar] | Doctor Name, Spec    │
│          | Rating & Reviews      │
│          | [Chevron →]           │
└─────────────────────────────────┘

Styling:
- Avatar: 48x48px circular, soft shadow 0 2px 8px
- Name: 16px bold #334155
- Spec: 12px gray #94A3B8
- Rating: Small, accent color
- Border-radius: 12px (card)
- Padding: 12px
- Background: White
- On-tap: Highlight with border-color change
- Transition: 200ms ease-in-out
```

#### 3.2.2 Status Badge (Soft UI Version)
```css
.badge {
  display: inline-block;
  padding: 6px 12px;
  border-radius: 12px;  /* More rounded for soft feel */
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

/* Soft colors, not harsh */
.badge.confirmed { 
  background: rgba(16, 185, 129, 0.15);  /* 15% opacity green */
  color: #059669;  /* Darker green for readability */
}

.badge.pending { 
  background: rgba(245, 158, 11, 0.15);  /* 15% opacity amber */
  color: #92400E;  /* Darker amber */
}

.badge.cancelled { 
  background: rgba(236, 72, 153, 0.15);  /* 15% opacity pink */
  color: #BE185D;  /* Darker pink, not scary red */
}
```

#### 3.2.3 Section Spacing
- **Top padding**: 20px (generous)
- **Bottom padding**: 24px (breathing room)
- **Card margin-bottom**: 20px (not cramped)
- **Internal card padding**: 16px (comfortable)

---

## [4] HALAMAN 2: BOOKINGS (`/patient/bookings`)

### 4.1 Layout: Soft UI List View

```
HEADER (Sticky, Soft Shadow)
├─ Title: "Janji Temu Saya" (24px, bold)
├─ Subtitle: "Kelola dan buat janji temu baru" (12px, gray)
├─ Background: White
├─ Shadow: Elevation 1
└─ Padding: 16px

FILTER CHIPS (Scrollable, below header)
├─ Layout: Horizontal scroll, with padding
├─ Chips Style:
│  ├─ Inactive: White bg, border 1px #E2E8F0, gray text
│  ├─ Active: Primary gradient bg, white text
│  ├─ Radius: 12px
│  ├─ Padding: 8px 16px
│  ├─ Transition: 200ms ease-in
│  └─ Fonts: 12px, semi-bold
│
├─ Chips: [All] [Menunggu Bayar] [Terkonfirmasi] [Selesai] [Dibatalkan]
└─ Interaction: Tap to filter list (smooth animation)

CONTENT AREA (Scrollable, Full Height)
├─ Background: Light gray (#F1F5F9)
├─ Cards spacing: 16px between items
├─ Padding: 16px sides

[BOOKING ITEM CARD] (Soft UI Style)
├─ Card Style:
│  ├─ Background: White
│  ├─ Shadow: Elevation 2 (soft, not aggressive)
│  ├─ Radius: 16px
│  ├─ Padding: 14px
│  ├─ Margin-bottom: 16px
│  └─ Hover: Slight lift, shadow increase
│
├─ Layout: Horizontal
│  ├─ Left (70%):
│  │  ├─ Doctor name (14px, bold, #334155)
│  │  ├─ Clinic name (12px, gray)
│  │  ├─ Date & Time (13px, bold, primary #3B82F6)
│  │  └─ Status badge (small, soft colors)
│  │
│  └─ Right (30%):
│     ├─ Chevron icon (24px, light gray)
│     └─ Subtle gradient on hover
│
└─ Interaction:
   ├─ Tap: Navigate to `/patient/bookings/:booking_id`
   ├─ Swipe (optional): Reveal cancel action
   └─ Animation: Smooth, 200ms

[EMPTY STATE CARD]
├─ Background: Soft gradient (#DBEAFE)
├─ Radius: 16px
├─ Padding: 40px 24px
├─ Content: Centered
│  ├─ Icon: Playful tooth mascot (48x48px)
│  ├─ Text: "Tidak ada janji temu" (#334155, bold)
│  └─ CTA: [Buat Sekarang] (primary blue)
│
└─ Tone: Friendly, encouraging

[FLOATING ACTION BUTTON]
├─ Position: Bottom-right, 20px from edges
├─ Gradient: Primary blue→cyan
├─ Shadow: Elevation 4
├─ Interaction: Same as Home FAB
└─ Animation: 200ms ease-out
```

### 4.2 Detail Booking (`/patient/bookings/:booking_id`)

```
HEADER (Sticky Back Button)
├─ Back button: Left-aligned, icon only
├─ Title: "Detail Janji Temu" (18px, bold)
├─ Background: White
└─ Shadow: Elevation 1

CONTENT AREA

[SECTION 1] DOCTOR CARD (Prominent)
├─ Card Style:
│  ├─ Background: White
│  ├─ Shadow: Elevation 2
│  ├─ Radius: 16px
│  ├─ Padding: 16px
│  └─ Margin-bottom: 20px
│
├─ Content:
│  ├─ Avatar: 64x64px circular, soft shadow
│  ├─ Name: "Dr. Sarah Williams, DMD" (18px, bold)
│  ├─ Specialization: "Cosmetic Dentistry" (12px, gray)
│  └─ Rating: "⭐ 4.9 (245 reviews)" (small, accent)
│
└─ Layout: Centered

[SECTION 2] APPOINTMENT DETAILS
├─ Card Style:
│  ├─ Background: White
│  ├─ Border-left: 4px solid #3B82F6
│  ├─ Radius: 16px
│  ├─ Padding: 16px
│  └─ Shadow: Elevation 1
│
├─ Grid Layout (2 columns):
│  ├─ Label (12px, gray) : Value (14px, bold)
│  ├─ Date    : 5 Juli 2026
│  ├─ Time    : 10:30 AM
│  ├─ Branch  : Jakarta Pusat
│  ├─ Room    : Ruang 3
│  └─ Code    : BOOKING-12345
│
└─ Copy button: "Salin Kode" (secondary button, 12px text)

[SECTION 3] ACTION BUTTONS (Conditional)
├─ If Pending:
│  ├─ [Konfirmasi] (Primary, blue, 100% width)
│  └─ [Batalkan] (Ghost, pink border, 100% width)
│
├─ If Confirmed:
│  ├─ [Reminder: ON/OFF] (Toggle button)
│  └─ [Batalkan] (Ghost, pink border)
│
└─ If Completed:
   ├─ [Lihat Catatan Medis] (Secondary, blue)
   └─ [Buat Janji Baru] (Primary, blue)

Button Styling (Soft UI):
- Primary: Background #3B82F6, white text, radius 12px
- Ghost: Border 1px #E2E8F0, primary text, radius 12px
- Hover: Slight lift, shadow increase, color deepening
- Padding: 12px 16px (relaxed)
- Transition: 200ms ease-in-out

[SECTION 4] PAYMENT STATUS
├─ Card Style:
│  ├─ Background: #DBEAFE (very light blue)
│  ├─ Radius: 16px
│  ├─ Padding: 16px
│  ├─ Border-left: 4px solid #3B82F6
│  └─ Shadow: Elevation 1
│
├─ Content:
│  ├─ Label: "Status Pembayaran" (12px, bold)
│  ├─ Amount: "DP Rp 50.000" (16px, bold, #334155)
│  ├─ Badge: "Pending" atau "Paid" (soft colors)
│  ├─ Payment method: "Midtrans / Cash"
│  └─ CTA: [Bayar Sekarang] (primary button, if pending)
│
└─ Tone: Friendly, informative

[SECTION 5] REMINDER INFO (Info Card)
├─ Background: #CCFBF1 (very light teal) atau #E0F2FE (light blue)
├─ Border-left: 3px solid #3B82F6
├─ Radius: 12px
├─ Padding: 12px
├─ Content: "Reminder akan dikirim via WhatsApp & notifikasi" (12px)
└─ Icon: Bell icon (12px, primary color)
```

### 4.3 Multi-Step Booking Flow (`/patient/booking`)

#### 4.3.1 Step 1: Pilih Cabang

```
PROGRESS INDICATOR (Top, Sticky)
├─ Style: Linear progress bar (soft)
├─ Fill: 25% (1 of 4 steps)
├─ Color: Primary gradient (#3B82F6 → #06B6D4)
├─ Height: 4px (subtle)
└─ Animation: Smooth fill on step change

HEADER
├─ Title: "Pilih Cabang Klinik" (20px, bold)
├─ Subtitle: "Dimana Anda ingin berkunjung?" (14px, gray)
└─ Padding: 16px

CONTENT (Clinic Cards)
├─ Card Style:
│  ├─ Background: White
│  ├─ Shadow: Elevation 2
│  ├─ Radius: 16px
│  ├─ Padding: 16px
│  ├─ Margin-bottom: 16px
│  ├─ Border: 2px solid transparent (changes on select)
│  └─ Hover: Border color shift, shadow increase
│
├─ Content per Card:
│  ├─ Clinic name (14px, bold)
│  ├─ Address (12px, gray)
│  ├─ Hours: "Buka 08:00 - 17:00" (11px, small)
│  ├─ Distance: "≈ 2.3 km" (11px, accent color)
│  └─ Selection indicator: Subtle gradient background on select
│
└─ Interaction:
   ├─ Tap: Select clinic, border becomes primary blue
   ├─ Animation: Smooth color transition, 200ms
   └─ Enable Next button only after selection

BUTTON AREA (Sticky Bottom)
├─ [← Back] (Ghost, disabled/opacity 50%)
└─ [Next →] (Primary blue, enabled after selection)
```

#### 4.3.2 Step 2: Pilih Dokter

```
PROGRESS INDICATOR
├─ Fill: 50% (2 of 4 steps)
└─ Animation: Smooth fill

HEADER
├─ Title: "Pilih Dokter" (20px, bold)
├─ Subtitle: "Spesialis apa yang Anda butuhkan?" (14px, gray)

FILTER CHIPS (Scrollable)
├─ Style: Horizontal scroll
├─ Active: Primary gradient (#3B82F6 → #06B6D4), white text
├─ Inactive: White bg, border, gray text
├─ Chips: [All] [Gigi Umum] [Periodonsia] [Orthodontia] [Endodonsia]

SEARCH INPUT (Optional)
├─ Placeholder: "Cari nama dokter..." (12px, gray)
├─ Radius: 12px
├─ Padding: 12px 14px
├─ Border: 1px solid #E2E8F0
├─ Focus: Border becomes primary blue, subtle shadow

DOCTOR CARDS (List or 2-column grid)
├─ Card Style:
│  ├─ Background: White
│  ├─ Shadow: Elevation 2
│  ├─ Radius: 16px
│  ├─ Padding: 16px
│  ├─ Margin-bottom: 16px
│  └─ Border: 2px solid transparent (changes on select)
│
├─ Content:
│  ├─ Avatar: 48x48px circular, soft shadow
│  ├─ Name: "Dr. Sarah Williams, DMD" (14px, bold)
│  ├─ Specialization: "Cosmetic Dentistry" (12px, gray)
│  ├─ Rating: "⭐ 4.9 (245 reviews)" (small)
│  ├─ Availability: "Available today" (11px, green) atau "Fully booked" (11px, gray)
│  └─ Price: "Rp 200.000 / konsultasi" (11px, gray)
│
└─ Interaction:
   ├─ Tap: Select doctor, border becomes primary
   ├─ Animation: Smooth, 200ms
   └─ Enable Next only after selection

BUTTON AREA (Sticky Bottom)
├─ [← Back] (Ghost, enabled)
└─ [Next →] (Primary blue, enabled after selection)
```

#### 4.3.3 Step 3: Pilih Tanggal & Waktu

```
PROGRESS INDICATOR
├─ Fill: 75% (3 of 4 steps)

HEADER
├─ Title: "Pilih Tanggal & Waktu" (20px, bold)
├─ Subtitle: "Kapan Anda bisa datang?" (14px, gray)

[SECTION A] CALENDAR PICKER
├─ Card Style:
│  ├─ Background: White
│  ├─ Shadow: Elevation 2
│  ├─ Radius: 16px
│  ├─ Padding: 16px
│  └─ Margin-bottom: 20px
│
├─ Calendar Widget:
│  ├─ Month header: "[< Prev] Jul 2026 [Next >]" (centered)
│  ├─ Day cells:
│  │  ├─ Available: White bg, gray text, radius 8px
│  │  ├─ Selected: Primary blue gradient, white text
│  │  ├─ Unavailable: Light gray, strikethrough
│  │  └─ Today: Border 2px primary blue
│  │
│  └─ Interaction:
│     ├─ Tap available date: Select, update display
│     └─ Animation: Smooth color transition, 150ms
│
└─ Selected date display: "[5 Juli 2026]" (14px, bold, primary)

[SECTION B] TIME SLOT PICKER
├─ Card Style:
│  ├─ Background: White
│  ├─ Shadow: Elevation 2
│  ├─ Radius: 16px
│  ├─ Padding: 16px
│  └─ Margin-bottom: 20px
│
├─ Header: "Pilih jam konsultasi" (12px, bold)
├─ Time slot grid (2-3 columns, responsive):
│  ├─ Slot Style:
│  │  ├─ Available: White bg, border 1px #E2E8F0, radius 12px, padding 12px
│  │  ├─ Selected: Primary gradient bg, white text
│  │  ├─ Unavailable: Light gray, opacity 50%
│  │  └─ Interaction: Smooth transition, 150ms
│  │
│  └─ Slots: [08:00] [09:00] [10:00] [11:00] ... [17:00]
│
└─ Info text: "Tersisa 3 slot kosong untuk tanggal ini" (11px, gray, accent)

BUTTON AREA (Sticky Bottom)
├─ [← Back] (Ghost, enabled)
└─ [Next →] (Primary blue, enabled after date & time selection)
```

#### 4.3.4 Step 4: Review & Bayar

```
PROGRESS INDICATOR
├─ Fill: 100% (4 of 4 steps)

HEADER
├─ Title: "Konfirmasi Janji Temu" (20px, bold)
├─ Subtitle: "Periksa detail sebelum membayar" (14px, gray)

[CARD 1] SUMMARY DETAILS
├─ Card Style:
│  ├─ Background: White
│  ├─ Shadow: Elevation 2
│  ├─ Radius: 16px
│  ├─ Padding: 16px
│  └─ Margin-bottom: 16px
│
├─ Content (2-column layout):
│  ├─ Label (12px, bold) : Value (14px)
│  ├─ Doctor    : Dr. Sarah Williams, DMD
│  ├─ Specialty : Cosmetic Dentistry
│  ├─ Clinic    : Jakarta Pusat
│  ├─ Date      : 5 Juli 2026
│  ├─ Time      : 10:30 AM
│  └─ Fee       : Rp 200.000
│
└─ Edit link: "Ubah" (small, accent color, top-right)

[CARD 2] PATIENT DATA
├─ Card Style: Same as above
├─ Content:
│  ├─ Name   : Floyd Miles
│  ├─ Phone  : (021) 1234-5678
│  ├─ Email  : floyd@example.com
│  └─ DoB    : 15 Januari 1990
│
└─ Edit link: "Ubah Profil" (small, accent color)

[CARD 3] PAYMENT SUMMARY
├─ Card Style:
│  ├─ Background: #DBEAFE (light blue)
│  ├─ Border-left: 4px solid #3B82F6
│  ├─ Radius: 16px
│  ├─ Padding: 16px
│  └─ Shadow: Elevation 1
│
├─ Content:
│  ├─ Consultation fee: Rp 200.000 (11px, gray)
│  ├─ DP Booking     : Rp 50.000 (12px, gray)
│  ├─ Divider line   : 1px #E2E8F0
│  └─ TOTAL          : Rp 50.000 (16px, bold, #3B82F6)
│
└─ Note: ⚠️ **PENTING:** DP Rp 50.000 bersifat **NON-REFUNDABLE**. 
	Jika batal atau tidak hadir (No-Show), DP tidak dapat 
	dikembalikan dengan alasan apapun.
	(Background: #FEF2F2, Border-Left: 4px solid #DC2626)

[SECTION] T&C CHECKBOX
├─ Checkbox: "Saya setuju dengan syarat & ketentuan"
├─ Link: "Baca selengkapnya" (small, accent color, underline)
└─ Required: Must check before payment

BUTTON AREA (Sticky Bottom)
├─ [← Back] (Ghost, enabled)
└─ [Bayar Sekarang] (Primary gradient, enabled after T&C check)

POST-PAYMENT SUCCESS SCREEN:
├─ Illustration: Checkmark icon (64x64px, green, circular bg)
├─ Heading: "Pembayaran Berhasil!" (18px, bold, #10B981)
├─ Sub-text: "Janji temu Anda sudah dikonfirmasi" (12px, gray)
├─ Auto-dismiss: 3 seconds
└─ Auto-navigate: to `/patient/home` with success banner
```

---

## [5] HALAMAN 3: EMR HISTORY (`/patient/encounters`)

### 5.1 List View (Soft UI)

```
HEADER
├─ Title: "Riwayat Kunjungan" (24px, bold)
├─ Subtitle: "Lihat catatan medis Anda" (12px, gray)
└─ Shadow: Elevation 1

FILTER/SEARCH (Sticky)
├─ Search input: "Cari tanggal atau dokter..."
├─ Filter chips: [All] [2026] [2025] [Older]

CONTENT AREA
├─ Background: Light gray (#F1F5F9)
├─ Padding: 16px

[GROUP] "Juli 2026"
├─ Grouped heading: "Juli 2026" (11px, bold, gray, sticky section header)

[ENCOUNTER CARD] (Soft UI Style)
├─ Card Style:
│  ├─ Background: White
│  ├─ Shadow: Elevation 2
│  ├─ Radius: 16px
│  ├─ Padding: 14px
│  ├─ Margin-bottom: 12px
│  └─ Border: 1px solid #F1F5F9 (very subtle)
│
├─ Content (Horizontal):
│  ├─ Left (70%):
│  │  ├─ Doctor name + date: "Dr. Budi S. - 5 Juli 2026" (13px, bold)
│  │  ├─ Time: "10:30" (11px, gray)
│  │  └─ Summary: "Konsultasi gigi berlubang, penambalan." (12px, gray, truncated)
│  │
│  └─ Right (30%):
│     ├─ Status badge: "Selesai" (soft gray)
│     └─ Chevron icon (24px, light gray)
│
└─ Interaction:
   ├─ Tap: Navigate to `/patient/encounters/:encounter_id`
   └─ Animation: Smooth, 200ms

[EMPTY STATE]
├─ Card Style:
│  ├─ Background: Gradient (#DBEAFE light blue)
│  ├─ Radius: 16px
│  ├─ Padding: 40px 24px
│  └─ Shadow: Elevation 1
│
├─ Content: Centered
│  ├─ Icon: Document icon (48x48px, primary color)
│  ├─ Text: "Belum ada riwayat kunjungan" (#334155, bold)
│  └─ Sub-text: "Janji temu Anda akan muncul di sini setelah selesai" (#94A3B8)
│
└─ Tone: Friendly, informative
```

### 5.2 Detail Encounter (`/patient/encounters/:encounter_id`)

```
HEADER (Back Button)
├─ Back button: Icon only, soft color
├─ Title: "Detail Kunjungan" (18px, bold)
└─ Shadow: Elevation 1

[SECTION 1] DOCTOR INFO
├─ Card Style:
│  ├─ Background: White
│  ├─ Shadow: Elevation 2
│  ├─ Radius: 16px
│  ├─ Padding: 16px
│  └─ Margin-bottom: 20px
│
├─ Content (Centered):
│  ├─ Avatar: 64x64px circular
│  ├─ Name: "Dr. Budi Santoso, SpGP" (18px, bold)
│  ├─ Specialization: "Gigi Umum" (12px, gray)
│  ├─ Date/Time: "5 Juli 2026 | 10:30" (11px, gray)
│  └─ Clinic: "DentFlow Jakarta Pusat" (11px, gray)
│
└─ Layout: Vertical, centered

[SECTION 2] CHIEF COMPLAINT (Keluhan Utama)
├─ Card Style:
│  ├─ Background: #F1F5F9 (light gray)
│  ├─ Radius: 12px
│  ├─ Padding: 12px
│  └─ Margin-bottom: 16px
│
├─ Content:
│  ├─ Label: "Keluhan Utama" (11px, bold, gray)
│  └─ Text: "Gigi belakang kanan terasa sakit saat mengunyah" (13px, #334155)
│
└─ Style: Read-only, soft background

[SECTION 3] ASSESSMENT (Penilaian Klinis)
├─ Card Style:
│  ├─ Background: White
│  ├─ Shadow: Elevation 1
│  ├─ Radius: 12px
│  ├─ Padding: 12px
│  └─ Margin-bottom: 16px
│
├─ Content (Collapsible sections):
│  ├─ [▸] Anamnesis (Riwayat Penyakit)
│  ├─ [▸] Objektif (Pemeriksaan Fisik)
│  ├─ [▸] Asesmen / Diagnosis (⚠️ HIDDEN dari patient untuk privasi)
│  └─ [▸] Rencana Tindakan
│
└─ Interaction: Tap to expand, smooth animation

[SECTION 4] TREATMENT PERFORMED (Tindakan)
├─ Card Style: Same as assessment
├─ Content:
│  ├─ Label: "Tindakan yang Dilakukan" (11px, bold)
│  ├─ List:
│  │  ├─ • Scaling & Polishing
│  │  ├─ • Penambalan resin
│  │  └─ • Fluoride application
│  │
│  └─ Style: Bullet list, soft background (#F1F5F9)
│
└─ Margin-bottom: 16px

[SECTION 5] PRESCRIPTION (Resep Obat)
├─ Card Style:
│  ├─ Background: White
│  ├─ Border-left: 3px solid #3B82F6
│  ├─ Radius: 12px
│  ├─ Padding: 12px
│  └─ Margin-bottom: 16px
│
├─ Content per medicine:
│  ├─ Name: "Paracetamol 500mg" (13px, bold)
│  ├─ Dosage: "3x sehari, 1 tablet" (12px, gray)
│  ├─ Duration: "Selama 5 hari" (11px, gray)
│  └─ Notes: "Jangan diminum saat perut kosong" (11px, italic, gray)
│
└─ Empty case: "Tidak ada obat yang diresepkan"

[SECTION 6] RADIOLOGICAL FILES (Rontgen)
├─ Card Style: White, Elevation 1, padding 12px
├─ Content per file:
│  ├─ Thumbnail: 80x80px preview image
│  ├─ File name: "Panoramic X-Ray" (13px, bold)
│  ├─ Date: "5 Juli 2026" (11px, gray)
│  ├─ Size: "2.3 MB" (11px, gray)
│  └─ CTA: [Download] (secondary button, 12px)
│
└─ Empty case: "Tidak ada file rontgen untuk kunjungan ini"

[SECTION 7] FOLLOW-UP INFO
├─ Card Style:
│  ├─ Background: #E0F2FE (light blue)
│  ├─ Border-left: 4px solid #3B82F6
│  ├─ Radius: 12px
│  ├─ Padding: 12px
│  └─ Margin-bottom: 16px
│
├─ Content:
│  ├─ Icon: Bell + smile emoji
│  ├─ Text: "Hindari makanan keras selama 2 hari..." (13px, #334155)
│  └─ CTA: [Buat Janji Lanjutan] (secondary button)
│
└─ Tone: Friendly, instructional

[SECTION 8] REFERENCE
├─ Encounter ID: "ENC-2026-07-0001" (11px, monospace, gray)
├─ Copy button: "Salin ID" (small link, accent color)
└─ Print button: "Cetak Laporan" (small link, accent color)
```

---

## [6] HALAMAN 4: PROFILE (`/patient/profile`)

### 6.1 Layout (Soft UI Settings Page)

```
HEADER
├─ Title: "Profil Saya" (24px, bold)
├─ Subtitle: "Kelola informasi personal & akun" (12px, gray)
└─ Shadow: Elevation 1

CONTENT AREA (Scrollable)
├─ Background: Light gray (#F1F5F9)
└─ Padding: 16px

[SECTION 1] PROFILE PICTURE & BASIC INFO
├─ Card Style:
│  ├─ Background: White
│  ├─ Shadow: Elevation 2
│  ├─ Radius: 16px
│  ├─ Padding: 16px
│  └─ Margin-bottom: 20px
│
├─ Content: Centered
│  ├─ Avatar: 96x96px circular
│  ├─ Edit button: Pencil icon (position: bottom-right of avatar)
│  ├─ Name: "Floyd Miles" (18px, bold)
│  ├─ Phone: "(021) 1234-5678" (12px, gray)
│  └─ Member since: "Anggota sejak 3 Januari 2023" (11px, gray)
│
└─ Interaction: Tap pencil → image picker

[SECTION 2] PERSONAL INFORMATION
├─ Card Style: White, Elevation 1, padding 16px
├─ Form fields (each with soft styling):
│  ├─ Input style:
│  │  ├─ Border: 1px #E2E8F0
│  │  ├─ Radius: 12px
│  │  ├─ Padding: 12px 14px
│  │  ├─ Focus: Border #3B82F6, shadow 0 0 0 3px rgba(59,130,246,0.1)
│  │  └─ Transition: 150ms ease-in-out
│  │
│  ├─ Full Name, Date of Birth, Gender, Phone, Email, Address
│  ├─ Allergy Information (multiline, important)
│  └─ Label per field (11px, bold, gray)
│
├─ Unsaved indicator: "Perubahan belum disimpan" (small, warn color)
└─ Button: [Simpan Perubahan] (primary blue, 100% width, 12px top margin)

[SECTION 3] SECURITY & PASSWORD
├─ Card Style: White, Elevation 1, padding 16px
├─ Items (list-style):
│  ├─ Item format:
│  │  ├─ Left: Title (13px, bold)
│  │  ├─ Right: Link atau toggle
│  │  └─ Divider: 1px #F1F5F9 between items
│  │
│  ├─ [Ganti Password]
│  │  ├─ Current setting: "Terakhir diubah 6 bulan lalu" (11px, gray)
│  │  └─ Tap: Navigate to `/patient/profile/change-password`
│  │
│  ├─ [Two-Factor Authentication]
│  │  ├─ Toggle: Off/On
│  │  └─ Description: "Lindungi akun Anda dengan autentikasi dua faktor" (11px, gray)
│  │
│  └─ [Login Activity]
│     └─ Tap: Show modal with recent logins

└─ Margin-bottom: 20px

[SECTION 4] PREFERENCES
├─ Card Style: White, Elevation 1, padding 16px
├─ Items: Toggles + Dropdowns
│  ├─ Push Notifications (Toggle, default ON)
│  ├─ Email Notifications (Toggle, default ON)
│  ├─ SMS Reminders (Toggle, default OFF)
│  └─ Language (Dropdown, default Bahasa Indonesia)
│
└─ Description text per item (11px, gray)

[SECTION 5] DATA & PRIVACY
├─ Card Style: White, Elevation 1, padding 16px
├─ Items:
│  ├─ [Privacy Policy] Link
│  ├─ [Terms of Service] Link
│  ├─ [Export Data] Button (secondary)
│  └─ [Delete Account] Button (pink/danger border)
│
└─ Margin-bottom: 20px

[SECTION 6] LOGOUT
├─ Button: [Keluar dari Akun]
├─ Style: Full-width, ghost style, gray text
├─ Action: Confirm logout → clear tokens → login screen
└─ Bottom padding: 40px (safe area)
```

---

## [7] GLOBAL INTERACTION PATTERNS (Soft UI)

### 7.1 Toast Notifications (Non-blocking)
```
Position: Bottom-center, 20px from bottom
Duration: 3 seconds (auto-dismiss)
Animation: Slide-up (entrance), slide-down (exit), 200ms ease

[SUCCESS TOAST]
├─ Background: #DCFCE7 (light green)
├─ Border-left: 4px solid #10B981
├─ Icon: ✓ (green)
├─ Text: "Janji temu dibatalkan" (12px, #065F46)
└─ Shadow: Elevation 2 (soft)

[ERROR TOAST]
├─ Background: #FECACA (light red, not harsh)
├─ Border-left: 4px solid #EC4899 (pink, not red)
├─ Icon: ✗ (pink)
├─ Text: "Terjadi kesalahan. Silakan coba lagi." (12px, #831843)
└─ Shadow: Elevation 2

[WARNING TOAST]
├─ Background: #FEF3C7 (light amber)
├─ Border-left: 4px solid #F59E0B
├─ Icon: ⚠️
├─ Text: "Slot waktu hampir penuh" (12px, #92400E)
└─ Shadow: Elevation 2

[INFO TOAST]
├─ Background: #E0F2FE (light blue)
├─ Border-left: 4px solid #3B82F6
├─ Icon: ℹ️
├─ Text: "Reminder akan dikirim 1 hari sebelumnya" (12px, #0C4A6E)
└─ Shadow: Elevation 2
```

### 7.2 Modals (Glassmorphic Style)
```
OVERLAY
├─ Background: rgba(0, 0, 0, 0.4) (softer than 50%)
├─ Backdrop blur: 10px (glassmorphic effect)
└─ Tap outside: Close modal

MODAL DIALOG
├─ Background: White (#FFFFFF)
├─ Backdrop blur: NO (dialog itself is white, not glass)
├─ Radius: 20px (large, premium)
├─ Max width: 320px (mobile), 400px (tablet)
├─ Shadow: Elevation 4 (0 12px 32px rgba(0,0,0,0.18))
├─ Padding: 24px
├─ Animation: Scale 0.9→1.0 + fade-in (200ms ease-out)

MODAL HEADER (Optional Icon)
├─ Icon: 48x64px centered (e.g., ⚠️, ✓, ?)
├─ Title: 18px, bold, #334155
├─ Subtitle: 14px, gray (optional)
└─ Margin-bottom: 16px

MODAL BODY
├─ Text color: #334155 (primary), #94A3B8 (secondary)
└─ Line height: 1.5 (relaxed, readable)

MODAL FOOTER (Buttons)
├─ Stack: Vertical on mobile, horizontal on tablet
├─ Gap between buttons: 12px
├─ Primary action: Bottom-most (mobile) or right-most (tablet)
└─ Min height per button: 44px (touch target)
```

### 7.3 Loading States (Soft UI)
```
SKELETON LOADING (Preferred)
├─ Use shimmer animation (gradient shift 1.5s loop)
├─ Colors:
│  ├─ Base: #F1F5F9
│  ├─ Shimmer: White → Light gray → White
│  └─ Border-radius: Match actual content
│
└─ Example: Skeleton card, skeleton text lines

SPINNER (Small Loads)
├─ Style: Circular SVG, smooth rotation
├─ Color: Primary blue #3B82F6
├─ Size: 32x32px (default), 24x24px (small), 48x48px (large)
├─ Animation: Linear rotation, 1.2s per turn
└─ Placement: Center of container

PROGRESS BAR (Multi-step Booking)
├─ Background: #F1F5F9
├─ Progress fill: Primary gradient (#3B82F6 → #06B6D4)
├─ Height: 4px (subtle)
├─ Border-radius: 2px
└─ Animation: Smooth width transition, 300ms ease
```

### 7.4 Form Validation (Soft UI)
```
VALID INPUT
├─ Border: 1px #10B981 (green, soft)
├─ Focus shadow: 0 0 0 3px rgba(16, 185, 129, 0.1)
├─ Icon: ✓ (green, right side)
└─ Message: "Terlihat bagus!" (11px, #065F46, optional)

INVALID INPUT
├─ Border: 1px #EC4899 (pink, not harsh red)
├─ Focus shadow: 0 0 0 3px rgba(236, 72, 153, 0.1)
├─ Icon: ✗ (pink, right side)
└─ Message: "Email tidak valid" (11px, #831843, below field)

FOCUSED INPUT
├─ Border: 2px #3B82F6
├─ Shadow: 0 0 0 3px rgba(59, 130, 246, 0.1)
└─ Transition: 150ms ease-in-out
```

### 7.5 Empty States (Soft UI & Friendly)
```
EMPTY STATE CARD
├─ Background: Gradient (#DBEAFE light blue) atau solid white
├─ Border: None (or 1px very light gray)
├─ Radius: 16px
├─ Padding: 40px 24px (generous)
├─ Shadow: Elevation 1 (soft)
├─ Text-align: Center

CONTENT
├─ Icon: 64x64px playful (tooth mascot, calendar, document, etc.)
├─ Icon color: Primary blue #3B82F6
├─ Heading: 16px, bold, #334155 (e.g., "Belum ada janji temu")
├─ Description: 13px, gray, #94A3B8 (encouraging message)
└─ CTA Button: [Buat Janji Temu Baru] (primary blue, 12px text)
```

---

## [8] BUTTON COMPONENT SYSTEM (Soft UI Variants)

### 8.1 Button Styling

```css
/* PRIMARY BUTTON */
.btn-primary {
  background: linear-gradient(135deg, #3B82F6 0%, #06B6D4 100%);
  color: #FFFFFF;
  padding: 12px 24px;
  border-radius: 12px;
  font-size: 14px;
  font-weight: 600;
  border: none;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.25);
  transition: all 200ms ease-in-out;
}

.btn-primary:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 20px rgba(59, 130, 246, 0.35);
}

.btn-primary:active {
  transform: translateY(0);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.25);
}

/* SECONDARY BUTTON */
.btn-secondary {
  background: #F1F5F9;
  color: #3B82F6;
  padding: 12px 24px;
  border-radius: 12px;
  font-size: 14px;
  font-weight: 600;
  border: 1px solid #E2E8F0;
  cursor: pointer;
  transition: all 200ms ease-in-out;
}

.btn-secondary:hover {
  background: #E0F2FE;
  border-color: #3B82F6;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.1);
}

/* GHOST BUTTON */
.btn-ghost {
  background: transparent;
  color: #3B82F6;
  padding: 12px 24px;
  border-radius: 12px;
  font-size: 14px;
  font-weight: 600;
  border: 1px solid #E2E8F0;
  cursor: pointer;
  transition: all 150ms ease-in-out;
}

.btn-ghost:hover {
  background: #F1F5F9;
  border-color: #3B82F6;
}

/* DANGER BUTTON */
.btn-danger {
  background: transparent;
  color: #EC4899;
  padding: 12px 24px;
  border-radius: 12px;
  font-size: 14px;
  font-weight: 600;
  border: 1px solid #FBCFE8;
  cursor: pointer;
  transition: all 150ms ease-in-out;
}

.btn-danger:hover {
  background: #FDF2F8;
  border-color: #EC4899;
}

/* DISABLED STATE (All variants) */
.btn:disabled {
  opacity: 50%;
  cursor: not-allowed;
  transform: none;
}
```

---

## [9] CARD COMPONENT SYSTEM (Soft UI)

```css
/* STANDARD CARD */
.card {
  background: #FFFFFF;
  border-radius: 16px;
  padding: 16px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12);
  border: none;
  transition: all 200ms ease-in-out;
}

.card:hover {
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.15);
  transform: translateY(-2px);
}

/* HIGHLIGHTED CARD (Border-left accent) */
.card.highlighted {
  border-left: 4px solid #3B82F6;
  padding-left: 12px;
}

/* GLASS CARD (Glassmorphic) */
.card.glass {
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(10px);
  border: 1px solid rgba(255, 255, 255, 0.2);
  box-shadow: 0 8px 32px rgba(31, 38, 135, 0.15);
}
```

---

## [10] COLOR TOKENS & CSS VARIABLES (Soft UI)

```css
:root {
  /* Primary Colors (Soft Blue) */
  --primary: #3B82F6;
  --primary-dark: #2563EB;
  --primary-light: #DBEAFE;
  --primary-gradient: linear-gradient(135deg, #3B82F6 0%, #06B6D4 100%);

  /* Neutral Colors */
  --text-primary: #334155;
  --text-secondary: #94A3B8;
  --bg-page: #F1F5F9;
  --bg-card: #FFFFFF;
  --border: #E2E8F0;

  /* Semantic Colors */
  --success: #10B981;
  --warning: #F59E0B;
  --danger: #EC4899;
  --accent: #F97316;

  /* Shadows (Soft UI Neumorphism) */
  --shadow-sm: 0 2px 8px rgba(0, 0, 0, 0.08);
  --shadow-md: 0 4px 16px rgba(0, 0, 0, 0.12);
  --shadow-lg: 0 8px 24px rgba(0, 0, 0, 0.15);
  --shadow-xl: 0 12px 32px rgba(0, 0, 0, 0.18);

  /* Glassmorphism */
  --glass-bg: rgba(255, 255, 255, 0.8);
  --glass-blur: blur(10px);

  /* Spacing Scale */
  --spacing-2: 4px;
  --spacing-4: 8px;
  --spacing-6: 12px;
  --spacing-8: 16px;
  --spacing-10: 20px;
  --spacing-12: 24px;
  --spacing-16: 32px;

  /* Border Radius */
  --radius-sm: 8px;
  --radius-md: 12px;
  --radius-lg: 16px;
  --radius-xl: 20px;
  --radius-full: 9999px;

  /* Transitions */
  --transition-fast: 150ms ease-in-out;
  --transition-normal: 200ms ease-in-out;
  --transition-slow: 300ms ease-in-out;
}
```

---

## [11] TYPOGRAPHY (Soft UI Optimized)

```css
/* Font Family (System fonts for optimal rendering) */
--font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', sans-serif;

/* Font Sizes */
--text-xs: 11px;    /* Small labels, helper text */
--text-sm: 12px;    /* Small body, captions */
--text-base: 14px;  /* Default body text */
--text-lg: 16px;    /* Larger body, inputs */
--text-xl: 18px;    /* Heading 3 */
--text-2xl: 20px;   /* Heading 2 */
--text-3xl: 24px;   /* Heading 1 */

/* Line Heights (Soft UI = Relaxed) */
--line-tight: 1.2;     /* Headings */
--line-normal: 1.5;    /* Body text (relaxed for readability) */
--line-relaxed: 1.75;  /* Long-form text */

/* Font Weights */
--font-regular: 400;
--font-medium: 500;
--font-semibold: 600;
--font-bold: 700;
```

---

## [12] MICRO-INTERACTIONS & ANIMATIONS (Soft UI Polish)

### 12.1 Button Interactions
```
Hover State:
├─ Transform: translateY(-2px) [subtle lift]
├─ Shadow: Increase elevation
├─ Duration: 200ms ease-in-out
└─ Cursor: pointer

Active/Pressed State:
├─ Transform: translateY(0) [return to normal]
├─ Shadow: Decrease elevation
├─ Opacity: 0.9
└─ Duration: 100ms ease-out

Focus State:
├─ Outline: 2px solid primary color
├─ Outline-offset: 2px (external ring)
└─ No other color changes (accessibility)
```

### 12.2 Card Interactions
```
Hover State (on tap-able cards):
├─ Shadow: Increase by 1 elevation tier
├─ Transform: translateY(-2px) [subtle lift]
├─ Duration: 200ms ease-in-out
└─ Cursor: pointer

Swipe Actions (Optional on bookings):
├─ Reveal action: Smooth slide-in (200ms)
├─ Color: Danger color (#EC4899)
└─ Animation: Easing: ease-out
```

### 12.3 Loading & Transitions
```
Page Transition:
├─ Fade-in: Opacity 0→1 (300ms ease-out)
├─ Scale: 0.98→1.0 (300ms ease-out)
└─ Combined: Natural, smooth

Skeleton to Content Transition:
├─ Fade-out skeleton: Opacity 1→0 (150ms ease-out)
├─ Fade-in content: Opacity 0→1 (150ms ease-in)
└─ Result: Smooth swap, no jarring

List Item Enter:
├─ Slide-in from bottom: translateY(10px)→0
├─ Fade-in: 0→1
├─ Stagger: 30ms delay per item
└─ Duration: 200ms ease-out
```

---

## [13] ACCESSIBILITY & WCAG AA (Maintained)

### 13.1 Color Contrast
- **Text on Background**: Minimum 4.5:1 (normal text)
- **Text on Colored Background**: Test with contrast checker
- **Interactive Elements**: Minimum 3:1

### 13.2 Touch Targets
- **Minimum**: 44x44px (iOS) / 48x48dp (Android)
- **Spacing**: Minimum 8px between interactive elements

### 13.3 Keyboard Navigation
- **Tab order**: Logical, visible focus indicator
- **Focus ring**: 2px solid primary color, offset 2px

### 13.4 Semantic HTML & ARIA
- All buttons have `type` attribute
- All inputs have associated labels
- Modals have `role="dialog"` and `aria-modal="true"`
- Live regions use `aria-live="polite"`

---

## [14] RESPONSIVE DESIGN BREAKPOINTS

```
Mobile Portrait:  320px - 480px   (Single column, full-width)
Mobile Landscape: 480px - 640px   (1.5 column, optimized)
Tablet:           640px - 1024px  (2 columns, side-by-side)
Tablet+ :         1024px+         (Multi-column, desktop)
```

---

## [15] IMPLEMENTATION NOTES FOR DEVELOPERS

### Key Reminders:
1. ✅ **Always use soft shadows** (not hard, always blur 8-12px)
2. ✅ **Generous spacing** (16-20px padding, 20-24px card margins)
3. ✅ **Rounded corners everywhere** (12-20px depending on element)
4. ✅ **Smooth transitions** (200-300ms ease-in-out, no instant changes)
5. ✅ **Playful mascot** (use tooth character in empty states, onboarding)
6. ✅ **Glassmorphic modals** (semi-transparent, backdrop blur)
7. ✅ **Soft colors** (no neon, no harsh reds—use pink/soft orange)
8. ✅ **Micro-interactions** (hover lifts, active feedback, loading states)

### Technology Stack:
- **Frontend**: React / React Native / Flutter
- **Styling**: Tailwind CSS + Custom CSS for soft shadows & animations
- **Components**: Custom (or Radix UI / shadcn/ui with customization)
- **Animations**: CSS transitions or Framer Motion for complex interactions

---

## [16] DESIGN SYSTEM SUMMARY TABLE

| Aspect | Soft UI Value |
|--------|---------------|
| **Shadow Blur** | 8-12px (soft, not sharp) |
| **Border Radius** | 12-20px (everywhere) |
| **Primary Color** | #3B82F6 (soft blue) |
| **Card Padding** | 16px (relaxed) |
| **Card Margin** | 20px (generous spacing) |
| **Button Padding** | 12px 24px (comfortable) |
| **Transition Duration** | 200ms ease-in-out |
| **Hover Effect** | Lift + Shadow increase |
| **Color Palette** | Soft, pastel, warm (no harsh colors) |
| **Tone** | Friendly, approachable, warm |
| **Mascot** | Playful tooth character (reduces anxiety) |

---

## [17] FUTURE ENHANCEMENTS (Post-MVP)

1. **Telemedicine Integration** - Video consultation booking
2. **Prescription E-delivery** - Pharmacy integration
3. **Dark Mode** - Full dark theme support (glassmorphism adapts well)
4. **Loyalty Program** - Points & rewards dashboard
5. **Doctor Reviews** - Post-visit feedback & rating
6. **Family Accounts** - Manage multiple family members
7. **AI Symptom Checker** - Chatbot assistant
8. **Offline Mode** - Service worker + IndexedDB
9. **Appointment Analytics** - Personal visit trends
10. **Multi-language Support** - i18n framework ready

---

**End of DESIGN_MOBILE.md v4.0 (Soft UI Neumorphism + Glassmorphism Edition)**

✨ **Catatan Penting:**
- File ini mengadopsi aesthetic "Soft UI + Glassmorphism" dari mockup gambar
- **Konten & struktur halaman TIDAK BERUBAH** dari file asli (Home, Bookings, EMR, Profile tetap sama)
- **Hanya styling & visual aesthetic yang di-update** (shadows, spacing, colors, micro-interactions)
- **Ready to upload ke Stitch With Google** untuk generate source code
- **Post-generation, manual refinement** untuk match mockup aesthetic 95%+ akan diperlukan

---
