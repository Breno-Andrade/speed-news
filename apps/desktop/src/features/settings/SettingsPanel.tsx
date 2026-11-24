import { useEffect, useId, useMemo, useState } from "react";
import { useApiClient } from "../../shared/api/ApiProvider";
import type { InterfaceLanguage, UserSettings } from "../../shared/api/types";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import { describeLanguage } from "../../shared/i18n/messages";
import { describeTimeZone, listSupportedTimeZones } from "../../shared/time/timeZone";
import { SkeletonBlock, SkeletonGroup, useDelayedFlag } from "../../shared/ui/Skeleton";
import { useSettings } from "./SettingsProvider";

const maximumVisibleTimeZones = 80;

function normalizeForSearch(searchText: string): string {
  return searchText
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/_/g, " ")
    .toLowerCase();
}

export function SettingsPanel() {
  const { messages } = useLocale();
  const { updateSettings } = useSettings();
  const [saveFeedback, setSaveFeedback] = useState<"saved" | "failed" | null>(null);

  // Never rejects: the provider already reverted the optimistic change; here the user is told.
  async function saveAndConfirm(settingsChanges: Partial<UserSettings>) {
    try {
      await updateSettings(settingsChanges);
      setSaveFeedback("saved");
    } catch {
      setSaveFeedback("failed");
    }
    window.setTimeout(() => setSaveFeedback(null), 2000);
  }

  return (
    <div className="settings-panel">
      {saveFeedback === "saved" && (
        <p className="settings-panel__saved fade-in" role="status">
          ✓ {messages.saved}
        </p>
      )}
      {saveFeedback === "failed" && (
        <p className="settings-panel__saved settings-panel__saved--failed fade-in" role="alert">
          {messages.settingsSaveFailed}
        </p>
      )}
      <InterfaceLanguageSection onSave={saveAndConfirm} />
      <NewsLanguagesSection onSave={saveAndConfirm} />
      <TimeZoneSection onSave={saveAndConfirm} />
      <WindowBehaviorSection onSave={saveAndConfirm} />
    </div>
  );
}

type SectionProps = { onSave: (settingsChanges: Partial<UserSettings>) => Promise<void> };

function WindowBehaviorSection({ onSave }: SectionProps) {
  const { messages } = useLocale();
  const { userSettings } = useSettings();
  return (
    <section className="settings-section">
      <h2 className="settings-section__title">{messages.windowBehaviorTitle}</h2>
      <label className="settings-panel__option">
        <input
          type="checkbox"
          checked={userSettings.closeOnFocusLoss}
          onChange={(changeEvent) => onSave({ closeOnFocusLoss: changeEvent.target.checked })}
        />
        <span>{messages.closeOnFocusLoss}</span>
      </label>
      <p className="settings-section__hint">{messages.closeOnFocusLossHint}</p>
    </section>
  );
}

function InterfaceLanguageSection({ onSave }: SectionProps) {
  const headingId = useId();
  const { messages, locale } = useLocale();
  const { userSettings, systemLocale } = useSettings();
  const languageOptions: { interfaceLanguage: InterfaceLanguage; label: string }[] = [
    { interfaceLanguage: "system", label: messages.interfaceLanguageAutomatic(describeLanguage(systemLocale, locale)) },
    // Each language is written in itself, so it stays recognizable whatever language is active.
    { interfaceLanguage: "pt-BR", label: "Português" },
    { interfaceLanguage: "en", label: "English" },
  ];

  return (
    <section className="settings-section">
      <h2 id={headingId} className="settings-section__title">
        {messages.interfaceLanguageTitle}
      </h2>
      {/* Named group: both this and the time zone have an "Automatic" option. */}
      <div role="radiogroup" aria-labelledby={headingId}>
        {languageOptions.map((languageOption) => (
          <label key={languageOption.interfaceLanguage} className="settings-panel__option">
            <input
              type="radio"
              name="interface-language"
              checked={userSettings.interfaceLanguage === languageOption.interfaceLanguage}
              onChange={() => onSave({ interfaceLanguage: languageOption.interfaceLanguage })}
            />
            <span>{languageOption.label}</span>
          </label>
        ))}
      </div>
    </section>
  );
}

