"use client";

import { Star } from "lucide-react";
import { Reveal } from "@/components/Reveal";
import { useLang } from "@/app/i18n/config";

const AVATAR_STYLES = [
  { bg: "bg-primary", initials: "RK" },
  { bg: "bg-accent", initials: "BS" },
  { bg: "bg-primary-light", initials: "SP" },
] as const;

const ITEMS = ["t1", "t2", "t3"] as const;

export default function Testimonials() {
  const { t } = useLang();

  return (
    <section id="testimoni" className="bg-neutral-50 py-24">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <Reveal className="mx-auto max-w-2xl text-center">
          <span className="text-xs font-bold uppercase tracking-widest text-accent">
            {t("testimonials.badge")}
          </span>
          <h2 className="mt-3 text-3xl font-extrabold tracking-tight text-neutral-900 sm:text-4xl">
            {t("testimonials.title")}
          </h2>
          <p className="mt-4 text-base leading-relaxed text-neutral-600 sm:text-lg">
            {t("testimonials.subtitle")}
          </p>
        </Reveal>

        <div className="mt-14 grid grid-cols-1 gap-6 md:grid-cols-3">
          {ITEMS.map((item, i) => (
            <Reveal delay={0.1 * i} key={item}>
              <figure className="flex h-full flex-col rounded-2xl border border-neutral-200 bg-white p-7 shadow-sm transition duration-300 hover:-translate-y-1.5 hover:shadow-xl">
                <div className="flex gap-1 text-accent">
                  {Array.from({ length: 5 }).map((_, s) => (
                    <Star key={s} className="h-4 w-4 fill-current" strokeWidth={0} />
                  ))}
                </div>
                <blockquote className="mt-4 flex-1 text-sm leading-relaxed text-neutral-700">
                  &ldquo;{t(`testimonials.${item}.quote`)}&rdquo;
                </blockquote>
                <figcaption className="mt-6 flex items-center gap-3 border-t border-neutral-100 pt-5">
                  <span
                    className={`flex h-11 w-11 items-center justify-center rounded-full text-sm font-bold text-white ${AVATAR_STYLES[i].bg}`}
                  >
                    {AVATAR_STYLES[i].initials}
                  </span>
                  <div>
                    <div className="text-sm font-bold text-neutral-900">
                      {t(`testimonials.${item}.name`)}
                    </div>
                    <div className="text-xs text-neutral-500">
                      {t(`testimonials.${item}.role`)}
                    </div>
                  </div>
                </figcaption>
              </figure>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}