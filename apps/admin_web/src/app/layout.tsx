import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "G-Flow Admin Panel",
  description: "Admin Web Panel for G-Flow Super-App",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-gray-100 text-gray-900 antialiased">
        {children}
      </body>
    </html>
  );
}