function NewsLanguagesSection({ onSave }: SectionProps) {
  const { messages, locale } = useLocale();
  const { userSettings } = useSettings();
  const apiClient = useApiClient();
  const [availableLanguageCodes, setAvailableLanguageCodes] = useState<string[] | null>(null);
  const showsSkeleton = useDelayedFlag(availableLanguageCodes === null);

  useEffect(() => {
    apiClient
      .listSources()
      .then((newsSources) => setAvailableLanguageCodes([...new Set(newsSources.map((newsSource) => newsSource.languageCode))].sort()))
      .catch(() => setAvailableLanguageCodes([]));
  }, [apiClient]);

  if (availableLanguageCodes === null) {
    return (
      <section className="settings-section">
        <h2 className="settings-section__title">{messages.newsLanguagesTitle}</h2>
        {showsSkeleton ? (
          <SkeletonGroup label={messages.loadingPanel}>
            {[0, 1].map((optionIndex) => (
              <div key={optionIndex} className="settings-panel__option">
                <SkeletonBlock width={14} height={14} radius={3} />
                <SkeletonBlock width={90} height={13} />
              </div>
            ))}
          </SkeletonGroup>
        ) : (
          <div className="settings-pending" />
        )}
      </section>
    );
  }

  // Narrowed once here: TypeScript does not carry the null check into nested functions.
  const loadedLanguageCodes = availableLanguageCodes;
  // An empty saved list means "all languages".
  const selectedLanguageCodes = userSettings.newsLanguageCodes.length === 0 ? loadedLanguageCodes : userSettings.newsLanguageCodes;

  function toggleLanguage(languageCode: string) {
    const nextSelection = selectedLanguageCodes.includes(languageCode)
      ? selectedLanguageCodes.filter((selectedCode) => selectedCode !== languageCode)
      : [...selectedLanguageCodes, languageCode];
    const selectsEverything = loadedLanguageCodes.every((availableCode) => nextSelection.includes(availableCode));
    // Saving "all" as an empty list lets languages added to the sources later appear automatically.
    return onSave({ newsLanguageCodes: selectsEverything ? [] : nextSelection });
  }

  return (
    <section className="settings-section">
      <h2 className="settings-section__title">{messages.newsLanguagesTitle}</h2>
      {loadedLanguageCodes.map((languageCode) => {
        const isSelected = selectedLanguageCodes.includes(languageCode);
        const isLastSelected = isSelected && selectedLanguageCodes.length === 1;
        return (
          <label key={languageCode} className="settings-panel__option">
            <input type="checkbox" checked={isSelected} disabled={isLastSelected} onChange={() => toggleLanguage(languageCode)} />
            <span>{describeLanguage(languageCode, locale)}</span>
          </label>
        );
      })}
      <p className="settings-section__hint">{messages.newsLanguagesHint}</p>
    </section>
  );
}

function TimeZoneSection({ onSave }: SectionProps) {
  const headingId = useId();
  const { messages, locale } = useLocale();
  const { userSettings, systemTimeZone } = useSettings();
  const [searchText, setSearchText] = useState("");

  const timeZonePreference = userSettings.timeZonePreference;
  const selectedManualTimeZone = timeZonePreference.mode === "manual" ? timeZonePreference.timeZoneId : null;

  const matchingTimeZones = useMemo(() => {
    const normalizedQuery = normalizeForSearch(searchText.trim());
    return listSupportedTimeZones()
      .filter((timeZoneId) => normalizeForSearch(timeZoneId).includes(normalizedQuery))
      .slice(0, maximumVisibleTimeZones);
  }, [searchText]);

  return (
    <section className="settings-section">
      <h2 id={headingId} className="settings-section__title">
        {messages.timeZoneTitle}
      </h2>

      <div role="radiogroup" aria-labelledby={headingId}>
        <label className="settings-panel__option">
          <input type="radio" name="time-zone-mode" checked={timeZonePreference.mode === "system"} onChange={() => onSave({ timeZonePreference: { mode: "system" } })} />
          <span>
            {messages.timeZoneAutomatic(systemTimeZone.replace(/_/g, " "))}
            <small>{describeTimeZone(systemTimeZone, locale)}</small>
          </span>
        </label>

        <label className="settings-panel__option">
          <input
            type="radio"
            name="time-zone-mode"
            checked={timeZonePreference.mode === "manual"}
            onChange={() => onSave({ timeZonePreference: { mode: "manual", timeZoneId: selectedManualTimeZone ?? systemTimeZone } })}
          />
          <span>{messages.timeZoneManual}</span>
        </label>
      </div>

      {timeZonePreference.mode === "manual" && (
        <div className="time-zone-picker">
          <input
            className="time-zone-picker__search"
            type="search"
            placeholder={messages.timeZoneSearchPlaceholder}
            value={searchText}
            onChange={(changeEvent) => setSearchText(changeEvent.target.value)}
          />
          <ul className="time-zone-picker__list" role="listbox">
            {matchingTimeZones.map((timeZoneId) => (
              <li key={timeZoneId}>
                <button
                  type="button"
                  role="option"
                  aria-selected={timeZoneId === selectedManualTimeZone}
                  className="time-zone-picker__item"
                  onClick={() => onSave({ timeZonePreference: { mode: "manual", timeZoneId } })}
                >
                  <span>{timeZoneId.replace(/_/g, " ")}</span>
                  <small>{describeTimeZone(timeZoneId, locale)}</small>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}
