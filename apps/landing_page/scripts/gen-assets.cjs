/* Generator aset visual landing page G-Flow (mockups + screenshots SVG). */
const fs = require("fs");
const path = require("path");

const P = "#2E7D32";
const PD = "#1B5E20";
const PL = "#A5D6A7";
const G50 = "#E8F5E9";
const A = "#FF6F00";
const AO50 = "#FFF3E0";
const TXT = "#1C1C1C";
const MUT = "#7A7A7A";
const BD = "#E5E7EB";
const BG = "#F4F7F4";
const WHITE = "#FFFFFF";

const esc = (s) => s.replaceAll("&", "&amp;");

function rect(x, y, w, h, r, fill, extra = "") {
  return `<rect x="${x}" y="${y}" width="${w}" height="${h}" ${r ? `rx="${r}"` : ""} fill="${fill}" ${extra}/>`;
}

function circle(cx, cy, r, fill) {
  return `<circle cx="${cx}" cy="${cy}" r="${r}" fill="${fill}"/>`;
}

function text(x, y, size, fill, content, weight = 400, anchor = "start") {
  return `<text x="${x}" y="${y}" font-size="${size}" fill="${fill}" font-weight="${weight}" text-anchor="${anchor}" font-family="Inter, system-ui, sans-serif">${esc(content)}</text>`;
}

function statusBar(bg) {
  return (
    rect(22, 18, 316, 24, 10, bg) +
    text(36, 35, 11, TXT, "09:41", 600) +
    circle(272, 30, 3, TXT) +
    rect(282, 27, 10, 6, 2, TXT) +
    rect(296, 25, 14, 8, 2, "#4ADE80") +
    rect(310, 27, 4, 6, 1, TXT)
  );
}

function screenFrame(bg) {
  return (
    rect(22, 18, 316, 664, 40, bg) +
    rect(156, 26, 48, 6, 3, "#161616") +
    statusBar(bg)
  );
}

function headerBar(title, bg, withAvatar, accent = false) {
  let out =
    rect(22, 66, 316, 56, 0, bg) +
    text(44, 100, 15, accent ? PD : TXT, "‹", 500) +
    text(64, 102, 17, TXT, title, 700);
  if (withAvatar) {
    out += circle(292, 95, 16, PL);
    out += text(292, 100, 13, PD, "R", 700, "middle");
  }
  return out;
}

function phoneFrame(content) {
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 360 700" width="360" height="700">` +
    rect(4, 4, 352, 692, 58, "#151515") +
    rect(14, 14, 332, 672, 46, "#0E0E0E") +
    content +
    `</svg>`
  );
}

function fullPage(content, bg = BG) {
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 360 720" width="360" height="720">` +
    rect(0, 0, 360, 720, 0, bg) +
    content +
    `</svg>`
  );
}

/* ---------------- MOCKUP: G-RIDE ---------------- */
function rideContent() {
  const mapBg =
    rect(22, 120, 316, 296, 20, "#EAF1EA") +
    rect(60, 140, 240, 6, 3, "#D8E2D8") +
    rect(120, 200, 160, 5, 2, "#D8E2D8") +
    rect(-30, 300, 200, 6, 3, "#D8E2D8") +
    rect(240, 340, 140, 5, 2, "#D8E2D8") +
    rect(70, 170, 120, 5, 2, PL) +
    circle(120, 240, 9, P) +
    circle(120, 240, 3, WHITE) +
    `<path d="M120 240 L270 360" stroke="${A}" stroke-width="6" stroke-linecap="round" fill="none"/>` +
    `<g transform="rotate(35 268 360)"><path d="M268 348 l0 24 l10 -8 l-10 -16 z" fill="${A}"/></g>` +
    circle(268, 360, 4, WHITE);

  const sheet = (
    rect(22, 438, 316, 240, 26, WHITE) +
    `<path d="M256 458 h-152" stroke="${BD}" stroke-width="4" stroke-linecap="round"/>` +
    rect(44, 478, 18, 18, 6, G50) + text(50, 491, 12, P, "◉", 700) +
    text(72, 491, 14, TXT, "Lokasi kamu", 600) +
    text(72, 506, 11, MUT, "Jl. Merdeka No. 8, Jakarta") +
    rect(44, 526, 18, 18, 6, AO50) + text(50, 539, 12, A, "▲", 700) +
    text(72, 539, 14, TXT, "Tujuan", 600) +
    text(72, 554, 11, MUT, "Mall Grand Indonesia") +
    rect(44, 578, 150, 30, 16, G50) + text(54, 597, 12, PD, "Dari Rp 28.000", 700) +
    rect(214, 578, 104, 30, 16, P) + text(266, 597, 13, WHITE, "Pesan", 700, "middle")
  );

  return screenFrame(G50) + headerBar("G-Ride", G50, true, true) + mapBg + sheet;
}

