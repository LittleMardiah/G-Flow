import { Sidebar } from "@/components/sidebar";
import { Providers } from "@/app/providers";
import { LedgerClient } from "@/components/ledger/ledger-client";

export default function LedgerPage() {
  return (
    <Providers>
      <div className="flex min-h-screen">
        <Sidebar />
        <main className="flex-1 p-6">
          <h1 className="mb-6 text-2xl font-bold">Ledger Audit Trail</h1>
          <LedgerClient />
        </main>
      </div>
    </Providers>
  );
}
