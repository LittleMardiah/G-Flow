"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import axios from "axios";
import { loginAdmin, setAdminSession } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [twoFa, setTwoFa] = useState("");
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const canSubmit = email.trim().length > 0 && password.length > 0 && !isSubmitting;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    setErrorMsg(null);
    setIsSubmitting(true);
    try {
      const data = await loginAdmin(email.trim(), password);
      setAdminSession(data.access_token, twoFa.trim() || undefined);
      router.push("/");
    } catch (err) {
      if (axios.isAxiosError(err)) {
        const status = err.response?.status;
        const message = err.response?.data?.error?.message;
        if (status === 401 || status === 403) {
          setErrorMsg(message ?? "Email, password, atau akun tidak valid.");
        } else {
          setErrorMsg("Terjadi kesalahan. Coba lagi beberapa saat.");
        }
      } else {
        setErrorMsg("Terjadi kesalahan. Coba lagi beberapa saat.");
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-100 p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>G-Flow Admin Login</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="grid gap-4">
            <div>
              <label className="mb-1 block text-sm text-gray-600">Email</label>
              <Input
                type="email"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="admin@g-flow.local"
              />
            </div>
            <div>
              <label className="mb-1 block text-sm text-gray-600">
                Password
              </label>
              <Input
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
              />
            </div>
            <div>
              <label className="mb-1 block text-sm text-gray-600">
                2FA Token <span className="text-gray-400">(opsional)</span>
              </label>
              <Input
                type="password"
                value={twoFa}
                onChange={(e) => setTwoFa(e.target.value)}
                placeholder="X-Admin-2FA-Token"
              />
            </div>
            {errorMsg && (
              <p className="text-sm text-red-500">{errorMsg}</p>
            )}
            <Button type="submit" disabled={!canSubmit} className="w-full">
              {isSubmitting ? "Memproses…" : "Login"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}