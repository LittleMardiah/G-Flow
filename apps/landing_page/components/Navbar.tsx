import Link from "next/link";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

const LINKS: { href: string; label: string; primary?: boolean }[] = [
  { href: "#beranda", label: "Beranda", primary: true },
  { href: "#layanan", label: "Layanan" },
  { href: "#mitra", label: "Mitra" },
  { href: "#kontak", label: "Kontak" },
];

export default function Navbar() {
  return (
    <header className="fixed inset-x-0 top-0 z-50 h-16 w-full border-b border-border-subtle bg-background/80 backdrop-blur-md">
      <div className="mx-auto flex h-16 w-full max-w-[1240px] items-center justify-between gap-4 px-4 lg:px-8">
        <div className="flex shrink-0 items-center gap-3">
          <Link href="#beranda" className="flex items-center gap-3">
            <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary font-headline font-bold text-surface-container-lowest">
              G
            </span>
            <span className="font-headline text-xl font-semibold tracking-tight text-text-primary">
              G-Flow
            </span>
          </Link>
          <span className="hidden items-center gap-1.5 rounded-full border border-border-subtle bg-surface-elevated px-2.5 py-1 sm:inline-flex">
            <span className="h-1.5 w-1.5 rounded-full bg-success-dot" />
            <span className="font-label text-text-secondary uppercase tracking-wider">
              Ecosystem
            </span>
          </span>
        </div>

        <nav className="hidden items-center gap-8 md:flex" aria-label="Navigasi utama">
          {LINKS.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className={
                link.primary
                  ? "font-medium text-text-primary transition-colors"
                  : "font-body text-sm text-text-secondary transition-colors hover:text-text-primary"
              }
            >
              {link.label}
            </Link>
          ))}
        </nav>

        <div className="flex shrink-0 items-center gap-3">
          <Link
            href="#kontak"
            className="inline-flex items-center justify-center rounded-full bg-primary-container px-5 py-2.5 font-label font-semibold text-surface-container-lowest shadow-[0_4px_16px_rgba(255,149,0,0.25)] transition-all hover:opacity-90"
          >
            Unduh Aplikasi
          </Link>
          <span className="flex h-8 w-8 items-center justify-center rounded-full bg-primary text-surface-container-lowest">
            <MaterialIcon name="person" className="text-[18px]" />
          </span>
        </div>
      </div>
    </header>
  );
}