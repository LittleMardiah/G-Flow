"use client";

import { Reveal } from "@/components/Reveal";
import { useLang } from "@/app/i18n/config";

const STACK = [
  {
    key: "go",
    initials: "Go",
    bg: "#00ADD8",
  },
  {
    key: "flutter",
    initials: "Fl",
    bg: "#02569B",
  },
  {
    key: "next",
    initials: "N",
    bg: "#111827",
  },
  {
    key: "supabase",
    initials: "S",
    bg: "#3ECF8E",
  },
  {
    key: "redis",
    initials: "R",
    bg: "#DC382D",
  },
] as const;

export default function TechStack() {
  const { t } = useLang();

  return (
    <section id="teknologi" className="py-24">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <Reveal className="mx-auto max-w-2xl text-center">
          <span className="text-xs font-bold uppercase tracking-widest text-accent">
            {t("tech.badge")}
          </span>
          <h2 className="mt-3 text-3xl font-extrabold tracking-tight text-neutral-900 sm:text-4xl">
            {t("tech.title")}
          </h2>
          <p className="mt-4 text-base leading-relaxed text-neutral-600 sm:text-lg">
            {t("tech.subtitle")}
          </p>
        </Reveal>

        <div className="mt-14 grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-5">
          {STACK.map((tech, i) => (
            <Reveal delay={0.08 * i} key={tech.key}>
              <article className="group h-full rounded-2xl border border-neutral-200 bg-white p-7 text-center shadow-sm transition duration-300 hover:-translate-y-1.5 hover:shadow-xl">
                <div
                  className="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl text-xl font-extrabold text-white shadow-md transition-transform duration-300 group-hover:scale-110"
                  style={{ backgroundColor: tech.bg }}
                >
                  {tech.initials}
                </div>
                <h3 className="mt-5 text-lg font-bold text-neutral-900">
                  {t(`tech.${tech.key}.name`)}
                </h3>
                <p className="mt-3 text-sm leading-relaxed text-neutral-600">
                  {t(`tech.${tech.key}.desc`)}
                </p>
              </article>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}