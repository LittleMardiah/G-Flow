"use client";

import Link from "next/link";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { useLang } from "@/app/i18n/config";

export default function Footer() {
  const { t } = useLang();
  const year = new Date().getFullYear();

  return (
    <footer className="w-full border-t border-border-subtle bg-surface-container-lowest py-8">
      <div className="mx-auto flex max-w-[1240px] flex-col items-center justify-between gap-6 px-4 md:flex-row lg:px-8">
        <div className="flex items-center gap-3 font-body text-sm text-text-secondary">
          <span className="font-display text-xl text-text-primary">G-Flow</span>
          <span className="text-border-subtle">|</span>
          <span>
            © {year} • {t("footer.tagline")}
          </span>
        </div>

        <div className="flex flex-wrap items-center gap-6 font-body text-sm text-text-muted">
          <Link
            href="/docs#privasi"
            className="text-text-muted transition-colors hover:text-text-secondary"
          >
            {t("footer.privacy")}
          </Link>
          <Link
            href="/docs#syarat"
            className="text-text-muted transition-colors hover:text-text-secondary"
          >
            {t("footer.terms")}
          </Link>
          <Link
            href="/docs"
            className="inline-flex items-center gap-1.5 text-text-muted transition-colors hover:text-text-secondary"
          >
            <MaterialIcon name="terminal" className="text-[16px]" />
            {t("footer.docs")}
          </Link>
          <a
            href="https://github.com/LittleMardiah/G-Flow"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 text-text-muted transition-colors hover:text-text-secondary"
          >
            <MaterialIcon name="code" className="text-[16px]" />
            {t("footer.github")}
          </a>
        </div>
      </div>
    </footer>
  );
}
