import { Sidebar } from "@/components/sidebar";
import { Providers } from "@/app/providers";
import { DashboardClient } from "@/components/dashboard/dashboard-client";

export default function DashboardPage() {
  return (
    <Providers>
      <div className="flex min-h-screen">
        <Sidebar />
        <main className="flex-1 p-6">
          <h1 className="mb-6 text-2xl font-bold">Dashboard</h1>
          <DashboardClient />
        </main>
      </div>
    </Providers>
  );
}
