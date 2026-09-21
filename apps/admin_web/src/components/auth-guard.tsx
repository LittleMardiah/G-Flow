"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [checked, setChecked] = useState(false);

  const isLoginPage = pathname === "/login";

  useEffect(() => {
    if (isLoginPage) {
      setChecked(true);
      return;
    }
    const token = window.localStorage.getItem("admin_token");
    if (!token) {
      router.replace("/login");
      return;
    }
    setChecked(true);
  }, [isLoginPage, pathname, router]);

  if (isLoginPage) return <>{children}</>;
  if (!checked) return null;

  return <>{children}</>;
}