/* ---------------- MOCKUP: G-FOOD ---------------- */
function foodContent() {
  const search =
    rect(32, 140, 296, 38, 19, WHITE) +
    rect(48, 154, 16, 8, 2, BD) +
    circle(64, 159, 4, "none", ) +
    rect(78, 152, 130, 12, 0, BD) +
    `<circle cx="60" cy="160" r="7" fill="none" stroke="${MUT}" stroke-width="2"/><line x1="65" y1="165" x2="72" y2="172" stroke="${MUT}" stroke-width="2" stroke-linecap="round"/>`;

  const chips = (
    rect(32, 192, 60, 26, 13, P) + text(62, 209, 11, WHITE, "Semua", 600, "middle") +
    rect(100, 192, 68, 26, 13, WHITE) + text(134, 209, 11, TXT, "Makanan", 500, "middle") +
    rect(176, 192, 70, 26, 13, WHITE) + text(211, 209, 11, TXT, "Minuman", 500, "middle") +
    rect(254, 192, 70, 26, 13, WHITE) + text(289, 209, 11, TXT, "Desert", 500, "middle")
  );

  function item(y, img, name, cat, price) {
    return (
      `<g>` +
      rect(32, y, 296, 120, 20, WHITE) +
      `<rect x="48" y="${y + 16}" width="96" height="88" rx="14" fill="${img}"/>` +
      circle(74, y + 52, 9, WHITE) + circle(74, y + 60, 7, img) +
      rect(92, y + 52, 26, 6, 3, WHITE) + rect(92, y + 60, 18, 6, 3, WHITE) +
      text(160, y + 42, 13, TXT, name, 700) +
      text(160, y + 58, 10, MUT, cat) +
      text(160, y + 78, 12, A, price, 800) +
      circle(316, y + 88, 13, P) + text(316, y + 93, 15, WHITE, "+", 700, "middle") +
      `</g>`
    );
  }

  const items =
    item(232, PL, "Nasi Goreng Spesial", "Makanan · 4.9", "Rp 25.000", "#C8E6C9") +
    item(364, "#FFCC80", "Es Teh Manis", "Minuman · 4.8", "Rp 8.000", "#FFE0B2");

  const bottom = rect(22, 622, 316, 60, 0, WHITE);
  const cart =
    rect(42, 634, 48, 36, 12, AO50) +
    rect(50, 642, 32, 20, 8, A) +
    rect(56, 646, 20, 10, 3, WHITE) +
    rect(98, 646, 70, 11, 0, TXT) + rect(98, 660, 50, 8, 0, MUT) +
    rect(210, 642, 108, 40, 12, A) + text(264, 667, 14, WHITE, "Pesan • Rp 33.000", 800, "middle");

  return screenFrame(WHITE) + headerBar("G-Food", WHITE, false) + search + chips + items + bottom + cart;
}

