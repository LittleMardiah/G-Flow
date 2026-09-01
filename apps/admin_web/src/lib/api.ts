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
  if (twoFaToken) {
    window.localStorage.setItem("admin_2fa_token", twoFaToken);
  }
}

export function clearAdminSession(): void {
  if (typeof window === "undefined") return;
  window.localStorage.removeItem("admin_token");
  window.localStorage.removeItem("admin_2fa_token");
}
