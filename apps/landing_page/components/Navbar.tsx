"use client";

import { useState } from "react";
import { Menu, X } from "lucide-react";
import { useLang } from "@/app/i18n/config";

const LINKS = [
  { href: "#beranda", key: "beranda" },
  { href: "#fitur", key: "fitur" },
  { href: "#teknologi", key: "teknologi" },
  { href: "#aplikasi", key: "aplikasi" },
  { href: "#kontak", key: "kontak" },
] as const;

export default function Navbar() {
  const { t, lang, setLang } = useLang();
  const [open, setOpen] = useState(false);

  const close = () => setOpen(false);

  return (
    <header className="fixed inset-x-0 top-0 z-50 border-b border-neutral-200/70 bg-white/90 backdrop-blur">
      <nav
        className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8"
        aria-label={t("nav.menu")}
      >
        <a href="#beranda" className="flex shrink-0 items-center gap-2.5">
          <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary text-lg font-extrabold text-white shadow-md shadow-primary/30">
            G
          </span>
          <span className="text-xl font-bold tracking-tight text-neutral-900">
            G-<span className="text-primary">Flow</span>
          </span>
        </a>

        <div className="hidden items-center gap-8 md:flex">
          {LINKS.map((link) => (
            <a
              key={link.href}
              href={link.href}
              className="text-sm font-medium text-neutral-600 transition-colors hover:text-primary"
            >
              {t(`nav.${link.key}`)}
            </a>
          ))}
        </div>

        <div className="flex items-center gap-3">
          <div
            className="flex items-center rounded-full border border-neutral-200 bg-neutral-50 p-0.5 text-xs font-semibold"
            role="group"
            aria-label={t("nav.toggleLang")}
          >
            <button
              type="button"
              onClick={() => setLang("id")}
              aria-pressed={lang === "id"}
              className={`rounded-full px-3 py-1 transition-colors ${
                lang === "id"
                  ? "bg-primary text-white shadow-sm"
                  : "text-neutral-500 hover:text-primary"
              }`}
            >
              ID
            </button>
            <button
              type="button"
              onClick={() => setLang("en")}
              aria-pressed={lang === "en"}
              className={`rounded-full px-3 py-1 transition-colors ${
                lang === "en"
                  ? "bg-primary text-white shadow-sm"
                  : "text-neutral-500 hover:text-primary"
              }`}
            >
              EN
            </button>
          </div>

          <a
            href="#aplikasi"
            className="hidden rounded-full bg-primary px-5 py-2 text-sm font-semibold text-white shadow-lg shadow-primary/30 transition hover:-translate-y-0.5 hover:bg-primary-dark sm:inline-flex"
          >
            {t("nav.demo")}
          </a>

          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            className="inline-flex h-10 w-10 items-center justify-center rounded-lg border border-neutral-200 text-neutral-700 md:hidden"
            aria-label={t("nav.menu")}
            aria-expanded={open}
          >
            {open ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>
        </div>
      </nav>

      {open && (
        <div className="border-t border-neutral-100 bg-white px-4 pb-5 pt-3 md:hidden">
          <div className="flex flex-col gap-1">
            {LINKS.map((link) => (
              <a
                key={link.href}
                href={link.href}
                onClick={close}
                className="rounded-lg px-3 py-2.5 text-sm font-medium text-neutral-700 transition hover:bg-primary-50 hover:text-primary"
              >
                {t(`nav.${link.key}`)}
              </a>
            ))}
            <a
              href="#aplikasi"
              onClick={close}
              className="mt-2 rounded-full bg-primary px-5 py-2.5 text-center text-sm font-semibold text-white shadow-lg shadow-primary/30 transition hover:bg-primary-dark"
            >
              {t("nav.demo")}
            </a>
          </div>
        </div>
      )}
    </header>
  );
}