/* ---------------- MOCKUP: PAYPULSE ---------------- */
function walletContent() {
  const balanceCard =
    rect(32, 132, 296, 120, 22, PD) +
    rect(32, 132, 296, 120, 22, `url(#grad)`) +
    `<defs><linearGradient id="grad" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${P}"/><stop offset="1" stop-color="${PD}"/></linearGradient></defs>` +
    text(48, 158, 11, "#C8E6C9", "Total Saldo") +
    circle(310, 152, 10, "rgba(255,255,255,0.2)") + text(310, 155, 11, WHITE, "👁", 600, "middle") +
    text(48, 196, 26, WHITE, "Rp 2.450.000", 800) +
    rect(48, 214, 120, 24, 12, "#173E1A") + circle(56, 226, 6, PL) + text(98, 231, 11, WHITE, "Top Up", 600, "middle") +
    rect(176, 214, 120, 24, 12, "rgba(255,255,255,0.18)") + text(236, 231, 11, WHITE, "Transfer", 600, "middle");

  const actions =
    [A, P, "#5B8DEF", PL].map((c, i) => {
      const x = 48 + i * 70;
      return rect(x, 278, 46, 46, 16, c) + circle(x + 23, 301, 12, "rgba(255,255,255,0.25)") + text(x + 23, 298, 14, WHITE, "◎", 700, "middle");
    }).join("") +
    [["Kirim", 59], ["Top Up", 124], ["Tagihan", 199], ["Riwayat", 271]].map(([l, x]) => text(x, 340, 10, MUT, l, 600, "middle")).join("");

  const txHeader = text(40, 376, 13, TXT, "Transaksi Terbaru", 700) + text(312, 376, 11, P, "Lihat semua", 600);

  const list =
    rect(32, 392, 296, 52, 14, WHITE) +
    rect(44, 402, 32, 32, 10, G50) + text(60, 423, 12, P, "↑", 700, "middle") +
    text(88, 418, 12, TXT, "Top Up dari Bank", 600) + text(88, 435, 10, MUT, "Hari ini · 10.24") +
    text(318, 421, 12, P, "+Rp 500.000", 800, "end") +
    rect(32, 452, 296, 52, 14, WHITE) +
    rect(44, 462, 32, 32, 10, AO50) + text(60, 483, 12, A, "➜", 700, "middle") +
    text(88, 478, 12, TXT, "Transfer ke Rina", 600) + text(88, 495, 10, MUT, "Kemarin · 19.02") +
    text(318, 481, 12, TXT, "-Rp 45.000", 700, "end") +
    rect(32, 512, 296, 52, 14, WHITE) +
    rect(44, 522, 32, 32, 10, "#E8EEF9") + text(60, 543, 12, "#5B8DEF", "◉", 700, "middle") +
    text(88, 538, 12, TXT, "Bayar G-Ride", 600) + text(88, 555, 10, MUT, "Kemarin · 08.15") +
    text(318, 541, 12, TXT, "-Rp 28.000", 700, "end");

  return (
    screenFrame(G50) +
    `<defs><linearGradient id="grad" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${P}"/><stop offset="1" stop-color="${PD}"/></linearGradient></defs>` +
    rect(22, 66, 316, 56, 0, G50) +
    text(40, 102, 15, TXT, "PayPulse", 700) +
    circle(292, 95, 16, PL) + text(292, 100, 13, PD, "R", 700, "middle") +
    balanceCard + actions + txHeader + list
  );
}

