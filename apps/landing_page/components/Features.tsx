import Link from "next/link";
import { MaterialIcon, MaterialIconName } from "@/components/ui/MaterialIcon";
import { Reveal } from "@/components/Reveal";

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
  return (
    <>
      {/* LAYANAN */}
      <section id="layanan" className="relative w-full border-b border-border-subtle bg-background py-20 lg:py-28">
        <div className="mx-auto max-w-[1240px] px-4 lg:px-8">
          <Reveal className="mb-16 max-w-2xl">
            <span className="mb-3 block font-label font-semibold uppercase tracking-wider text-primary-container">
              Ekosistem Utama
            </span>
            <h2 className="font-display text-[28px] font-bold leading-[1.2] tracking-tight text-text-primary sm:text-4xl">
              Layanan Harian yang Didesain untuk Efisiensi Waktu Anda.
            </h2>
            <p className="font-body text-text-secondary">
              Setiap fitur dibangun dengan fokus pada kecepatan respon,
              keandalan armada, dan transparansi tarif tanpa biaya tersembunyi.
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
                        G-Ride &amp; G-Car
                      </span>
                    </div>
                    <h3 className="font-display text-2xl font-semibold text-text-primary">
                      Mobilitas Cepat Tanpa Drama Macet
                    </h3>
                    <p className="mb-6 max-w-xl font-body text-text-secondary">
                      Penjemputan terdekat dengan armada terverifikasi dan
                      estimasi harga pasti di awal tanpa lonjakan tersembunyi.
                    </p>
                    <ul className="mb-8 space-y-2.5 font-body text-sm text-text-secondary">
                      <CheckItem>Penjemputan armada terverifikasi dengan estimasi jelas</CheckItem>
                      <CheckItem>Pelacakan lokasi presisi tinggi secara real-time</CheckItem>
                      <CheckItem>Proteksi keamanan terpasang di setiap perjalanan</CheckItem>
                    </ul>
                  </div>
                  <div className="mt-4 flex flex-col justify-between gap-4 rounded-xl border border-border-subtle bg-surface-elevated p-4 sm:flex-row sm:items-center">
                    <div className="flex items-center gap-3">
                      <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-surface-container text-primary-container">
                        <MaterialIcon name="near_me" className="text-[20px]" />
                      </div>
                      <div>
                        <div className="font-display text-[14px] text-text-primary">
                          Kuningan Sentral → Grand Indonesia
                        </div>
                        <div className="font-label text-[12px] text-text-muted">
                          Rute perkotaan • Estimasi tiba real-time
                        </div>
                      </div>
                    </div>
                    <div className="inline-flex shrink-0 items-center gap-2 rounded-full border border-border-subtle bg-accent-soft px-3 py-1.5">
                      <span className="h-2 w-2 rounded-full bg-primary-container" />
                      <span className="font-label text-[12px] font-semibold text-primary-container">
                        Tarif Tetap di Muka
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
                        G-Food
                      </span>
                    </div>
                    <h3 className="mb-3 font-display text-2xl font-semibold text-text-primary">
                      Kuliner Hangat Langsung ke Meja
                    </h3>
                    <p className="mb-6 font-body text-text-secondary">
                      Mitra kuliner terkurasi dengan kurir tas insulated untuk
                      menjaga temperatur makanan tetap prima.
                    </p>
                    <ul className="mb-6 space-y-2.5 font-body text-sm text-text-secondary">
                      <CheckItem>Kurasi resto lokal &amp; brand favorit</CheckItem>
                      <CheckItem>Estimasi masak &amp; antar akurat per menit</CheckItem>
                      <CheckItem>Promo ongkir jujur langsung di checkout</CheckItem>
                    </ul>
                  </div>
                  <div className="flex items-center justify-between border-t border-border-subtle pt-4 font-label text-text-muted">
                    <span>Tas Insulated Khusus</span>
                    <span className="font-medium text-success-dot">Higienis Terjaga</span>
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
                        G-Send
                      </span>
                    </div>
                    <h3 className="mb-3 font-display text-2xl font-semibold text-text-primary">
                      Kirim Dokumen &amp; Paket Tiba Hari Ini
                    </h3>
                    <p className="mb-6 font-body text-text-secondary">
                      Layanan logistik on-demand aman dengan bukti foto serah
                      terima dan kode verifikasi penerima langsung.
                    </p>
                    <ul className="mb-6 space-y-2.5 font-body text-sm text-text-secondary">
                      <CheckItem>Instant &amp; Same-Day delivery terjamin</CheckItem>
                      <CheckItem>Foto bukti serah terima langsung di app</CheckItem>
                      <CheckItem>Asuransi proteksi barang bernilai tinggi</CheckItem>
                    </ul>
                  </div>
                  <div className="flex items-center justify-between border-t border-border-subtle pt-4 font-label text-text-muted">
                    <span>Keamanan Validasi</span>
                    <span className="font-medium text-primary-container">OTP Wajib</span>
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
                        PayPulse by G-Flow
                      </span>
                    </div>
                    <h3 className="font-display text-2xl font-semibold text-text-primary">
                      Dompet Digital Cerdas &amp; Transaksi Sekali Sentuh
                    </h3>
                    <p className="mb-6 max-w-xl font-body text-text-secondary">
                      Satu saldo terintegrasi untuk seluruh kebutuhan mobilitas,
                      scan QRIS di merchant nasional, dan transfer antar bank
                      instan tanpa biaya ribet.
                    </p>
                    <ul className="mb-8 space-y-2.5 font-body text-sm text-text-secondary">
                      <CheckItem>Standar enkripsi perbankan berlapis dan otentikasi biometrik</CheckItem>
                      <CheckItem>Pembayaran QRIS instan di merchant nasional</CheckItem>
                      <CheckItem>Riwayat pengeluaran rapi dengan kategorisasi otomatis</CheckItem>
                    </ul>
                  </div>
                  <div className="mt-4 flex flex-col justify-between gap-4 rounded-xl border border-border-subtle bg-surface-elevated p-4 sm:flex-row sm:items-center">
                    <div className="flex items-center gap-3">
                      <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-surface-container text-primary-container">
                        <MaterialIcon name="qr_code_scanner" className="text-[20px]" />
                      </div>
                      <div>
                        <div className="font-display text-[14px] text-text-primary">
                          QRIS Nasional Siap Pakai
                        </div>
                        <div className="font-label text-[12px] text-text-muted">
                          Kompatibel dengan gerai QRIS di seluruh Indonesia
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      <div className="rounded border border-border-subtle bg-surface-container px-3 py-1 font-label text-[12px] text-text-secondary">
                        Program Poin
                      </div>
                      <div className="rounded bg-primary-container px-3 py-1 font-label text-[12px] font-semibold text-surface-container-lowest">
                        Scan Cepat
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
              Peluang Pertumbuhan
            </span>
            <h2 className="font-display text-[28px] font-bold leading-[1.2] tracking-tight text-text-primary sm:text-4xl">
              Tumbuh Nyata Bersama Jaringan G-Flow
            </h2>
            <p className="font-body text-text-secondary">
              Wujudkan kemandirian finansial bersama wirausaha dan pengemudi
              dalam ekosistem terbuka.
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
                    Gabung Menjadi Mitra Pengemudi
                  </h3>
                  <p className="mb-6 font-body text-text-secondary">
                    Atur waktu kerja Anda sendiri secara bebas. Dapatkan skema
                    bonus yang transparan, proteksi kecelakaan kerja, dan
                    pencairan saldo penghasilan setiap hari tanpa potongan
                    tersembunyi.
                  </p>
                  <ul className="mb-8 space-y-2 font-body text-sm text-text-secondary">
                    <li className="flex items-center gap-2">
                      <MaterialIcon name="check" className="text-[18px] text-primary-container" />
                      <span>Pencairan dana langsung ke rekening bank harian</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <MaterialIcon name="check" className="text-[18px] text-primary-container" />
                      <span>Program santunan &amp; asuransi kecelakaan mitra aktif</span>
                    </li>
                  </ul>
                </div>
                <Link
                  href="#kontak"
                  className="inline-flex items-center gap-2 font-display text-[15px] font-semibold text-primary-container transition-all duration-200 hover:gap-3"
                >
                  <span>Daftar Mitra Driver</span>
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
                    Kembangkan Bisnis Kuliner &amp; Usaha Anda
                  </h3>
                  <p className="mb-6 font-body text-text-secondary">
                    Perluas jangkauan pembeli di seluruh kota melalui Merchant
                    Hub. Akses laporan penjualan real-time, manajemen katalog
                    menu instan, dan kampanye promosi tepat sasaran.
                  </p>
                  <ul className="mb-8 space-y-2 font-body text-sm text-text-secondary">
                    <li className="flex items-center gap-2">
                      <MaterialIcon name="check" className="text-[18px] text-primary-container" />
                      <span>Dashboard analitik performa menu dan jam sibuk</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <MaterialIcon name="check" className="text-[18px] text-primary-container" />
                      <span>Dukungan materi promosi terverifikasi di aplikasi</span>
                    </li>
                  </ul>
                </div>
                <Link
                  href="#kontak"
                  className="inline-flex items-center gap-2 font-display text-[15px] font-semibold text-primary-container transition-all duration-200 hover:gap-3"
                >
                  <span>Daftar Merchant Hub</span>
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