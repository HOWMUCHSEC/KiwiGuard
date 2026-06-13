import { createContext, ReactNode, useContext, useEffect, useMemo, useState } from "react";

import { enMessages } from "./en";
import type { Locale, TranslationMessages } from "./types";
import { zhCNMessages } from "./zh-CN";

export type { Locale } from "./types";

type TranslationValues = Record<string, string | number>;

type I18nContextValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: string, values?: TranslationValues) => string;
};

const storageKey = "kiwiguard.console.locale";

const translations: Record<Locale, TranslationMessages> = {
  en: enMessages,
  "zh-CN": zhCNMessages
};

const I18nContext = createContext<I18nContextValue | undefined>(undefined);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(() => {
    const stored = localStorage.getItem(storageKey);
    return stored === "zh-CN" || stored === "en" ? stored : "en";
  });

  useEffect(() => {
    localStorage.setItem(storageKey, locale);
    document.documentElement.lang = locale;
  }, [locale]);

  const value = useMemo<I18nContextValue>(
    () => ({
      locale,
      setLocale: setLocaleState,
      t: (key, values) => interpolate(translations[locale][key] ?? translations.en[key] ?? key, values)
    }),
    [locale]
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const context = useContext(I18nContext);
  if (!context) throw new Error("useI18n must be used within I18nProvider");
  return context;
}

function interpolate(template: string, values?: TranslationValues) {
  if (!values) return template;
  return template.replace(/\{(\w+)\}/g, (_, key: string) => String(values[key] ?? ""));
}
