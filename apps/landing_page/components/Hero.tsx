"use client";

import { motion } from "framer-motion";
import Image from "next/image";
import { Play, Sparkles } from "lucide-react";
import { GitHubIcon } from "@/components/icons";
import { useLang } from "@/app/i18n/config";

const PHONES = [
  { src: "/mockups/phone-ride.svg", alt: "G-Ride mockup", className: "-rotate-6 translate-y-6" },
  { src: "/mockups/phone-food.svg", alt: "G-Food mockup", className: "z-10 scale-105" },
  { src: "/mockups/phone-wallet.svg", alt: "PayPulse mockup", className: "rotate-6 translate-y-6" },
];

const STATS = [
  { value: "statValueServices", label: "statServices" },
  { value: "statValueUptime", label: "statUptime" },
  { value: "statValueLanguages", label: "statLanguages" },
];

const fadeUp = {
  hidden: { opacity: 0, y: 28 },
  show: (i: number) => ({
    opacity: 1,
    y: 0,
    transition: { duration: 0.6, ease: "easeOut" as const, delay: 0.1 * i },
  }),
};

export default function Hero() {
  const { t } = useLang();

  return (
    <section id="beranda" className="relative overflow-hidden bg-gradient-to-b from-primary-50/70 via-white to-white pt-16">
      <div className="pointer-events-none absolute -left-32 top-24 h-72 w-72 rounded-full bg-primary-50 blur-3xl" />
      <div className="pointer-events-none absolute -right-24 top-40 h-80 w-80 rounded-full bg-accent-50 blur-3xl" />

      <div className="relative mx-auto grid max-w-7xl items-center gap-12 px-4 pb-10 pt-14 sm:px-6 lg:grid-cols-2 lg:gap-8 lg:px-8 lg:pt-20">
        <div className="text-center lg:text-left">
          <motion.span
            variants={fadeUp}
            initial="hidden"
            animate="show"
            custom={0}
            className="inline-flex items-center gap-2 rounded-full border border-primary-200 bg-primary-50 px-4 py-1.5 text-xs font-semibold text-primary-dark"
          >
            <Sparkles className="h-3.5 w-3.5" />
            {t("hero.badge")}
          </motion.span>

          <motion.h1
            variants={fadeUp}
            initial="hidden"
            animate="show"
            custom={1}
            className="mt-6 text-4xl font-extrabold leading-tight tracking-tight text-neutral-900 sm:text-5xl lg:text-6xl"
          >
            {t("hero.title1")}
            <br />
            <span className="text-primary">{t("hero.title2")}</span>
          </motion.h1>

          <motion.p
            variants={fadeUp}
            initial="hidden"
            animate="show"
            custom={2}
            className="mx-auto mt-6 max-w-xl text-base leading-relaxed text-neutral-600 sm:text-lg lg:mx-0"
          >
            {t("hero.subtitle")}
          </motion.p>

          <motion.div
            variants={fadeUp}
            initial="hidden"
            animate="show"
            custom={3}
            className="mt-8 flex flex-col items-center justify-center gap-4 sm:flex-row lg:justify-start"
          >
            <a
              href="#aplikasi"
              className="inline-flex w-full items-center justify-center gap-2 rounded-full bg-primary px-7 py-3.5 font-semibold text-white shadow-lg shadow-primary/30 transition hover:-translate-y-0.5 hover:bg-primary-dark sm:w-auto"
            >
              <Play className="h-4 w-4" />
              {t("hero.demo")}
            </a>
            <a
              href="https://github.com/LittleMardiah/G-Flow"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex w-full items-center justify-center gap-2 rounded-full border border-neutral-300 bg-white px-7 py-3.5 font-semibold text-neutral-700 transition hover:-translate-y-0.5 hover:border-primary hover:text-primary sm:w-auto"
            >
              <GitHubIcon className="h-4 w-4" />
              {t("hero.github")}
            </a>
          </motion.div>

          <motion.dl
            variants={fadeUp}
            initial="hidden"
            animate="show"
            custom={4}
            className="mt-12 grid grid-cols-3 gap-4 border-t border-neutral-200 pt-8"
          >
            {STATS.map((stat) => (
              <div key={stat.label} className="text-center lg:text-left">
                <dt className="order-2 text-xs font-medium text-neutral-500">
                  {t(`hero.${stat.label}`)}
                </dt>
                <dd className="order-1 text-2xl font-extrabold text-primary sm:text-3xl">
                  {t(`hero.${stat.value}`)}
                </dd>
              </div>
            ))}
          </motion.dl>
        </div>

        <motion.div
          initial={{ opacity: 0, y: 40 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, ease: "easeOut", delay: 0.2 }}
          className="relative mx-auto flex w-full max-w-md items-end justify-center lg:max-w-none"
        >
          {PHONES.map((phone) => (
            <div key={phone.src} className={`relative w-1/3 shrink-0 transition-transform duration-300 hover:scale-105 ${phone.className}`}>
              <Image
                src={phone.src}
                alt={phone.alt}
                width={300}
                height={620}
                className="h-auto w-full drop-shadow-2xl"
                priority={phone.src === "/mockups/phone-food.svg"}
              />
            </div>
          ))}
        </motion.div>
      </div>
    </section>
  );
}