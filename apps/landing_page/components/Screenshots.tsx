"use client";

import { useEffect, useState } from "react";
import useEmblaCarousel from "embla-carousel-react";
import Image from "next/image";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { Reveal } from "@/components/Reveal";
import { useLang } from "@/app/i18n/config";

const IMAGES = [
  { src: "/screenshots/home.svg", alt: "Home" },
  { src: "/screenshots/ride.svg", alt: "G-Ride" },
  { src: "/screenshots/food.svg", alt: "G-Food" },
  { src: "/screenshots/send.svg", alt: "G-Send" },
  { src: "/screenshots/wallet.svg", alt: "PayPulse" },
];

export default function Screenshots() {
  const { t } = useLang();
  const [emblaRef, emblaApi] = useEmblaCarousel({ loop: true, align: "start" });
  const [selected, setSelected] = useState(0);
  const [count, setCount] = useState(0);

  useEffect(() => {
    if (!emblaApi) return;
    const onInit = () => setCount(emblaApi.scrollSnapList().length);
    const onSelect = () => setSelected(emblaApi.selectedScrollSnap());
    emblaApi.on("init", onInit);
    emblaApi.on("reInit", onInit);
    emblaApi.on("select", onSelect);
    onInit();
    onSelect();
  }, [emblaApi]);

  return (
    <section id="aplikasi" className="bg-neutral-50 py-24">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <Reveal className="mx-auto max-w-2xl text-center">
          <span className="text-xs font-bold uppercase tracking-widest text-accent">
            {t("screenshots.badge")}
          </span>
          <h2 className="mt-3 text-3xl font-extrabold tracking-tight text-neutral-900 sm:text-4xl">
            {t("screenshots.title")}
          </h2>
          <p className="mt-4 text-base leading-relaxed text-neutral-600 sm:text-lg">
            {t("screenshots.subtitle")}
          </p>
        </Reveal>

        <Reveal delay={0.1} className="mt-14">
          <div className="embla">
            <div className="embla__viewport" ref={emblaRef}>
              <div className="embla__container">
                {IMAGES.map((img) => (
                  <div className="embla__slide" key={img.src}>
                    <div className="overflow-hidden rounded-3xl border border-neutral-200 bg-white shadow-lg shadow-neutral-200/60">
                      <Image
                        src={img.src}
                        alt={`${t("screenshots.badge")} — ${img.alt}`}
                        width={480}
                        height={960}
                        className="mx-auto h-auto w-full max-w-[280px] object-contain sm:max-w-[340px]"
                      />
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div className="mt-8 flex items-center justify-center gap-4">
              <button
                type="button"
                onClick={() => emblaApi?.scrollPrev()}
                className="flex h-11 w-11 items-center justify-center rounded-full border border-neutral-300 bg-white text-neutral-700 transition hover:border-primary hover:text-primary"
                aria-label="Previous"
              >
                <ChevronLeft className="h-5 w-5" />
              </button>

              <div className="flex items-center gap-2" role="tablist">
                {Array.from({ length: count }).map((_, i) => (
                  <button
                    key={i}
                    type="button"
                    role="tab"
                    aria-selected={selected === i}
                    onClick={() => emblaApi?.scrollTo(i)}
                    className={`h-2.5 w-2.5 rounded-full transition-all ${
                      selected === i
                        ? "w-7 bg-primary"
                        : "bg-neutral-300 hover:bg-neutral-400"
                    }`}
                  />
                ))}
              </div>

              <button
                type="button"
                onClick={() => emblaApi?.scrollNext()}
                className="flex h-11 w-11 items-center justify-center rounded-full border border-neutral-300 bg-white text-neutral-700 transition hover:border-primary hover:text-primary"
                aria-label="Next"
              >
                <ChevronRight className="h-5 w-5" />
              </button>
            </div>
          </div>
        </Reveal>
      </div>
    </section>
  );
}