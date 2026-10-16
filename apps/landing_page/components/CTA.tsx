"use client";

import { useState } from "react";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { Reveal } from "@/components/Reveal";
import { useLang } from "@/app/i18n/config";

export default function CTA() {
  const [sent, setSent] = useState(false);
  const { t } = useLang();

  function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const data = new FormData(form);
    const name = String(data.get("name") ?? "");
    const contact = String(data.get("contact") ?? "");
    const category = String(data.get("category") ?? "");
    const message = String(data.get("message") ?? "");

    const subject = encodeURIComponent(t("cta.mailSubject", { name }));
    const body = encodeURIComponent(
      t("cta.mailBody", { name, contact, category, message })
    );
    window.location.href = `mailto:kemitraan@gflow.id?subject=${subject}&body=${body}`;
    setSent(true);
  }

  const inputClass =
    "w-full rounded-xl border border-border-subtle bg-surface-elevated px-4 py-3 font-body text-sm text-text-primary placeholder:text-text-muted transition-colors focus:border-primary-container focus:outline-none";

  return (
    <section id="kontak" className="relative w-full bg-background py-20 lg:py-28">
      <div className="mx-auto max-w-[1240px] px-4 lg:px-8">
        <div className="grid grid-cols-1 gap-12 lg:grid-cols-12 lg:gap-14">
          <div className="flex flex-col justify-between lg:col-span-5">
            <div>
              <span className="mb-3 block font-label font-semibold uppercase tracking-wider text-primary-container">
                {t("cta.eyebrow")}
              </span>
              <h2 className="mb-5 font-display text-[28px] font-bold leading-[1.2] tracking-tight text-text-primary sm:text-4xl">
                {t("cta.title")}
              </h2>
              <p className="mb-10 font-body text-text-secondary">
                {t("cta.subtitle")}
              </p>

              <div className="mb-10 space-y-6">
                <div className="flex items-start gap-4">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-border-subtle bg-surface-elevated text-primary-container">
                    <MaterialIcon name="mail" className="text-[20px]" />
                  </div>
                  <div>
                    <span className="block font-label text-text-muted">
                      {t("cta.emailLabel")}
                    </span>
                    <a
                      href="mailto:kemitraan@gflow.id"
                      className="font-body text-text-primary transition-colors hover:text-primary-container"
                    >
                      kemitraan@gflow.id
                    </a>
                  </div>
                </div>

                <div className="flex items-start gap-4">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-border-subtle bg-surface-elevated text-primary-container">
                    <MaterialIcon name="call" className="text-[20px]" />
                  </div>
                  <div>
                    <span className="block font-label text-text-muted">
                      {t("cta.phoneLabel")}
                    </span>
                    <p className="font-body text-text-primary">+62 21 5088 2900</p>
                    <p className="font-label text-text-muted">
                      {t("cta.phoneHours")}
                    </p>
                  </div>
                </div>

                <div className="flex items-start gap-4">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-border-subtle bg-surface-elevated text-primary-container">
                    <MaterialIcon name="apartment" className="text-[20px]" />
                  </div>
                  <div>
                    <span className="block font-label text-text-muted">{t("cta.officeLabel")}</span>
                    <p className="font-body text-text-primary">
                      {t("cta.officeLine1")}
                    </p>
                    <p className="font-label text-text-muted">
                      {t("cta.officeLine2")}
                    </p>
                  </div>
                </div>
              </div>
            </div>

            <Reveal>
              <div className="flex items-center gap-3 rounded-xl border border-border-subtle bg-surface p-4">
                <MaterialIcon name="verified_user" className="text-[22px] text-success-dot" />
                <p className="font-label text-text-secondary">
                  {t("cta.responseNote")}
                </p>
              </div>
            </Reveal>
          </div>

          <div className="lg:col-span-7">
            <Reveal delay={0.1}>
              <div className="relative rounded-2xl border border-border-subtle bg-surface p-6 shadow-xl sm:p-10">
                <h3 className="font-display text-2xl font-semibold text-text-primary">
                  {t("cta.formTitle")}
                </h3>
                <p className="mb-8 font-body text-sm text-text-secondary">
                  {t("cta.formDesc")}
                </p>

                <form className="space-y-5" onSubmit={handleSubmit}>
                  <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
                    <div className="space-y-2">
                      <label className="font-label text-text-secondary" htmlFor="name">
                        {t("cta.nameLabel")}
                      </label>
                      <input
                        id="name"
                        name="name"
                        type="text"
                        required
                        placeholder={t("cta.namePlaceholder")}
                        className={inputClass}
                      />
                    </div>
                    <div className="space-y-2">
                      <label className="font-label text-text-secondary" htmlFor="contact">
                        {t("cta.contactLabel")}
                      </label>
                      <input
                        id="contact"
                        name="contact"
                        type="text"
                        required
                        placeholder={t("cta.contactPlaceholder")}
                        className={inputClass}
                      />
                    </div>
                  </div>

                  <div className="space-y-2">
                    <label className="font-label text-text-secondary" htmlFor="category">
                      {t("cta.categoryLabel")}
                    </label>
                    <div className="relative">
                      <select
                        id="category"
                        name="category"
                        defaultValue="armada"
                        className={`${inputClass} cursor-pointer appearance-none`}
                      >
                        <option className="bg-surface" value="armada">
                          {t("cta.optionFleet")}
                        </option>
                        <option className="bg-surface" value="merchant">
                          {t("cta.optionMerchant")}
                        </option>
                        <option className="bg-surface" value="paypulse">
                          {t("cta.optionPaypulse")}
                        </option>
                        <option className="bg-surface" value="umum">
                          {t("cta.optionGeneral")}
                        </option>
                      </select>
                      <MaterialIcon
                        name="expand_more"
                        className="pointer-events-none absolute right-4 top-1/2 -translate-y-1/2 text-[18px] text-text-muted"
                      />
                    </div>
                  </div>

                  <div className="space-y-2">
                    <label className="font-label text-text-secondary" htmlFor="message">
                      {t("cta.messageLabel")}
                    </label>
                    <textarea
                      id="message"
                      name="message"
                      required
                      rows={4}
                      placeholder={t("cta.messagePlaceholder")}
                      className={`${inputClass} resize-none`}
                    />
                  </div>

                  <button
                    type="submit"
                    className="flex w-full items-center justify-center gap-2 rounded-full bg-primary-container py-4 font-body font-semibold text-surface-container-lowest shadow-[0_4px_16px_rgba(255,149,0,0.25)] transition-all duration-200 hover:opacity-90"
                  >
                    <span>{t("cta.submit")}</span>
                    <MaterialIcon name="send" className="text-[18px]" />
                  </button>
                </form>

                {sent && (
                  <p className="mt-4 rounded-lg border border-emerald-800 bg-emerald-950/40 p-3 text-center font-label text-success-dot">
                    {t("cta.success")}
                  </p>
                )}
              </div>
            </Reveal>
          </div>
        </div>
      </div>
    </section>
  );
}