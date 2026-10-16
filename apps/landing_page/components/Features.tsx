"use client";

import Link from "next/link";
import { MaterialIcon, MaterialIconName } from "@/components/ui/MaterialIcon";
import { Reveal } from "@/components/Reveal";
import { useLang } from "@/app/i18n/config";

function CheckItem({ children }: { children: React.ReactNode }) {
  return (
    <li className="flex items-center gap-2">
      <span className="text-text-muted">•</span>
      <span>{children}</span>
    </li>
  );
}

function CardIcon({ icon }: { icon: MaterialIconName }) {
  return (
    <div className="flex h-11 w-11 items-center justify-center rounded-xl border border-border-subtle bg-accent-soft text-primary-container">
      <MaterialIcon name={icon} className="text-[24px]" />
    </div>
  );
}

export default function Features() {
  const { t } = useLang();

  return (
    <>
      {/* LAYANAN */}
      <section id="layanan" className="relative w-full border-b border-border-subtle bg-background py-20 lg:py-28">
        <div className="mx-auto max-w-[1240px] px-4 lg:px-8">
          <Reveal className="mb-16 max-w-2xl">
            <span className="mb-3 block font-label font-semibold uppercase tracking-wider text-primary-container">
              {t("features.main.badge")}
            </span>
            <h2 className="font-display text-[28px] font-bold leading-[1.2] tracking-tight text-text-primary sm:text-4xl">
              {t("features.main.title")}
            </h2>
            <p className="font-body text-text-secondary">
              {t("features.main.subtitle")}
            </p>
          </Reveal>

          <div className="space-y-6">
            <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
              {/* G-Ride & G-Car */}
              <Reveal className="lg:col-span-2">
                <article className="relative flex h-full flex-col justify-between overflow-hidden rounded-2xl border border-border-subtle bg-surface p-6 transition-all duration-300 hover:-translate-y-1 hover:border-border-hover sm:p-8">
                  <div>
                    <div className="mb-6 flex items-center justify-between">
                      <CardIcon icon="moped" />
                      <span className="font-label font-medium uppercase tracking-wider text-primary-container">
                        {t("features.rideCard.badge")}
                      </span>
                    </div>
                    <h3 className="font-display text-2xl font-semibold text-text-primary">
                      {t("features.rideCard.title")}
                    </h3>
                    <p className="mb-6 max-w-xl font-body text-text-secondary">
                      {t("features.rideCard.desc")}
                    </p>
                    <ul className="mb-8 space-y-2.5 font-body text-sm text-text-secondary">
                      <CheckItem>{t("features.rideCard.check1")}</CheckItem>
                      <CheckItem>{t("features.rideCard.check2")}</CheckItem>
                      <CheckItem>{t("features.rideCard.check3")}</CheckItem>
                    </ul>
                  </div>
                  <div className="mt-4 flex flex-col justify-between gap-4 rounded-xl border border-border-subtle bg-surface-elevated p-4 sm:flex-row sm:items-center">
                    <div className="flex items-center gap-3">
                      <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-surface-container text-primary-container">
                        <MaterialIcon name="near_me" className="text-[20px]" />
                      </div>
                      <div>
                        <div className="font-display text-[14px] text-text-primary">
                          {t("features.rideCard.route")}
                        </div>
                        <div className="font-label text-[12px] text-text-muted">
                          {t("features.rideCard.routeSub")}
                        </div>
                      </div>
                    </div>
                    <div className="inline-flex shrink-0 items-center gap-2 rounded-full border border-border-subtle bg-accent-soft px-3 py-1.5">
                      <span className="h-2 w-2 rounded-full bg-primary-container" />
                      <span className="font-label text-[12px] font-semibold text-primary-container">
                        {t("features.rideCard.tag")}
                      </span>
                    </div>
                  </div>
                </article>
              </Reveal>

              {/* G-Food */}
              <Reveal delay={0.1}>
                <article className="flex h-full flex-col justify-between rounded-2xl border border-border-subtle bg-surface p-6 transition-all duration-300 hover:-translate-y-1 hover:border-border-hover sm:p-8">
                  <div>
                    <div className="mb-6 flex items-center justify-between">
                      <CardIcon icon="soup_kitchen" />
                      <span className="font-label font-medium uppercase tracking-wider text-primary-container">
                        {t("features.food.badge")}
                      </span>
                    </div>
                    <h3 className="mb-3 font-display text-2xl font-semibold text-text-primary">
                      {t("features.food.title")}
                    </h3>
                    <p className="mb-6 font-body text-text-secondary">
                      {t("features.food.desc")}
                    </p>
                    <ul className="mb-6 space-y-2.5 font-body text-sm text-text-secondary">
                      <CheckItem>{t("features.food.check1")}</CheckItem>
                      <CheckItem>{t("features.food.check2")}</CheckItem>
                      <CheckItem>{t("features.food.check3")}</CheckItem>
                    </ul>
                  </div>
                  <div className="flex items-center justify-between border-t border-border-subtle pt-4 font-label text-text-muted">
                    <span>{t("features.food.footerLeft")}</span>
                    <span className="font-medium text-success-dot">{t("features.food.footerRight")}</span>
                  </div>
                </article>
              </Reveal>
            </div>

            <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
              {/* G-Send */}
              <Reveal>
                <article className="flex h-full flex-col justify-between rounded-2xl border border-border-subtle bg-surface p-6 transition-all duration-300 hover:-translate-y-1 hover:border-border-hover sm:p-8">
                  <div>
                    <div className="mb-6 flex items-center justify-between">
                      <CardIcon icon="inventory_2" />
                      <span className="font-label font-medium uppercase tracking-wider text-primary-container">
                        {t("features.send.badge")}
                      </span>
                    </div>
                    <h3 className="mb-3 font-display text-2xl font-semibold text-text-primary">
                      {t("features.send.title")}
                    </h3>
                    <p className="mb-6 font-body text-text-secondary">
                      {t("features.send.desc")}
                    </p>
                    <ul className="mb-6 space-y-2.5 font-body text-sm text-text-secondary">
                      <CheckItem>{t("features.send.check1")}</CheckItem>
                      <CheckItem>{t("features.send.check2")}</CheckItem>
                      <CheckItem>{t("features.send.check3")}</CheckItem>
                    </ul>
                  </div>
                  <div className="flex items-center justify-between border-t border-border-subtle pt-4 font-label text-text-muted">
                    <span>{t("features.send.footerLeft")}</span>
                    <span className="font-medium text-primary-container">{t("features.send.footerRight")}</span>
                  </div>
                </article>
              </Reveal>

              {/* PayPulse */}
              <Reveal delay={0.1} className="lg:col-span-2">
                <article className="relative flex h-full flex-col justify-between overflow-hidden rounded-2xl border border-border-subtle bg-surface p-6 transition-all duration-300 hover:-translate-y-1 hover:border-border-hover sm:p-8">
                  <div>
                    <div className="mb-6 flex items-center justify-between">
                      <CardIcon icon="account_balance_wallet" />
                      <span className="font-label font-medium uppercase tracking-wider text-primary-container">
                        {t("features.wallet.badge")}
                      </span>
                    </div>
                    <h3 className="font-display text-2xl font-semibold text-text-primary">
                      {t("features.wallet.title")}
                    </h3>
                    <p className="mb-6 max-w-xl font-body text-text-secondary">
                      {t("features.wallet.desc")}
                    </p>
                    <ul className="mb-8 space-y-2.5 font-body text-sm text-text-secondary">
                      <CheckItem>{t("features.wallet.check1")}</CheckItem>
                      <CheckItem>{t("features.wallet.check2")}</CheckItem>
                      <CheckItem>{t("features.wallet.check3")}</CheckItem>
                    </ul>
                  </div>
                  <div className="mt-4 flex flex-col justify-between gap-4 rounded-xl border border-border-subtle bg-surface-elevated p-4 sm:flex-row sm:items-center">
                    <div className="flex items-center gap-3">
                      <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-surface-container text-primary-container">
                        <MaterialIcon name="qr_code_scanner" className="text-[20px]" />
                      </div>
                      <div>
                        <div className="font-display text-[14px] text-text-primary">
                          {t("features.wallet.miniTitle")}
                        </div>
                        <div className="font-label text-[12px] text-text-muted">
                          {t("features.wallet.miniSub")}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      <div className="rounded border border-border-subtle bg-surface-container px-3 py-1 font-label text-[12px] text-text-secondary">
                        {t("features.wallet.badge1")}
                      </div>
                      <div className="rounded bg-primary-container px-3 py-1 font-label text-[12px] font-semibold text-surface-container-lowest">
                        {t("features.wallet.badge2")}
                      </div>
                    </div>
                  </div>
                </article>
              </Reveal>
            </div>
          </div>
        </div>
      </section>

      {/* MITRA */}
      <section id="mitra" className="w-full border-b border-border-subtle bg-surface-container-lowest py-20 lg:py-28">
        <div className="mx-auto max-w-[1240px] px-4 lg:px-8">
          <Reveal className="mx-auto mb-16 max-w-2xl text-center">
            <span className="mb-3 block font-label font-semibold uppercase tracking-wider text-primary-container">
              {t("features.mitra.badge")}
            </span>
            <h2 className="font-display text-[28px] font-bold leading-[1.2] tracking-tight text-text-primary sm:text-4xl">
              {t("features.mitra.title")}
            </h2>
            <p className="font-body text-text-secondary">
              {t("features.mitra.subtitle")}
            </p>
          </Reveal>

          <div className="grid grid-cols-1 gap-8 md:grid-cols-2">
            <Reveal>
              <article className="flex h-full flex-col justify-between rounded-2xl border border-border-subtle bg-surface p-8 transition-all duration-300 hover:-translate-y-1 hover:border-border-hover sm:p-10">
                <div>
                  <div className="mb-6 flex h-12 w-12 items-center justify-center rounded-xl border border-border-subtle bg-accent-soft text-primary-container">
                    <MaterialIcon name="sports_motorsports" className="text-[26px]" />
                  </div>
                  <h3 className="font-display text-2xl font-semibold text-text-primary">
                    {t("features.driver.title")}
                  </h3>
                  <p className="mb-6 font-body text-text-secondary">
                    {t("features.driver.desc")}
                  </p>
                  <ul className="mb-8 space-y-2 font-body text-sm text-text-secondary">
                    <li className="flex items-center gap-2">
                      <MaterialIcon name="check" className="text-[18px] text-primary-container" />
                      <span>{t("features.driver.bullet1")}</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <MaterialIcon name="check" className="text-[18px] text-primary-container" />
                      <span>{t("features.driver.bullet2")}</span>
                    </li>
                  </ul>
                </div>
                <Link
                  href="#kontak"
                  className="inline-flex items-center gap-2 font-display text-[15px] font-semibold text-primary-container transition-all duration-200 hover:gap-3"
                >
                  <span>{t("features.driver.cta")}</span>
                  <MaterialIcon name="arrow_forward" className="text-[18px]" />
                </Link>
              </article>
            </Reveal>

            <Reveal delay={0.1}>
              <article className="flex h-full flex-col justify-between rounded-2xl border border-border-subtle bg-surface p-8 transition-all duration-300 hover:-translate-y-1 hover:border-border-hover sm:p-10">
                <div>
                  <div className="mb-6 flex h-12 w-12 items-center justify-center rounded-xl border border-border-subtle bg-accent-soft text-primary-container">
                    <MaterialIcon name="storefront" className="text-[26px]" />
                  </div>
                  <h3 className="font-display text-2xl font-semibold text-text-primary">
                    {t("features.merchant.title")}
                  </h3>
                  <p className="mb-6 font-body text-text-secondary">
                    {t("features.merchant.desc")}
                  </p>
                  <ul className="mb-8 space-y-2 font-body text-sm text-text-secondary">
                    <li className="flex items-center gap-2">
                      <MaterialIcon name="check" className="text-[18px] text-primary-container" />
                      <span>{t("features.merchant.bullet1")}</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <MaterialIcon name="check" className="text-[18px] text-primary-container" />
                      <span>{t("features.merchant.bullet2")}</span>
                    </li>
                  </ul>
                </div>
                <Link
                  href="#kontak"
                  className="inline-flex items-center gap-2 font-display text-[15px] font-semibold text-primary-container transition-all duration-200 hover:gap-3"
                >
                  <span>{t("features.merchant.cta")}</span>
                  <MaterialIcon name="arrow_forward" className="text-[18px]" />
                </Link>
              </article>
            </Reveal>
          </div>
        </div>
      </section>
    </>
  );
}
