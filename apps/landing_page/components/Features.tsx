"use client";

import { RideIcon, FoodIcon, SendIcon, WalletIcon } from "@/components/icons";
import { Reveal } from "@/components/Reveal";
import { useLang } from "@/app/i18n/config";

const FEATURES = [
  { key: "ride", icon: RideIcon },
  { key: "food", icon: FoodIcon },
  { key: "send", icon: SendIcon },
  { key: "wallet", icon: WalletIcon },
] as const;

export default function Features() {
  const { t } = useLang();

  return (
    <section id="fitur" className="py-24 lg:py-28">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <Reveal className="mx-auto max-w-2xl text-center">
          <span className="text-xs font-bold uppercase tracking-widest text-secondary-dark">
            {t("features.badge")}
          </span>
          <h2 className="mt-3 text-3xl font-extrabold tracking-tight text-text-primary sm:text-4xl">
            {t("features.title")}
          </h2>
          <p className="mt-4 text-base leading-relaxed text-text-support sm:text-lg">
            {t("features.subtitle")}
          </p>
        </Reveal>

        <div className="mt-14 grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {FEATURES.map((feature, i) => {
            const Icon = feature.icon;
            return (
              <Reveal delay={0.08 * i} key={feature.key}>
                <article className="group h-full rounded-card border border-neutral-200 bg-white p-7 shadow-sm transition duration-300 hover:scale-[1.02] hover:border-secondary/40 hover:shadow-soft">
                  <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 text-primary transition-colors duration-300 group-hover:bg-primary group-hover:text-white">
                    <Icon className="h-6 w-6" />
                  </div>
                  <h3 className="mt-5 text-lg font-bold text-text-primary">
                    {t(`features.${feature.key}.title`)}
                  </h3>
                  <p className="mt-3 text-sm leading-relaxed text-text-support">
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