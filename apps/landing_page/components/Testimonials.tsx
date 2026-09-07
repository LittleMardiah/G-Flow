"use client";

import { Reveal } from "@/components/Reveal";
import { useLang } from "@/app/i18n/config";

const AVATAR_GRADIENTS = [
  "from-[#0F3B2E] to-[#2E6B55]",
  "from-[#D4A853] to-[#B8903B]",
  "from-[#3B4A5A] to-[#6B7B8D]",
] as const;

const ITEMS = ["t1", "t2", "t3"] as const;

export default function Testimonials() {
  const { t } = useLang();

  return (
    <section id="testimoni" className="bg-neutral-50 py-24 lg:py-28">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <Reveal className="mx-auto max-w-2xl text-center">
          <span className="text-xs font-bold uppercase tracking-widest text-secondary-dark">
            {t("testimonials.badge")}
          </span>
          <h2 className="mt-3 text-3xl font-extrabold tracking-tight text-text-primary sm:text-4xl">
            {t("testimonials.title")}
          </h2>
          <p className="mt-4 text-base leading-relaxed text-text-support sm:text-lg">
            {t("testimonials.subtitle")}
          </p>
        </Reveal>

        <div className="mt-14 grid grid-cols-1 gap-6 md:grid-cols-3">
          {ITEMS.map((item, i) => (
            <Reveal delay={0.1 * i} key={item}>
              <figure className="flex h-full flex-col rounded-card border border-neutral-200 bg-white p-7 shadow-soft transition duration-300 hover:-translate-y-1.5 hover:border-secondary/40">
                <blockquote className="flex-1 text-sm italic leading-relaxed text-text-support">
                  &ldquo;{t(`testimonials.${item}.quote`)}&rdquo;
                </blockquote>
                <figcaption className="mt-6 flex items-center gap-3 border-t border-neutral-100 pt-5">
                  <span
                    className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-gradient-to-br text-sm font-bold text-white ${AVATAR_GRADIENTS[i]}`}
                  >
                    {t(`testimonials.${item}.initials`)}
                  </span>
                  <div>
                    <div className="text-sm font-bold text-text-primary">
                      {t(`testimonials.${item}.name`)}
                    </div>
                    <div className="text-xs text-text-support">
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