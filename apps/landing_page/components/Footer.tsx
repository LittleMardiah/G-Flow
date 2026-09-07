"use client";

import { GitHubIcon, LinkedInIcon } from "@/components/icons";
import { useLang } from "@/app/i18n/config";

export default function Footer() {
  const { t } = useLang();
  const year = new Date().getFullYear();

  return (
    <footer className="bg-neutral-900 py-12 text-neutral-300">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="flex flex-col items-center justify-between gap-8 md:flex-row">
          <div className="flex items-center gap-2.5">
            <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary text-lg font-extrabold text-white">
              G
            </span>
            <span className="text-xl font-bold tracking-tight text-white">
              G-<span className="text-primary-light">Flow</span>
            </span>
          </div>

          <p className="max-w-sm text-center text-sm leading-relaxed text-neutral-400 md:text-left">
            {t("footer.desc")}
          </p>

          <div className="flex items-center gap-3">
            <a
              href="https://github.com/LittleMardiah/G-Flow"
              target="_blank"
              rel="noopener noreferrer"
              aria-label="GitHub"
              className="flex h-10 w-10 items-center justify-center rounded-full border border-neutral-700 text-neutral-300 transition hover:border-primary hover:bg-primary hover:text-white"
            >
              <GitHubIcon className="h-4.5 w-4.5" />
            </a>
            <a
              href="https://www.linkedin.com/"
              target="_blank"
              rel="noopener noreferrer"
              aria-label="LinkedIn"
              className="flex h-10 w-10 items-center justify-center rounded-full border border-neutral-700 text-neutral-300 transition hover:border-primary hover:bg-primary hover:text-white"
            >
              <LinkedInIcon className="h-4.5 w-4.5" />
            </a>
          </div>
        </div>

        <div className="mt-10 flex flex-col items-center justify-between gap-3 border-t border-neutral-800 pt-8 text-xs text-neutral-500 sm:flex-row">
          <p>{t("footer.rights", { year })}</p>
          <p>
            <span className="mr-1.5 inline-block h-1.5 w-1.5 rounded-full bg-primary" />
            {t("footer.madeWith")}
          </p>
        </div>
      </div>
    </footer>
  );
}