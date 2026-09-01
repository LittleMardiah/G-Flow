import { Sidebar } from "@/components/sidebar";
import { Providers } from "@/app/providers";
import { UsersClient } from "@/components/users/users-client";

export default function UsersPage() {
  return (
    <Providers>
      <div className="flex min-h-screen">
        <Sidebar />
        <main className="flex-1 p-6">
          <h1 className="mb-6 text-2xl font-bold">User Management</h1>
          <UsersClient />
        </main>
      </div>
    </Providers>
  );
}
