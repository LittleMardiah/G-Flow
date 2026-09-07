"use client";

import { GitHubIcon, LinkedInIcon } from "@/components/icons";
import { useLang } from "@/app/i18n/config";

const LINKS = [
  { href: "#beranda", key: "beranda" },
  { href: "#fitur", key: "fitur" },
  { href: "#teknologi", key: "teknologi" },
  { href: "#aplikasi", key: "aplikasi" },
  { href: "#kontak", key: "kontak" },
] as const;

export default function Footer() {
  const { t } = useLang();
  const year = new Date().getFullYear();

  return (
    <footer className="bg-[#1A1A1A] py-16 text-white">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="grid grid-cols-1 gap-12 md:grid-cols-[1.5fr_1fr_1fr] lg:gap-16">
          <div>
            <a href="#beranda" className="flex items-center gap-2.5">
              <span className="relative flex h-9 w-9 items-center justify-center overflow-hidden rounded-xl bg-primary text-lg font-extrabold text-white">
                <span className="relative z-10">G</span>
                <span className="absolute -bottom-2 -right-2 h-5 w-5 rounded-full bg-secondary" />
              </span>
              <span className="text-xl font-bold tracking-tight text-white">
                G-<span className="text-secondary">Flow</span>
              </span>
            </a>
            <p className="mt-5 max-w-sm text-sm leading-relaxed text-white/60">
              {t("footer.desc")}
            </p>
          </div>

          <nav aria-label={t("nav.menu")}>
            <h3 className="text-sm font-bold uppercase tracking-widest text-white/40">
              {t("nav.menu")}
            </h3>
            <ul className="mt-5 space-y-3">
              {LINKS.map((link) => (
                <li key={link.href}>
                  <a
                    href={link.href}
                    className="text-sm text-white/70 transition-colors hover:text-secondary"
                  >
                    {t(`nav.${link.key}`)}
                  </a>
                </li>
              ))}
            </ul>
          </nav>

          <div>
            <h3 className="text-sm font-bold uppercase tracking-widest text-white/40">
              {t("footer.social")}
            </h3>
            <div className="mt-5 flex items-center gap-3">
              <a
                href="https://github.com/LittleMardiah/G-Flow"
                target="_blank"
                rel="noopener noreferrer"
                aria-label="GitHub"
                className="flex h-10 w-10 items-center justify-center rounded-full border border-white/15 text-white/70 transition hover:border-secondary hover:bg-secondary hover:text-white"
              >
                <GitHubIcon className="h-4.5 w-4.5" />
              </a>
              <a
                href="https://www.linkedin.com/"
                target="_blank"
                rel="noopener noreferrer"
                aria-label="LinkedIn"
                className="flex h-10 w-10 items-center justify-center rounded-full border border-white/15 text-white/70 transition hover:border-secondary hover:bg-secondary hover:text-white"
              >
                <LinkedInIcon className="h-4.5 w-4.5" />
              </a>
            </div>
          </div>
        </div>

        <div className="mt-12 flex flex-col items-center justify-between gap-3 border-t border-white/10 pt-8 text-xs text-white/50 sm:flex-row">
          <p>{t("footer.rights", { year })}</p>
          <p>
            <span className="mr-1.5 inline-block h-1.5 w-1.5 rounded-full bg-secondary" />
            {t("footer.madeWith")}
          </p>
        </div>
      </div>
    </footer>
  );
}