"use client";

import { useState } from "react";
import { motion, useMotionValueEvent, useScroll } from "framer-motion";
import { Menu, X } from "lucide-react";
import { useLang } from "@/app/i18n/config";
import { Button } from "@/components/ui/Button";

const LINKS = [
  { href: "#beranda", key: "beranda" },
  { href: "#fitur", key: "fitur" },
  { href: "#teknologi", key: "teknologi" },
  { href: "#aplikasi", key: "aplikasi" },
  { href: "#kontak", key: "kontak" },
] as const;

function LangToggle() {
  const { lang, setLang } = useLang();
  const options = ["id", "en"] as const;

  return (
    <div
      className="relative flex items-center rounded-button border border-neutral-200 bg-accent/60 p-1 text-xs font-semibold"
      role="group"
      aria-label="Language"
    >
      {options.map((opt) => (
        <button
          key={opt}
          type="button"
          onClick={() => setLang(opt)}
          aria-pressed={lang === opt}
          className={`relative z-10 w-12 py-1 uppercase transition-colors duration-300 ${
            lang === opt ? "text-white" : "text-text-support hover:text-primary"
          }`}
        >
          {opt}
        </button>
      ))}
      <motion.span
        className="absolute left-1 top-1 h-[calc(100%-0.5rem)] w-12 rounded-button bg-primary shadow-md shadow-primary/30"
        initial={false}
        animate={{ x: lang === "en" ? 48 : 0 }}
        transition={{ type: "spring", stiffness: 400, damping: 32 }}
      />
    </div>
  );
}

export default function Navbar() {
  const { t } = useLang();
  const [scrolled, setScrolled] = useState(false);
  const [open, setOpen] = useState(false);
  const { scrollY } = useScroll();

  useMotionValueEvent(scrollY, "change", (latest) => {
    setScrolled(latest > 24);
  });

  const close = () => setOpen(false);

  return (
    <header
      className={`fixed inset-x-0 top-0 z-50 transition-all duration-300 ${
        scrolled
          ? "border-b border-neutral-200/70 bg-white/90 shadow-soft backdrop-blur-xl"
          : "border-b border-transparent bg-transparent"
      }`}
    >
      <nav
        className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8"
        aria-label={t("nav.menu")}
      >
        <a href="#beranda" className="flex shrink-0 items-center gap-2.5">
          <span className="relative flex h-9 w-9 items-center justify-center overflow-hidden rounded-xl bg-primary text-lg font-extrabold text-white">
            <span className="relative z-10">G</span>
            <span className="absolute -bottom-2 -right-2 h-5 w-5 rounded-full bg-secondary" />
          </span>
          <span className="text-xl font-bold tracking-tight text-text-primary">
            G-<span className="text-primary">Flow</span>
          </span>
        </a>

        <div className="hidden items-center gap-8 lg:flex">
          {LINKS.map((link) => (
            <a
              key={link.href}
              href={link.href}
              className="text-sm font-medium text-text-support transition-colors hover:text-primary"
            >
              {t(`nav.${link.key}`)}
            </a>
          ))}
        </div>

        <div className="flex items-center gap-3">
          <LangToggle />

          <Button
            href="#aplikasi"
            variant="ghost"
            className="hidden border-2 border-secondary sm:inline-flex"
          >
            {t("nav.demo")}
          </Button>

          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            className="inline-flex h-10 w-10 items-center justify-center rounded-button border border-neutral-200 bg-white/80 text-text-primary lg:hidden"
            aria-label={t("nav.menu")}
            aria-expanded={open}
          >
            {open ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>
        </div>
      </nav>

      {open && (
        <div className="border-t border-neutral-100 bg-white px-4 pb-5 pt-3 shadow-soft lg:hidden">
          <div className="flex flex-col gap-1">
            {LINKS.map((link) => (
              <a
                key={link.href}
                href={link.href}
                onClick={close}
                className="rounded-lg px-3 py-2.5 text-sm font-medium text-text-support transition hover:bg-accent hover:text-primary"
              >
                {t(`nav.${link.key}`)}
              </a>
            ))}
            <div className="mt-2 flex items-center justify-between gap-3">
              <Button href="#aplikasi" onClick={close} className="flex-1">
                {t("nav.demo")}
              </Button>
              <LangToggle />
            </div>
          </div>
        </div>
      )}
    </header>
  );
}