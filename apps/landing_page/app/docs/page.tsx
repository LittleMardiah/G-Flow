import Link from "next/link";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";

const STACK = [
  { label: "Backend", value: "Go (REST API)" },
  { label: "Merchant / Driver / Customer", value: "Flutter" },
  { label: "Landing & Web", value: "Next.js" },
  { label: "Database", value: "PostgreSQL (Supabase)" },
];

export default function DocsPage() {
  return (
    <>
      <Navbar />
      <main className="w-full bg-background">
        <section className="mx-auto max-w-[1240px] px-4 pb-20 pt-28 lg:px-8 lg:pt-32">
          <Link
            href="/"
            className="mb-6 inline-flex items-center gap-2 font-body text-sm text-text-secondary transition-colors hover:text-text-primary"
          >
            ← Kembali ke Beranda
          </Link>

          <h1 className="font-display text-4xl font-bold tracking-tight text-text-primary sm:text-5xl">
            Developer Docs
          </h1>
          <p className="mt-4 max-w-2xl font-body text-lg text-text-secondary">
            Dokumentasi teknis G-Flow. Sumber kode terbuka tersedia di GitHub.
          </p>

          <div className="mt-10 rounded-2xl border border-border-subtle bg-surface p-8">
            <h2 className="font-display text-2xl font-semibold text-text-primary">
              Repositori
            </h2>
            <p className="mt-3 font-body text-text-secondary">
              Seluruh kode (backend Go, aplikasi Flutter, dan landing page ini)
              dapat diakses pada repositori publik di GitHub.
            </p>
            <a
              href="https://github.com/LittleMardiah/G-Flow"
              target="_blank"
              rel="noopener noreferrer"
              className="mt-5 inline-flex items-center gap-2 rounded-full bg-primary-container px-6 py-3 font-body font-semibold text-surface-container-lowest transition-all hover:opacity-90"
            >
              Buka GitHub
            </a>
          </div>

          <div className="mt-10 rounded-2xl border border-border-subtle bg-surface p-8">
            <h2 className="font-display text-2xl font-semibold text-text-primary">
              Arsitektur &amp; Teknologi
            </h2>
            <ul className="mt-5 space-y-3">
              {STACK.map((item) => (
                <li
                  key={item.label}
                  className="flex flex-col gap-1 rounded-xl border border-border-subtle bg-surface-elevated p-4 sm:flex-row sm:items-center sm:justify-between"
                >
                  <span className="font-body text-sm text-text-muted">{item.label}</span>
                  <span className="font-body text-sm font-medium text-text-primary">{item.value}</span>
                </li>
              ))}
            </ul>
          </div>

          <div className="mt-10 grid grid-cols-1 gap-8 md:grid-cols-2">
            <div id="privasi" className="scroll-mt-24 rounded-2xl border border-border-subtle bg-surface p-8">
              <h2 className="font-display text-2xl font-semibold text-text-primary">
                Kebijakan Privasi
              </h2>
              <p className="mt-3 font-body text-text-secondary">
                Dokumen kebijakan privasi sedang dalam penyusunan. Kami berkomitmen
                untuk transparan dalam pengelolaan data pengguna sejak tahap
                paling awal.
              </p>
              <p className="mt-3 font-label text-text-muted">Status: Sedang disusun</p>
            </div>

            <div id="syarat" className="scroll-mt-24 rounded-2xl border border-border-subtle bg-surface p-8">
              <h2 className="font-display text-2xl font-semibold text-text-primary">
                Syarat &amp; Ketentuan
              </h2>
              <p className="mt-3 font-body text-text-secondary">
                Dokumen syarat dan ketentuan sedang dalam penyusunan dan akan
                dipublikasikan sebelum layanan resmi diluncurkan.
              </p>
              <p className="mt-3 font-label text-text-muted">Status: Sedang disusun</p>
            </div>
          </div>

          <div className="mt-10 rounded-2xl border border-border-subtle bg-surface-container-lowest p-8 text-center">
            <p className="font-body text-text-secondary">
              Ada pertanyaan teknis atau ingin bermitra?
            </p>
            <Link
              href="/#kontak"
              className="mt-4 inline-flex items-center gap-2 rounded-full bg-primary-container px-6 py-3 font-body font-semibold text-surface-container-lowest transition-all hover:opacity-90"
            >
              Hubungi Kami
            </Link>
          </div>
        </section>
      </main>
      <Footer />
    </>
  );
}