/* ---------------- SCREENSHOTS ---------------- */
function shotHome() {
  const header =
    text(28, 96, 22, TXT, "Halo, Rizky 👋", 800) +
    text(28, 118, 12, MUT, "Apa yang kamu butuhkan hari ini?");
  const bal =
    rect(24, 138, 312, 96, 20, PD) + `<defs><linearGradient id="gb" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${P}"/><stop offset="1" stop-color="${A}"/></linearGradient></defs>` +
    rect(24, 138, 312, 96, 20, "url(#gb)") +
    text(44, 166, 11, "#E8F5E9", "Saldo PayPulse") +
    text(44, 202, 26, WHITE, "Rp 2.450.000", 800) +
    rect(248, 156, 64, 28, 14, "rgba(255,255,255,0.2)") + text(280, 175, 11, WHITE, "+ Top Up", 700, "middle");
  const gridTitle = text(28, 284, 14, TXT, "Layanan", 800);
  const grid = [P, A, "#5B8DEF", PL].map((c, i) => {
    const x = 28 + (i % 2) * 160;
    const y = 298 + Math.floor(i / 2) * 92;
    return rect(x, y, 148, 76, 18, WHITE) + rect(x + 12, y + 12, 40, 40, 12, c) +
      [["G-Ride", "Naik kendaraan", "🚗"], ["G-Food", "Pesan makanan", "🍜"], ["G-Send", "Kirim paket", "📦"], ["PayPulse", "Dompet", "💳"]][i].map((txi, j) => {
        if (j === 0) return text(x + 62, y + 34, 13, TXT, txi, 800);
        if (j === 1) return text(x + 62, y + 52, 11, MUT, txi);
        return text(x + 30, y + 34, 18, WHITE, txi, 700, "middle");
      }).join("");
  }).join("");
  const promo = rect(24, 500, 312, 76, 18, AO50) + text(44, 532, 14, A, "Promo Spesial! 🎉", 800) + text(44, 555, 12, "#5C3A00", "Diskon 40% G-Food pertama kali.") + circle(290, 528, 22, WHITE) + text(290, 534, 14, A, "→", 800, "middle");
  const listTitle = text(28, 600, 14, TXT, "Aktivitas Terbaru", 800) +
    rect(24, 614, 312, 52, 14, WHITE) + rect(36, 624, 32, 32, 10, G50) + text(52, 645, 12, P, "↑", 700, "middle") +
    text(80, 640, 12, TXT, "Top Up sukses", 600) + text(80, 656, 10, MUT, "Baru saja") + text(324, 646, 12, P, "+Rp 500.000", 800, "end");
  return fullPage(`<circle cx="40" cy="40" r="90" fill="${G50}" opacity="0.6"/>` + header + bal + gridTitle + grid + promo + listTitle);
}

function shotRide() {
  const map =
    rect(0, 90, 360, 320, 0, "#EAF1EA") +
    rect(40, 120, 240, 6, 3, "#D8E2D8") + rect(120, 180, 160, 5, 2, "#D8E2D8") + rect(-40, 260, 220, 6, 3, "#D8E2D8") +
    rect(60, 150, 120, 5, 2, PL) + circle(110, 200, 10, P) + circle(110, 200, 3, WHITE) +
    `<path d="M110 200 L300 330" stroke="${A}" stroke-width="7" stroke-linecap="round" fill="none"/>` +
    `<path d="M300 318 l0 24 l12 -9 l-12 -15 z" fill="${A}"/>` + circle(300, 330, 4, WHITE);
  const card =
    rect(24, 424, 312, 248, 24, WHITE) +
    `<path d="M264 444 h-168" stroke="${BD}" stroke-width="4" stroke-linecap="round"/>` +
    rect(48, 462, 22, 22, 8, G50) + text(59, 478, 14, P, "◉", 700, "middle") +
    text(84, 476, 15, TXT, "Lokasi kamu", 700) + text(84, 494, 12, MUT, "Jl. Merdeka No. 8, Jakarta") +
    rect(48, 514, 22, 22, 8, AO50) + text(59, 530, 14, A, "▲", 700, "middle") +
    text(84, 528, 15, TXT, "Tujuan", 700) + text(84, 546, 12, MUT, "Mall Grand Indonesia") +
    rect(48, 566, 150, 34, 17, G50) + text(62, 588, 14, PD, "G-Ride Now", 800) +
    text(168, 588, 13, PD, "•", 800) + text(182, 588, 13, TXT, "Rp 28.000", 700) +
    rect(48, 616, 264, 40, 20, P) + text(180, 641, 15, WHITE, "Pesan Sekarang", 800, "middle") +
    text(180, 604, 11, MUT, "eta 12 menit • pengemudi terdekat 2,1 km", 500, "middle");
  return fullPage(rect(0, 0, 360, 90, G50) + text(72, 60, 22, TXT, "G-Ride", 800) + circle(300, 54, 18, PL) + text(300, 60, 14, PD, "R", 800, "middle") + text(28, 60, 20, TXT, "‹", 500) + map + card, "#F4F7F4");
}

