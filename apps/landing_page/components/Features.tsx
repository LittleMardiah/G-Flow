"use client";

import { Car, UtensilsCrossed, Package, Wallet } from "lucide-react";
import { Reveal } from "@/components/Reveal";
import { useLang } from "@/app/i18n/config";

const FEATURES = [
  { key: "ride", icon: Car },
  { key: "food", icon: UtensilsCrossed },
  { key: "send", icon: Package },
  { key: "wallet", icon: Wallet },
] as const;

export default function Features() {
  const { t } = useLang();

  return (
    <section id="fitur" className="py-24">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <Reveal className="mx-auto max-w-2xl text-center">
          <span className="text-xs font-bold uppercase tracking-widest text-accent">
            {t("features.badge")}
          </span>
          <h2 className="mt-3 text-3xl font-extrabold tracking-tight text-neutral-900 sm:text-4xl">
            {t("features.title")}
          </h2>
          <p className="mt-4 text-base leading-relaxed text-neutral-600 sm:text-lg">
            {t("features.subtitle")}
          </p>
        </Reveal>

        <div className="mt-14 grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {FEATURES.map((feature, i) => {
            const Icon = feature.icon;
            return (
              <Reveal delay={0.08 * i} key={feature.key}>
                <article className="group h-full rounded-2xl border border-neutral-200 bg-white p-7 shadow-sm transition duration-300 hover:-translate-y-1.5 hover:border-primary/30 hover:shadow-xl hover:shadow-primary/10">
                  <div className="flex h-13 w-13 items-center justify-center rounded-2xl bg-primary-50 p-3 text-primary transition-colors duration-300 group-hover:bg-primary group-hover:text-white">
                    <Icon className="h-7 w-7" strokeWidth={1.8} />
                  </div>
                  <h3 className="mt-5 text-xl font-bold text-neutral-900">
                    {t(`features.${feature.key}.title`)}
                  </h3>
                  <p className="mt-3 text-sm leading-relaxed text-neutral-600">
                    {t(`features.${feature.key}.desc`)}
                  </p>
                </article>
              </Reveal>
            );
          })}
        </div>
      </div>
    </section>
  );
}