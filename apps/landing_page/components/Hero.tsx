"use client";

import { motion } from "framer-motion";
import Link from "next/link";
import { MaterialIcon, MaterialIconName } from "@/components/ui/MaterialIcon";
import { useLang } from "@/app/i18n/config";

const fadeUp = {
  hidden: { opacity: 0, y: 28 },
  show: (i: number) => ({
    opacity: 1,
    y: 0,
    transition: { duration: 0.6, ease: "easeOut" as const, delay: 0.12 * i },
  }),
};

const STATS: { icon: MaterialIconName; key: string }[] = [
  { icon: "star", key: "rating" },
  { icon: "verified_user", key: "transparent" },
  { icon: "location_on", key: "realtime" },
];

export default function Hero() {
  const { t } = useLang();

  return (
    <section
      id="beranda"
      className="relative overflow-hidden border-b border-border-subtle bg-surface-container-lowest"
    >
      <div className="pointer-events-none absolute left-1/2 top-1/4 h-[550px] w-[850px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-zinc-glow opacity-40 blur-[140px]" />
      <div className="pointer-events-none absolute -top-24 right-12 h-[320px] w-[320px] rounded-full bg-accent-glow blur-[100px]" />

      <div className="relative z-10 mx-auto grid max-w-[1240px] grid-cols-1 items-center gap-12 px-4 pb-20 pt-28 lg:grid-cols-12 lg:gap-10 lg:px-8 lg:pt-32 lg:pb-32">
        <div className="flex flex-col items-start lg:col-span-7">
          <motion.div
            variants={fadeUp}
            initial="hidden"
            animate="show"
            custom={0}
            className="mb-6 inline-flex items-center gap-2.5 rounded-full border border-border-subtle bg-surface-elevated px-3.5 py-1.5"
          >
            <span className="h-2 w-2 animate-pulse rounded-full bg-success-dot" />
            <span className="font-label text-text-secondary">
              {t("hero.badge")}
            </span>
          </motion.div>

          <motion.h1
            variants={fadeUp}
            initial="hidden"
            animate="show"
            custom={1}
            className="font-display text-4xl font-bold leading-[1.1] tracking-tight text-text-primary sm:text-5xl lg:text-6xl"
          >
            {t("hero.title")}
            <br />
            <span className="text-text-secondary">{t("hero.titleAccent")}</span>
          </motion.h1>

          <motion.p
            variants={fadeUp}
            initial="hidden"
            animate="show"
            custom={2}
            className="mb-9 mt-4 max-w-xl font-body text-lg leading-relaxed text-text-secondary"
          >
            {t("hero.subtitle")}
          </motion.p>

          <motion.div
            variants={fadeUp}
            initial="hidden"
            animate="show"
            custom={3}
            className="mb-14 flex flex-wrap items-center gap-4"
          >
            <Link
              href="#layanan"
              className="inline-flex items-center justify-center gap-2 rounded-full bg-primary-container px-7 py-3.5 font-body font-semibold text-surface-container-lowest shadow-[0_8px_24px_rgba(255,149,0,0.28)] transition-all duration-200 hover:-translate-y-0.5 hover:opacity-90"
            >
              <span>{t("hero.ctaPrimary")}</span>
              <MaterialIcon name="arrow_forward" className="text-[18px]" />
            </Link>
            <Link
              href="#layanan"
              className="inline-flex items-center justify-center rounded-full border border-border-subtle bg-transparent px-7 py-3.5 font-body text-text-secondary transition-all duration-200 hover:bg-surface-elevated hover:text-text-primary"
            >
              {t("hero.ctaSecondary")}
            </Link>
          </motion.div>

          <motion.div
            variants={fadeUp}
            initial="hidden"
            animate="show"
            custom={4}
            className="grid w-full grid-cols-3 gap-4 border-t border-border-subtle pt-8"
          >
            {STATS.map((stat, i) => (
              <div
                key={stat.key}
                className={i === 0 ? "" : "border-l border-border-subtle pl-4 sm:pl-6"}
              >
                <div className="mb-1 flex items-center gap-1 text-primary-container">
                  <MaterialIcon name={stat.icon} className="text-[18px]" fill />
                    <span className="font-display text-2xl font-semibold text-text-primary">
                      {t(`hero.stats.${stat.key}.title`)}
                    </span>
                  </div>
                  <p className="font-label text-[11px] text-text-muted">{t(`hero.stats.${stat.key}.desc`)}</p>
              </div>
            ))}
          </motion.div>
        </div>

        <div className="group relative flex min-h-[460px] items-center justify-center py-4 lg:col-span-5 lg:min-h-[520px]">
          {/* Mockup 1 — G-Food */}
          <div className="absolute z-10 w-[240px] -translate-x-16 translate-y-3 -rotate-6 rounded-2xl border border-border-subtle bg-surface p-4 shadow-2xl transition-all duration-500 ease-out group-hover:-translate-x-20 group-hover:-rotate-3 sm:w-[260px] sm:-translate-x-24">
            <div className="mb-3 flex items-center justify-between border-b border-border-subtle pb-3">
              <div className="flex items-center gap-2">
                <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-accent-soft text-primary-container">
                  <MaterialIcon name="restaurant" className="text-[16px]" />
                </div>
                <span className="font-display text-[13px] text-text-primary">{t("hero.mockups.food.title")}</span>
              </div>
              <span className="rounded-full bg-accent-soft px-2 py-0.5 font-label text-[10px] text-primary-container">
                {t("hero.mockups.food.status")}
              </span>
            </div>
            <div className="space-y-2">
              <div className="flex items-center justify-between text-[12px] text-text-primary">
                <span className="font-medium">{t("hero.mockups.food.item")}</span>
                <span className="text-text-muted">–</span>
              </div>
              <div className="h-1.5 w-full overflow-hidden rounded-full bg-surface-container-high">
                <div className="h-full w-3/4 rounded-full bg-primary-container" />
              </div>
              <div className="flex items-center justify-between pt-1 text-[11px] text-text-muted">
                <span>{t("hero.mockups.food.driver")}</span>
                <span className="font-medium text-primary-container">{t("hero.mockups.food.eta")}</span>
              </div>
            </div>
          </div>

          {/* Mockup 2 — G-Send Instant */}
          <div className="absolute z-10 w-[240px] translate-x-16 translate-y-4 rotate-6 rounded-2xl border border-border-subtle bg-surface p-4 shadow-2xl transition-all duration-500 ease-out group-hover:translate-x-20 group-hover:rotate-3 sm:w-[260px] sm:translate-x-24">
            <div className="mb-3 flex items-center justify-between border-b border-border-subtle pb-3">
              <div className="flex items-center gap-2">
                <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-accent-soft text-primary-container">
                  <MaterialIcon name="local_shipping" className="text-[16px]" />
                </div>
                <span className="font-display text-[13px] text-text-primary">{t("hero.mockups.send.title")}</span>
              </div>
              <span className="flex items-center gap-1 font-label text-[10px] text-success-dot">
                <span className="h-1.5 w-1.5 rounded-full bg-success-dot" />
                {t("hero.mockups.send.status")}
              </span>
            </div>
            <div className="mb-2 rounded-lg border border-border-subtle/50 bg-surface-container-low p-2.5">
              <p className="font-label text-[11px] uppercase text-text-muted">{t("hero.mockups.send.trackingLabel")}</p>
              <p className="font-mono text-[12px] tracking-wider text-text-primary">
                GF-000000-JKT
              </p>
            </div>
            <div className="flex items-center justify-between text-[11px] text-text-secondary">
              <span>{t("hero.mockups.send.destination")}</span>
              <span className="font-medium text-primary-container">{t("hero.mockups.send.otp")}</span>
            </div>
          </div>

          {/* Mockup 3 — PayPulse */}
          <div className="relative z-20 w-[280px] rounded-[28px] border border-zinc-700/60 bg-surface-elevated p-5 shadow-[0_25px_60px_-15px_rgba(0,0,0,0.9)] transition-transform duration-500 ease-out group-hover:-translate-y-2 sm:w-[300px]">
            <div className="mb-5 flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="flex h-8 w-8 items-center justify-center rounded-full border border-border-subtle bg-surface-container text-primary-container">
                  <MaterialIcon name="account_circle" className="text-[18px]" />
                </div>
                <div>
                  <p className="font-label text-[10px] leading-tight text-text-muted">{t("hero.mockups.pay.welcome")}</p>
                  <p className="font-display text-[13px] text-text-primary">{t("hero.mockups.pay.user")}</p>
                </div>
              </div>
              <MaterialIcon name="notifications" className="text-[20px] text-text-secondary" />
            </div>

            <div className="mb-4 rounded-xl border border-border-subtle bg-surface p-3.5">
              <span className="mb-0.5 block font-label text-text-muted">{t("hero.mockups.pay.balanceLabel")}</span>
              <div className="flex items-baseline justify-between">
                <span className="font-display text-2xl font-semibold tracking-tight text-text-primary">
                  Rp 0
                </span>
              </div>
              <div className="mt-3 grid grid-cols-3 gap-2 border-t border-border-subtle pt-3">
                <span className="flex flex-col items-center gap-1 rounded bg-surface-container-high/60 py-1">
                  <MaterialIcon name="arrow_upward" className="text-[16px] text-primary-container" />
                  <span className="font-label text-[10px] text-text-secondary">{t("hero.mockups.pay.actionPay")}</span>
                </span>
                <span className="flex flex-col items-center gap-1 rounded bg-surface-container-high/60 py-1">
                  <MaterialIcon name="add_card" className="text-[16px] text-primary-container" />
                  <span className="font-label text-[10px] text-text-secondary">{t("hero.mockups.pay.actionTopUp")}</span>
                </span>
                <span className="flex flex-col items-center gap-1 rounded bg-surface-container-high/60 py-1">
                  <MaterialIcon name="receipt_long" className="text-[16px] text-primary-container" />
                  <span className="font-label text-[10px] text-text-secondary">{t("hero.mockups.pay.actionHistory")}</span>
                </span>
              </div>
            </div>

            <div className="mb-4 grid grid-cols-4 gap-2">
              {[
                { icon: "two_wheeler", label: "G-Ride" },
                { icon: "directions_car", label: "G-Car" },
                { icon: "restaurant", label: "G-Food" },
                { icon: "inventory_2", label: "G-Send" },
              ].map((service) => (
                <div key={service.label} className="flex flex-col items-center gap-1">
                  <div className="flex h-11 w-11 items-center justify-center rounded-xl border border-border-subtle bg-accent-soft text-primary-container">
                    <MaterialIcon name={service.icon as MaterialIconName} className="text-[20px]" />
                  </div>
                  <span className="font-label text-[11px] text-text-secondary">{service.label}</span>
                </div>
              ))}
            </div>

            <div className="flex items-center justify-between rounded-xl border border-border-subtle bg-surface-container-lowest/80 p-3">
              <div className="flex items-center gap-2.5">
                <div className="flex h-7 w-7 items-center justify-center rounded-full bg-emerald-950 text-success-dot">
                  <MaterialIcon name="check_circle" className="text-[15px]" />
                </div>
                <div>
                  <p className="font-label text-[11px] text-text-primary">{t("hero.mockups.pay.rideDone")}</p>
                  <p className="font-label text-[10px] text-text-muted">{t("hero.mockups.pay.rideSaved")}</p>
                </div>
              </div>
              <span className="font-label text-[11px] text-text-secondary">–</span>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}