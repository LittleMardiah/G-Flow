"use client";

import { GitHubIcon } from "@/components/icons";
import { Reveal } from "@/components/Reveal";
import { Button } from "@/components/ui/Button";
import { useLang } from "@/app/i18n/config";

export default function CTA() {
  const { t } = useLang();

  return (
    <section id="kontak" className="py-24 lg:py-28">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <Reveal>
          <div className="bg-grid-pattern relative overflow-hidden rounded-card bg-primary px-6 py-16 text-center sm:px-16 sm:py-20">
            <div className="pointer-events-none absolute -left-16 -top-16 h-64 w-64 rounded-full bg-secondary/20 blur-2xl" />
            <div className="pointer-events-none absolute -bottom-20 -right-10 h-72 w-72 rounded-full bg-white/10 blur-2xl" />

            <h2 className="relative text-4xl font-bold tracking-tight text-white">
              {t("cta.title")}
            </h2>
            <p className="relative mx-auto mt-4 max-w-xl text-lg leading-relaxed text-white/80">
              {t("cta.subtitle")}
            </p>

            <div className="relative mt-9 flex flex-col items-center justify-center gap-4 sm:flex-row">
              <Button
                href="https://github.com/LittleMardiah/G-Flow"
                target="_blank"
                rel="noopener noreferrer"
                variant="secondary"
                size="lg"
                className="w-full sm:w-auto"
              >
                <GitHubIcon className="h-4 w-4" />
                {t("cta.github")}
              </Button>
              <Button
                href="mailto:hello@gflow.app"
                variant="outlineWhite"
                size="lg"
                className="w-full sm:w-auto"
              >
                {t("cta.contact")}
              </Button>
            </div>
          </div>
        </Reveal>
      </div>
    </section>
  );
}