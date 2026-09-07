"use client";

import {
  GoIcon,
  FlutterIcon,
  NextIcon,
  SupabaseIcon,
  RedisIcon,
} from "@/components/icons";
import { Reveal } from "@/components/Reveal";
import { useLang } from "@/app/i18n/config";

const STACK = [
  { key: "go", icon: GoIcon },
  { key: "flutter", icon: FlutterIcon },
  { key: "next", icon: NextIcon },
  { key: "supabase", icon: SupabaseIcon },
  { key: "redis", icon: RedisIcon },
] as const;

export default function TechStack() {
  const { t } = useLang();

  return (
    <section id="teknologi" className="py-24 lg:py-28">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <Reveal className="mx-auto max-w-2xl text-center">
          <span className="text-xs font-bold uppercase tracking-widest text-secondary-dark">
            {t("tech.badge")}
          </span>
          <h2 className="mt-3 text-3xl font-extrabold tracking-tight text-text-primary sm:text-4xl">
            {t("tech.title")}
          </h2>
          <p className="mt-4 text-base leading-relaxed text-text-support sm:text-lg">
            {t("tech.subtitle")}
          </p>
        </Reveal>

        <div className="mt-14 grid grid-cols-2 gap-6 sm:grid-cols-3 lg:grid-cols-5">
          {STACK.map((tech, i) => {
            const Icon = tech.icon;
            return (
              <Reveal delay={0.08 * i} key={tech.key}>
                <div
                  className="group flex aspect-square flex-col items-center justify-center gap-4 rounded-card border border-neutral-200 bg-white p-6 shadow-sm transition duration-300 hover:rotate-1 hover:scale-105 hover:border-secondary/40 hover:shadow-soft"
                >
                  <Icon className="h-12 w-12 transition-transform duration-300 group-hover:scale-110" />
                  <span className="text-sm font-bold text-text-primary">
                    {t(`tech.${tech.key}.name`)}
                  </span>
                </div>
              </Reveal>
            );
          })}
        </div>
      </div>
    </section>
  );
}