import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import { LanguageProvider } from "@/app/i18n/config";

const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin"],
  display: "swap",
});

export const metadata: Metadata = {
  title: "G-Flow — Super App Ecosystem",
  description:
    "G-Flow menghadirkan G-Ride, G-Food, G-Send, dan PayPulse dalam satu ekosistem super-app — dibangun dengan Go, Flutter, dan Next.js.",
  keywords: ["G-Flow", "super app", "ride-hailing", "food", "wallet", "Go", "Flutter"],
  openGraph: {
    title: "G-Flow — Super App Ecosystem",
    description:
      "G-Ride, G-Food, G-Send, dan PayPulse dalam satu ekosistem super-app.",
    type: "website",
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="id" className={`${inter.variable}`}>
      <body className="min-h-full antialiased">
        <LanguageProvider>{children}</LanguageProvider>
      </body>
    </html>
  );
}