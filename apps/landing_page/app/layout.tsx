import type { Metadata } from "next";
import { Space_Grotesk, DM_Sans } from "next/font/google";
import "./globals.css";

const spaceGrotesk = Space_Grotesk({
  variable: "--gflow-display",
  subsets: ["latin"],
  display: "swap",
  weight: ["600", "700"],
});

const dmSans = DM_Sans({
  variable: "--gflow-body",
  subsets: ["latin"],
  display: "swap",
  weight: ["400", "500", "600"],
});

export const metadata: Metadata = {
  title: "G-Flow — Bayar • Pesan • Kirim",
  description:
    "Akses mobilitas harian, pesan kuliner favorit, antar paket instan, hingga transaksi digital tanpa hambatan dalam satu genggaman aman dan transparan.",
  keywords: ["G-Flow", "super app", "ride-hailing", "food", "wallet", "Go", "Flutter"],
  openGraph: {
    title: "G-Flow — Bayar • Pesan • Kirim",
    description:
      "Satu Aplikasi untuk Semua. Mobilitas, kuliner, kirim paket, hingga transaksi digital.",
    type: "website",
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="id" className={`dark ${spaceGrotesk.variable} ${dmSans.variable}`}>
      <head>
        <link
          href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:opsz,wght,FILL,GRAD@24,400,0,0"
          rel="stylesheet"
        />
      </head>
      <body className="min-h-full antialiased">{children}</body>
    </html>
  );
}