function shotFood() {
  function item(y, b, name, cat, price, img) {
    return rect(24, y, 312, 88, 18, WHITE) + rect(38, y + 12, 64, 64, 14, img) +
      `<rect x="38" y="${y + 12}" width="64" height="64" rx="14" fill="${img}" opacity="0.85"/>` +
      circle(56, y + 40, 8, WHITE) + rect(70, y + 40, 22, 5, 2, WHITE) +
      text(116, y + 34, 13, TXT, name, 800) + text(116, y + 52, 11, MUT, cat) +
      text(116, y + 72, 13, A, price, 800) + circle(316, y + 60, 14, P) + text(316, y + 65, 16, WHITE, "+", 800, "middle");
  }
  return fullPage(
    rect(0, 0, 360, 66, WHITE) + rect(0, 66, 360, 2, BD) +
    text(72, 44, 20, TXT, "Najib Kitchen", 800) + circle(44, 40, 16, "#FFCC80") +
    text(28, 40, 18, TXT, "‹", 500) + text(300, 44, 14, A, "♥", 700) +
    rect(24, 84, 312, 40, 20, WHITE) + circle(44, 104, 8, "none") + `<circle cx="46" cy="104" r="9" fill="none" stroke="${MUT}" stroke-width="2"/><line x1="52" y1="110" x2="62" y2="120" stroke="${MUT}" stroke-width="2" stroke-linecap="round"/>` +
    text(76, 110, 13, "#333", "Cari menu di restoran ini...") +
    [["Nasi Goreng Spesial", "Makanan · 4.9 · 30 mnt", "Rp 25.000", "#C8E6C9", 140], ["Ayam Geprek", "Makanan · 4.8 · 25 mnt", "Rp 17.000", "#FFE0B2", 240], ["Es Teh Manis", "Minuman · 4.8 · 5 mnt", "Rp 8.000", "#B3E5FC", 340], ["Es Jeruk", "Minuman · 4.7 · 5 mnt", "Rp 9.000", "#FFCCCC", 440]].map(([n, c, p, img, y]) => item(y, img, n, c, p)).join("") +
    rect(0, 672, 360, 48, WHITE) + rect(0, 672, 360, 1, BD) +
    rect(24, 682, 312, 30, 15, P) + text(180, 702, 14, WHITE, "Lihat Pesanan • 4 item", 800, "middle"),
  );
}

function shotSend() {
  const map =
    rect(0, 80, 360, 220, 0, "#EAF1EA") +
    rect(30, 110, 200, 5, 2, "#D8E2D8") + rect(150, 160, 150, 5, 2, "#D8E2D8") +
    `<path d="M70 180 L120 180 L120 220 L290 220" stroke="${P}" stroke-width="6" stroke-linecap="round" stroke-linejoin="round" fill="none" stroke-dasharray="1 9"/>` +
    circle(70, 180, 9, P) + text(70, 185, 11, WHITE, "A", 800, "middle") +
    circle(120, 220, 9, A) + text(120, 225, 11, WHITE, "B", 800, "middle") +
    circle(290, 220, 9, "#5B8DEF") + text(290, 225, 11, WHITE, "C", 800, "middle") +
    text(48, 120, 12, "#5B8DEF", "Terhubung ke 3 lokasi", 700);
  const card =
    rect(24, 316, 312, 196, 22, WHITE) + `<path d="M264 336 h-168" stroke="${BD}" stroke-width="4" stroke-linecap="round"/>` +
    text(44, 366, 15, TXT, "Rute Multi-Stop", 800) + text(44, 386, 12, MUT, "3 titik pengantaran • 8,4 km") +
    rect(44, 408, 20, 20, 8, G50) + text(54, 422, 11, P, "A", 800, "middle") + text(80, 423, 13, TXT, "Paket dijemput", 700) + text(250, 423, 12, MUT, "Jl. Sudirman 12") +
    rect(44, 438, 20, 20, 8, AO50) + text(54, 452, 11, A, "B", 800, "middle") + text(80, 453, 13, TXT, "Antar ke Rina", 700) + text(250, 453, 12, MUT, "Kebon Jeruk") +
    rect(44, 468, 20, 20, 8, "#E8EEF9") + text(54, 482, 11, "#5B8DEF", "C", 800, "middle") + text(80, 483, 13, TXT, "Antar ke kantor", 700) + text(250, 483, 12, MUT, "SCBD Kuningan") +
    rect(24, 528, 312, 44, 22, A) + text(180, 555, 15, WHITE, "Cari Kurir • Rp 22.000", 800, "middle");
  return fullPage(rect(0, 0, 360, 80, G50) + text(72, 52, 20, TXT, "G-Send", 800) + circle(304, 48, 16, PL) + text(304, 54, 13, PD, "K", 800, "middle") + text(30, 52, 18, TXT, "‹", 500) + map + card);
}

