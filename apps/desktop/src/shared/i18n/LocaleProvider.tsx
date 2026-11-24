import { createContext, useContext, useMemo, type ReactNode } from "react";
import { resolveLocalizedContent, type LocalizedContent } from "./messages";

export function detectSystemLocale(): string {
  return navigator.language || "pt-BR";
}

const LocaleContext = createContext<LocalizedContent>(resolveLocalizedContent("pt-BR"));

/**
 * Provides texts for a locale. The outer provider (system locale) covers the connection screen;
 * a nested one, fed by the saved settings, overrides it once settings are loaded.
 */
export function LocaleProvider({ preferredLocale, children }: { preferredLocale?: string; children: ReactNode }) {
  const effectiveLocale = preferredLocale ?? detectSystemLocale();
  const localizedContent = useMemo(() => resolveLocalizedContent(effectiveLocale), [effectiveLocale]);
  return <LocaleContext.Provider value={localizedContent}>{children}</LocaleContext.Provider>;
}

export function useLocale(): LocalizedContent {
  return useContext(LocaleContext);
}
