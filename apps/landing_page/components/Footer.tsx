import Link from "next/link";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

export default function Footer() {
  const year = new Date().getFullYear();

  return (
    <footer className="w-full border-t border-border-subtle bg-surface-container-lowest py-8">
      <div className="mx-auto flex max-w-[1240px] flex-col items-center justify-between gap-6 px-4 md:flex-row lg:px-8">
        <div className="flex items-center gap-3 font-body text-sm text-text-secondary">
          <span className="font-display text-xl text-text-primary">G-Flow</span>
          <span className="text-border-subtle">|</span>
          <span>© {year} • Ekosistem Indonesia</span>
        </div>

        <div className="flex flex-wrap items-center gap-6 font-body text-sm text-text-muted">
          <Link
            href="/docs#privasi"
            className="text-text-muted transition-colors hover:text-text-secondary"
          >
            Kebijakan Privasi
          </Link>
          <Link
            href="/docs#syarat"
            className="text-text-muted transition-colors hover:text-text-secondary"
          >
            Syarat &amp; Ketentuan
          </Link>
          <Link
            href="/docs"
            className="inline-flex items-center gap-1.5 text-text-muted transition-colors hover:text-text-secondary"
          >
            <MaterialIcon name="terminal" className="text-[16px]" />
            Developer Docs
          </Link>
          <a
            href="https://github.com/LittleMardiah/G-Flow"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 text-text-muted transition-colors hover:text-text-secondary"
          >
            <MaterialIcon name="code" className="text-[16px]" />
            GitHub
          </a>
        </div>
      </div>
    </footer>
  );
}