function shotWallet() {
  function txn(y, c, ic, label, time, amt, sgn) {
    return rect(24, y, 312, 54, 14, WHITE) + rect(36, y + 10, 34, 34, 11, c) +
      text(53, y + 32, 13, WHITE, ic, 800, "middle") +
      text(84, y + 27, 13, TXT, label, 700) + text(84, y + 45, 11, MUT, time) +
      text(324, y + 32, 13, sgn === "+" ? P : TXT, `${sgn}${amt}`, 800, "end");
  }
  return fullPage(
    rect(0, 0, 360, 110, PD) + `<defs><linearGradient id="gb2" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${P}"/><stop offset="1" stop-color="${PD}"/></linearGradient></defs>` +
    rect(0, 0, 360, 110, "url(#gb2)") +
    text(28, 40, 14, "#C8E6C9", "Total Saldo") + text(28, 70, 24, WHITE, "Rp 2.450.000", 800) +
    rect(264, 30, 68, 26, 13, "rgba(255,255,255,0.2)") + text(298, 48, 11, WHITE, "+ Top Up", 800, "middle") +
    rect(28, 130, 304, 46, 14, WHITE) + text(44, 159, 13, TXT, "₹  Saring Transaksi", 600) +
    txn(196, G50, "↑", "Top up dari bank", "Hari ini · 10.24", "500.000", "+") +
    txn(258, AO50, "→", "Transfer ke Rina", "Kemarin · 19.02", "45.000", "-") +
    txn(320, "#E8EEF9", "㋡", "Bayar G-Ride", "Kemarin · 08.15", "28.000", "-") +
    txn(382, "#FCE4EC", "♡", "Bayar G-Food", "Senin · 12.40", "33.000", "-") +
    txn(444, G50, "↑", "Top up berhasil", "Minggu · 09.12", "200.000", "+") +
    text(40, 540, 13, P, "Lihat semua transaksi →", 700),
  );
}

const files = {
  "public/mockups/phone-ride.svg": phoneFrame(rideContent()),
  "public/mockups/phone-food.svg": phoneFrame(foodContent()),
  "public/mockups/phone-wallet.svg": phoneFrame(walletContent()),
  "public/screenshots/home.svg": shotHome(),
  "public/screenshots/ride.svg": shotRide(),
  "public/screenshots/food.svg": shotFood(),
  "public/screenshots/send.svg": shotSend(),
  "public/screenshots/wallet.svg": shotWallet(),
};

for (const [rel, svg] of Object.entries(files)) {
  const p = path.join(process.cwd(), rel);
  fs.mkdirSync(path.dirname(p), { recursive: true });
  fs.writeFileSync(p, svg, "utf8");
  console.log("✓", rel, (Buffer.byteLength(svg) / 1024).toFixed(1) + "KB");
}