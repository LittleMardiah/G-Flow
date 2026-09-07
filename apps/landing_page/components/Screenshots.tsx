"use client";

import Image from "next/image";
import { Reveal } from "@/components/Reveal";
import { useLang } from "@/app/i18n/config";

const IMAGES = [
  { key: "home", src: "/screenshots/home.svg", alt: "Home" },
  { key: "ride", src: "/screenshots/ride.svg", alt: "G-Ride" },
  { key: "food", src: "/screenshots/food.svg", alt: "G-Food" },
] as const;

export default function Screenshots() {
  const { t } = useLang();

  return (
    <section
      id="aplikasi"
      className="bg-gradient-to-b from-neutral-100 to-white py-24 lg:py-28"
    >
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <Reveal className="mx-auto max-w-2xl text-center">
          <span className="text-xs font-bold uppercase tracking-widest text-secondary-dark">
            {t("screenshots.badge")}
          </span>
          <h2 className="mt-3 text-3xl font-extrabold tracking-tight text-text-primary sm:text-4xl">
            {t("screenshots.title")}
          </h2>
          <p className="mt-4 text-base leading-relaxed text-text-support sm:text-lg">
            {t("screenshots.subtitle")}
          </p>
        </Reveal>

        <div className="mt-14 grid grid-cols-1 gap-8 sm:grid-cols-2 lg:grid-cols-3">
          {IMAGES.map((img, i) => (
            <Reveal delay={0.1 * i} key={img.key}>
              <figure className="group">
                <div className="rounded-xl border border-neutral-200 bg-white p-3 shadow-soft transition duration-300 hover:-translate-y-1.5">
                  <div className="overflow-hidden rounded-lg border border-neutral-100">
                    <Image
                      src={img.src}
                      alt={`${img.alt} — ${t("screenshots.badge")}`}
                      width={480}
                      height={960}
                      className="mx-auto h-auto w-full max-w-[240px] object-contain transition-transform duration-500 group-hover:scale-[1.03]"
                    />
                  </div>
                </div>
                <figcaption className="mt-5 text-center">
                  <div className="text-lg font-bold text-text-primary">
                    {t(`screenshots.items.${img.key}.title`)}
                  </div>
                  <div className="mt-1 text-sm text-text-support">
                    {t(`screenshots.items.${img.key}.desc`)}
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