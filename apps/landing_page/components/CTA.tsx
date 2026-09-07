"use client";

import { Play } from "lucide-react";
import { GitHubIcon } from "@/components/icons";
import { Reveal } from "@/components/Reveal";
import { useLang } from "@/app/i18n/config";

export default function CTA() {
  const { t } = useLang();

  return (
    <section id="kontak" className="py-24">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <Reveal>
          <div className="relative overflow-hidden rounded-3xl bg-gradient-to-br from-primary to-primary-dark px-6 py-16 text-center sm:px-16 sm:py-20">
            <div className="pointer-events-none absolute -left-16 -top-16 h-64 w-64 rounded-full bg-white/10 blur-2xl" />
            <div className="pointer-events-none absolute -bottom-20 -right-10 h-72 w-72 rounded-full bg-accent/40 blur-2xl" />

            <h2 className="relative text-3xl font-extrabold tracking-tight text-white sm:text-4xl">
              {t("cta.title")}
            </h2>
            <p className="relative mx-auto mt-4 max-w-xl text-base leading-relaxed text-primary-50 sm:text-lg">
              {t("cta.subtitle")}
            </p>

            <div className="relative mt-9 flex flex-col items-center justify-center gap-4 sm:flex-row">
              <a
                href="https://github.com/LittleMardiah/G-Flow"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex w-full items-center justify-center gap-2 rounded-full bg-white px-7 py-3.5 font-semibold text-primary-dark shadow-lg transition hover:-translate-y-0.5 hover:bg-primary-50 sm:w-auto"
              >
                <GitHubIcon className="h-4 w-4" />
                {t("cta.github")}
              </a>
              <a
                href="#aplikasi"
                className="inline-flex w-full items-center justify-center gap-2 rounded-full border border-white/40 bg-white/10 px-7 py-3.5 font-semibold text-white backdrop-blur transition hover:-translate-y-0.5 hover:bg-white/20 sm:w-auto"
              >
                <Play className="h-4 w-4" />
                {t("cta.demo")}
              </a>
            </div>
          </div>
        </Reveal>
      </div>
    </section>
  );
}