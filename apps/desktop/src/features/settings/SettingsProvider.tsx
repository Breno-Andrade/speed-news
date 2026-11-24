import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { useApiClient } from "../../shared/api/ApiProvider";
import type { UserSettings } from "../../shared/api/types";
import { detectSystemLocale, LocaleProvider } from "../../shared/i18n/LocaleProvider";
import { resolvePreferredLocale } from "../../shared/i18n/messages";
import { setCloseOnFocusLoss } from "../../shared/platform/tauriBridge";
import { detectSystemTimeZone, resolveEffectiveTimeZone } from "../../shared/time/timeZone";

const defaultUserSettings: UserSettings = {
  timeZonePreference: { mode: "system" },
  interfaceLanguage: "system",
  newsLanguageCodes: [],
  closeOnFocusLoss: true,
};

type SettingsContextValue = {
  userSettings: UserSettings;
  systemTimeZone: string;
  systemLocale: string;
  effectiveTimeZone: string;
  updateSettings: (settingsChanges: Partial<UserSettings>) => Promise<void>;
};

const SettingsContext = createContext<SettingsContextValue | null>(null);

export function SettingsProvider({ children }: { children: ReactNode }) {
  const apiClient = useApiClient();
  const [userSettings, setUserSettings] = useState<UserSettings>(defaultUserSettings);
  const [systemTimeZone, setSystemTimeZone] = useState(detectSystemTimeZone);
  const [systemLocale, setSystemLocale] = useState(detectSystemLocale);

  useEffect(() => {
    apiClient
      .fetchSettings()
      .then((loadedSettings) => setUserSettings({ ...defaultUserSettings, ...loadedSettings }))
      .catch(() => undefined);
  }, [apiClient]);

  // The native shell decides whether to hide on focus loss, so it must know the saved choice.
  useEffect(() => {
    setCloseOnFocusLoss(userSettings.closeOnFocusLoss).catch(() => undefined);
  }, [userSettings.closeOnFocusLoss]);

  // "Automatic" options must follow the computer while the app keeps running:
  // re-detect time zone and language every time the popover gains focus.
  useEffect(() => {
    const refreshSystemPreferences = () => {
      setSystemTimeZone(detectSystemTimeZone());
      setSystemLocale(detectSystemLocale());
    };
    window.addEventListener("focus", refreshSystemPreferences);
    return () => window.removeEventListener("focus", refreshSystemPreferences);
  }, []);

  // Optimistic: the control reflects the click immediately; the previous value comes back if saving fails.
  const updateSettings = useCallback(
    async (settingsChanges: Partial<UserSettings>) => {
      const previousSettings = userSettings;
      const nextSettings = { ...userSettings, ...settingsChanges };
      setUserSettings(nextSettings);
      try {
        setUserSettings(await apiClient.saveSettings(nextSettings));
      } catch (saveError) {
        setUserSettings(previousSettings);
        throw saveError;
      }
    },
    [apiClient, userSettings],
  );

  const contextValue = useMemo(
    () => ({
      userSettings,
      systemTimeZone,
      systemLocale,
      effectiveTimeZone: resolveEffectiveTimeZone(userSettings.timeZonePreference, systemTimeZone),
      updateSettings,
    }),
    [userSettings, systemTimeZone, systemLocale, updateSettings],
  );

  return (
    <SettingsContext.Provider value={contextValue}>
      <LocaleProvider preferredLocale={resolvePreferredLocale(userSettings.interfaceLanguage, systemLocale)}>{children}</LocaleProvider>
    </SettingsContext.Provider>
  );
}

export function useSettings(): SettingsContextValue {
  const settingsContext = useContext(SettingsContext);
  if (!settingsContext) throw new Error("useSettings must be used inside <SettingsProvider>");
  return settingsContext;
}
