"use client";

import { useState } from "react";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { Reveal } from "@/components/Reveal";

export default function CTA() {
  const [sent, setSent] = useState(false);

  function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const data = new FormData(form);
    const name = String(data.get("name") ?? "");
    const contact = String(data.get("contact") ?? "");
    const category = String(data.get("category") ?? "");
    const message = String(data.get("message") ?? "");

    const subject = encodeURIComponent(`Pengajuan Kemitraan — ${name}`);
    const body = encodeURIComponent(
      `Nama: ${name}\nKontak: ${contact}\nKategori: ${category}\nPesan:\n${message}`
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
                Hubungi Kami
              </span>
              <h2 className="mb-5 font-display text-[28px] font-bold leading-[1.2] tracking-tight text-text-primary sm:text-4xl">
                Mari Bangun Ekosistem Bersama G-Flow
              </h2>
              <p className="mb-10 font-body text-text-secondary">
                Punya pertanyaan seputar kemitraan korporat, integrasi armada
                skala besar, atau dukungan operasional? Tim kami siap
                berdiskusi langsung dengan Anda.
              </p>

              <div className="mb-10 space-y-6">
                <div className="flex items-start gap-4">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-border-subtle bg-surface-elevated text-primary-container">
                    <MaterialIcon name="mail" className="text-[20px]" />
                  </div>
                  <div>
                    <span className="block font-label text-text-muted">
                      Surel Kemitraan
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
                      Telepon Operasional
                    </span>
                    <p className="font-body text-text-primary">+62 21 5088 2900</p>
                    <p className="font-label text-text-muted">
                      Senin – Jumat, 08.00 – 18.00 WIB
                    </p>
                  </div>
                </div>

                <div className="flex items-start gap-4">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-border-subtle bg-surface-elevated text-primary-container">
                    <MaterialIcon name="apartment" className="text-[20px]" />
                  </div>
                  <div>
                    <span className="block font-label text-text-muted">Kantor Pusat</span>
                    <p className="font-body text-text-primary">
                      G-Flow Tower, Kawasan SCBD Lot 28
                    </p>
                    <p className="font-label text-text-muted">
                      Jl. Jend. Sudirman Kav. 52-53, Jakarta Selatan 12190
                    </p>
                  </div>
                </div>
              </div>
            </div>

            <Reveal>
              <div className="flex items-center gap-3 rounded-xl border border-border-subtle bg-surface p-4">
                <MaterialIcon name="verified_user" className="text-[22px] text-success-dot" />
                <p className="font-label text-text-secondary">
                  Respon resmi kemitraan dijamin dalam kurun waktu 1x24 jam kerja.
                </p>
              </div>
            </Reveal>
          </div>

          <div className="lg:col-span-7">
            <Reveal delay={0.1}>
              <div className="relative rounded-2xl border border-border-subtle bg-surface p-6 shadow-xl sm:p-10">
                <h3 className="font-display text-2xl font-semibold text-text-primary">
                  Formulir Kemitraan
                </h3>
                <p className="mb-8 font-body text-sm text-text-secondary">
                  Lengkapi informasi awal bisnis atau instansi Anda. Formulir
                  ini akan membuka aplikasi email Anda dan menyusun pengajuan
                  ke kemitraan@gflow.id.
                </p>

                <form className="space-y-5" onSubmit={handleSubmit}>
                  <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
                    <div className="space-y-2">
                      <label className="font-label text-text-secondary" htmlFor="name">
                        Nama Lengkap
                      </label>
                      <input
                        id="name"
                        name="name"
                        type="text"
                        required
                        placeholder="Contoh: Faris Adrian"
                        className={inputClass}
                      />
                    </div>
                    <div className="space-y-2">
                      <label className="font-label text-text-secondary" htmlFor="contact">
                        Email Bisnis / WhatsApp
                      </label>
                      <input
                        id="contact"
                        name="contact"
                        type="text"
                        required
                        placeholder="faris@perusahaan.co.id"
                        className={inputClass}
                      />
                    </div>
                  </div>

                  <div className="space-y-2">
                    <label className="font-label text-text-secondary" htmlFor="category">
                      Kategori Kemitraan
                    </label>
                    <div className="relative">
                      <select
                        id="category"
                        name="category"
                        defaultValue="armada"
                        className={`${inputClass} cursor-pointer appearance-none`}
                      >
                        <option className="bg-surface" value="armada">
                          Mitra Armada Korporat
                        </option>
                        <option className="bg-surface" value="merchant">
                          Integrasi Merchant Nasional
                        </option>
                        <option className="bg-surface" value="paypulse">
                          Kerjasama Ekosistem PayPulse
                        </option>
                        <option className="bg-surface" value="umum">
                          Pertanyaan Umum &amp; Media
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
                      Pesan / Kebutuhan Kerjasama
                    </label>
                    <textarea
                      id="message"
                      name="message"
                      required
                      rows={4}
                      placeholder="Jelaskan secara singkat jenis kemitraan yang ingin Anda jalankan bersama G-Flow..."
                      className={`${inputClass} resize-none`}
                    />
                  </div>

                  <button
                    type="submit"
                    className="flex w-full items-center justify-center gap-2 rounded-full bg-primary-container py-4 font-body font-semibold text-surface-container-lowest shadow-[0_4px_16px_rgba(255,149,0,0.25)] transition-all duration-200 hover:opacity-90"
                  >
                    <span>Kirim Pengajuan Kemitraan</span>
                    <MaterialIcon name="send" className="text-[18px]" />
                  </button>
                </form>

                {sent && (
                  <p className="mt-4 rounded-lg border border-emerald-800 bg-emerald-950/40 p-3 text-center font-label text-success-dot">
                    Aplikasi email telah dibuka. Kami tunggu pengajuan Anda di
                    kemitraan@gflow.id.
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