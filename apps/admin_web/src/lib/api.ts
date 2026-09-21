import axios from "axios";

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080";

export const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 15000,
  headers: {
    "Content-Type": "application/json",
  },
});

api.interceptors.request.use((config) => {
  if (typeof window !== "undefined") {
    const token = window.localStorage.getItem("admin_token");
    const twoFa = window.localStorage.getItem("admin_2fa_token");
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    if (twoFa) {
      config.headers["X-Admin-2FA-Token"] = twoFa;
    }
  }
  return config;
});

export function setAdminSession(
  token: string,
  twoFaToken?: string,
): void {
  if (typeof window === "undefined") return;
  window.localStorage.setItem("admin_token", token);
  window.localStorage.removeItem("admin_2fa_token");
  if (twoFaToken && twoFaToken.trim()) {
    window.localStorage.setItem("admin_2fa_token", twoFaToken.trim());
  }
}

export function clearAdminSession(): void {
  if (typeof window === "undefined") return;
  window.localStorage.removeItem("admin_token");
  window.localStorage.removeItem("admin_2fa_token");
}

export interface AdminLoginData {
  access_token: string;
  refresh_token: string;
  user_id: string;
  user_type: string;
}

export async function loginAdmin(
  email: string,
  password: string,
): Promise<AdminLoginData> {
  const res = await api.post<{ success: boolean; data: AdminLoginData }>(
    "/api/v1/admin/login",
    { email, password },
  );
  return res.data.data;
}
