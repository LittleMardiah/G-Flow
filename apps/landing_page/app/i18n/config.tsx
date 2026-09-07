"use client";

/** Custom i18n (Context) — bukan next-i18next, tanpa dependency ekstra. */
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react";
import type { ReactNode } from "react";
import id from "@/locales/id.json";
import en from "@/locales/en.json";

export type Lang = "id" | "en";

export const dictionaries = { id, en } as const;
export type Dictionary = (typeof dictionaries)["id"];

type ContextValue = {
  lang: Lang;
  setLang: (lang: Lang) => void;
  t: (key: string, vars?: Record<string, string | number>) => string;
};

const LangContext = createContext<ContextValue | null>(null);

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>("id");

  useEffect(() => {
    document.documentElement.lang = lang;
  }, [lang]);

  const setLang = useCallback((next: Lang) => setLangState(next), []);

  const t = useCallback(
    (key: string, vars?: Record<string, string | number>) => {
      const dict = dictionaries[lang] as unknown as Record<string, unknown>;
      const value = key
        .split(".")
        .reduce<unknown>(
          (acc, k) => (acc as Record<string, unknown> | undefined)?.[k],
          dict,
        );

      if (typeof value !== "string") return key;
      if (!vars) return value;
      return Object.entries(vars).reduce((str, [k, v]) => {
        return str.replaceAll(`{${k}}`, String(v));
      }, value);
    },
    [lang],
  );

  return (
    <LangContext.Provider value={{ lang, setLang, t }}>
      {children}
    </LangContext.Provider>
  );
}

export function useLang(): ContextValue {
  const ctx = useContext(LangContext);
  if (!ctx) {
    throw new Error("useLang harus dipakai di dalam <LanguageProvider>");
  }
  return ctx;
}