"use client";

import { motion } from "framer-motion";
import Image from "next/image";
import { GitHubIcon } from "@/components/icons";
import { Button } from "@/components/ui/Button";
import { useLang } from "@/app/i18n/config";

const PHONES = [
  {
    src: "/mockups/phone-ride.svg",
    alt: "G-Flow driver app",
    className: "left-[2%] top-[7%] z-10 w-[50%] -rotate-[8deg]",
  },
  {
    src: "/mockups/phone-food.svg",
    alt: "G-Flow customer app",
    className: "left-[26%] top-0 z-20 w-[50%] scale-105",
  },
  {
    src: "/mockups/phone-wallet.svg",
    alt: "G-Flow wallet app",
    className: "right-[2%] top-[7%] z-10 w-[50%] rotate-[8deg]",
  },
];

const fadeUp = {
  hidden: { opacity: 0, y: 28 },
  show: (i: number) => ({
    opacity: 1,
    y: 0,
    transition: { duration: 0.6, ease: "easeOut" as const, delay: 0.12 * i },
  }),
};

export default function Hero() {
  const { t } = useLang();

  return (
    <section
      id="beranda"
      className="relative overflow-hidden bg-gradient-to-b from-accent/60 via-white to-white pt-16"
    >
      <div className="pointer-events-none absolute inset-0" aria-hidden="true">
        <div className="absolute -left-32 top-16 h-96 w-96 rounded-full bg-primary/10 blur-3xl" />
        <div className="absolute -right-24 top-40 h-[28rem] w-[28rem] rounded-full bg-secondary/20 blur-3xl" />
      </div>

      <div className="relative mx-auto grid max-w-7xl items-center gap-14 px-4 pb-20 pt-14 sm:px-6 lg:grid-cols-[3fr_2fr] lg:gap-8 lg:px-8 lg:pt-24">
        <div className="max-w-xl text-center lg:text-left">
          <motion.span
            variants={fadeUp}
            initial="hidden"
            animate="show"
            custom={0}
            className="inline-flex items-center gap-2 rounded-button border border-neutral-200 bg-white/70 px-4 py-1.5 text-sm tracking-wide text-text-support"
          >
            <span className="h-1.5 w-1.5 rounded-full bg-secondary" />
            {t("hero.badge")}
          </motion.span>

          <motion.h1
            variants={fadeUp}
            initial="hidden"
            animate="show"
            custom={1}
            className="mt-6 text-5xl font-extrabold leading-tight tracking-tight text-text-primary sm:text-6xl"
          >
            {t("hero.title")}
          </motion.h1>

          <motion.p
            variants={fadeUp}
            initial="hidden"
            animate="show"
            custom={2}
            className="mt-6 text-lg leading-relaxed text-text-support sm:text-xl"
          >
            {t("hero.subtitle")}
          </motion.p>

          <motion.div
            variants={fadeUp}
            initial="hidden"
            animate="show"
            custom={3}
            className="mt-9 flex flex-col items-center justify-center gap-4 sm:flex-row lg:justify-start"
          >
            <Button href="#aplikasi" size="lg" className="w-full sm:w-auto">
              {t("hero.demo")}
            </Button>
            <Button
              href="https://github.com/LittleMardiah/G-Flow"
              target="_blank"
              rel="noopener noreferrer"
              variant="outline"
              size="lg"
              className="w-full sm:w-auto"
            >
              <GitHubIcon className="h-4 w-4" />
              {t("hero.github")}
            </Button>
          </motion.div>
        </div>

        <motion.div
          initial={{ opacity: 0, y: 40 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, ease: "easeOut", delay: 0.25 }}
          className="relative mx-auto h-[540px] w-full max-w-[540px] sm:h-[580px]"
        >
          <div className="absolute left-1/2 top-1/2 h-80 w-80 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary/10 blur-3xl" />
          <div className="absolute bottom-8 left-10 h-40 w-40 rounded-full bg-secondary/20 blur-2xl" />

          {PHONES.map((phone) => (
            <div
              key={phone.src}
              className={`absolute transition-transform duration-500 hover:scale-105 ${phone.className}`}
            >
              <Image
                src={phone.src}
                alt={phone.alt}
                width={300}
                height={620}
                className="h-auto w-full drop-shadow-[0_30px_50px_rgba(10,10,10,0.25)]"
                priority={phone.src === "/mockups/phone-food.svg"}
              />
            </div>
          ))}
        </motion.div>
      </div>
    </section>
  );
}