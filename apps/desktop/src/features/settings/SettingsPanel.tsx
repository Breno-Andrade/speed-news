import { useEffect, useId, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useApiClient } from "../../shared/api/ApiProvider";
import type { InterfaceLanguage, NewsSource, UserSettings } from "../../shared/api/types";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import { describeLanguage } from "../../shared/i18n/messages";
import { getAppVersion, openExternalUrl } from "../../shared/platform/tauriBridge";
import { listSupportedTimeZones } from "../../shared/time/timeZone";
import { buildTimeZoneOptions, describeTimeZoneOption, filterTimeZoneOptions, groupByCountry } from "../../shared/time/timeZoneOptions";
import { BackIcon, ChevronRightIcon } from "../../shared/ui/icons";
import { SkeletonBlock, SkeletonGroup, useDelayedFlag } from "../../shared/ui/Skeleton";
import { GnomeExtensionRow } from "../gnome/GnomeExtensionRow";
import { LaunchAtLoginRow } from "./LaunchAtLoginRow";
import { useSettings } from "./SettingsProvider";

const REPOSITORY_URL = "https://github.com/Breno-Andrade/speed-news";

type SaveSettings = (settingsChanges: Partial<UserSettings>) => Promise<void>;

export function SettingsPanel() {
  const { messages } = useLocale();
  const { updateSettings } = useSettings();
  const [saveFeedback, setSaveFeedback] = useState<"saved" | "failed" | null>(null);
  const [currentPage, setCurrentPage] = useState<"main" | "timeZone">("main");

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

      {currentPage === "main" ? (
        <div key="main" className="settings-page view-enter-back">
          <h2 className="settings-panel__title">{messages.settings}</h2>
          <GeneralGroup onSave={saveAndConfirm} />
          <NewsLanguagesGroup onSave={saveAndConfirm} />
          <ScheduleGroup onOpenTimeZonePicker={() => setCurrentPage("timeZone")} />
          <AboutGroup />
        </div>
      ) : (
        <div key="timeZone" className="settings-page view-enter-forward">
          <TimeZonePicker onSave={saveAndConfirm} onClose={() => setCurrentPage("main")} />
        </div>
      )}
    </div>
  );
}

// ---------- building blocks ----------

function SettingsGroup({ title, footer, children }: { title: string; footer?: string; children: ReactNode }) {
  const headingId = useId();
  return (
    <section className="settings-group" aria-labelledby={headingId}>
      <h3 id={headingId} className="settings-group__title">
        {title}
      </h3>
      <div className="settings-card">{children}</div>
      {footer && <p className="settings-group__footer">{footer}</p>}
    </section>
  );
}

function ToggleRow({ label, description, isOn, isDisabled = false, onToggle }: { label: string; description?: string; isOn: boolean; isDisabled?: boolean; onToggle: (isOn: boolean) => void }) {
  return (
    <label className={`settings-row ${isDisabled ? "settings-row--disabled" : ""}`}>
      <span className="settings-row__text">
        <span className="settings-row__label">{label}</span>
        {description && <span className="settings-row__description">{description}</span>}
      </span>
      <input type="checkbox" role="switch" className="toggle-switch" checked={isOn} disabled={isDisabled} onChange={(changeEvent) => onToggle(changeEvent.target.checked)} />
    </label>
  );
}

// ---------- groups ----------

function GeneralGroup({ onSave }: { onSave: SaveSettings }) {
  const { messages, locale } = useLocale();
  const { userSettings, systemLocale } = useSettings();
  const labelId = useId();
  const languageOptions: { interfaceLanguage: InterfaceLanguage; label: string }[] = [
    { interfaceLanguage: "system", label: messages.automaticShort },
    // Each language is written in itself, so it stays recognizable whatever language is active.
    { interfaceLanguage: "pt-BR", label: "Português" },
    { interfaceLanguage: "en", label: "English" },
  ];

  return (
    <SettingsGroup title={messages.generalTitle}>
      <div className="settings-row settings-row--stacked">
        <span className="settings-row__text">
          <span id={labelId} className="settings-row__label">
            {messages.interfaceLanguageTitle}
          </span>
          {userSettings.interfaceLanguage === "system" && (
            <span className="settings-row__description">{messages.interfaceLanguageAutomatic(describeLanguage(systemLocale, locale))}</span>
          )}
        </span>
        <div className="segmented-choice" role="radiogroup" aria-labelledby={labelId}>
          {languageOptions.map((languageOption) => (
            <label key={languageOption.interfaceLanguage} className="segmented-choice__option">
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
      </div>
      <ToggleRow
        label={messages.closeOnFocusLoss}
        description={messages.closeOnFocusLossHint}
        isOn={userSettings.closeOnFocusLoss}
        onToggle={(isOn) => onSave({ closeOnFocusLoss: isOn })}
      />
      <LaunchAtLoginRow />
      <GnomeExtensionRow />
    </SettingsGroup>
  );
}

function useNewsSources(): NewsSource[] | null {
  const apiClient = useApiClient();
  const [newsSources, setNewsSources] = useState<NewsSource[] | null>(null);
  useEffect(() => {
    apiClient
      .listSources()
      .then(setNewsSources)
      .catch(() => setNewsSources([]));
  }, [apiClient]);
  return newsSources;
}

function NewsLanguagesGroup({ onSave }: { onSave: SaveSettings }) {
  const { messages, locale } = useLocale();
  const { userSettings } = useSettings();
  const newsSources = useNewsSources();
  const showsSkeleton = useDelayedFlag(newsSources === null);

  if (newsSources === null) {
    return (
      <SettingsGroup title={messages.newsTitle}>
        {showsSkeleton ? (
          <SkeletonGroup label={messages.loadingPanel}>
            {[0, 1].map((rowIndex) => (
              <div key={rowIndex} className="settings-row">
                <SkeletonBlock width={140} height={13} />
                <SkeletonBlock width={34} height={20} radius={10} />
              </div>
            ))}
          </SkeletonGroup>
        ) : (
          <div className="settings-pending" />
        )}
      </SettingsGroup>
    );
  }

  const sourceCountByLanguage = new Map<string, number>();
  for (const newsSource of newsSources) {
    sourceCountByLanguage.set(newsSource.languageCode, (sourceCountByLanguage.get(newsSource.languageCode) ?? 0) + 1);
  }
  const availableLanguageCodes = [...sourceCountByLanguage.keys()].sort();
  // An empty saved list means "all languages".
  const selectedLanguageCodes = userSettings.newsLanguageCodes.length === 0 ? availableLanguageCodes : userSettings.newsLanguageCodes;

  function toggleLanguage(languageCode: string, isOn: boolean) {
    const nextSelection = isOn ? [...selectedLanguageCodes, languageCode] : selectedLanguageCodes.filter((selectedCode) => selectedCode !== languageCode);
    const selectsEverything = availableLanguageCodes.every((availableCode) => nextSelection.includes(availableCode));
    // Saving "all" as an empty list lets languages added to the sources later appear automatically.
    return onSave({ newsLanguageCodes: selectsEverything ? [] : nextSelection });
  }

  return (
    <SettingsGroup title={messages.newsTitle} footer={messages.newsLanguagesHint}>
      {availableLanguageCodes.map((languageCode) => {
        const isSelected = selectedLanguageCodes.includes(languageCode);
        return (
          <ToggleRow
            key={languageCode}
            label={describeLanguage(languageCode, locale)}
            description={messages.sourceCount(sourceCountByLanguage.get(languageCode) ?? 0)}
            isOn={isSelected}
            // The last selected language cannot be turned off: no news at all makes no sense.
            isDisabled={isSelected && selectedLanguageCodes.length === 1}
            onToggle={(isOn) => toggleLanguage(languageCode, isOn)}
          />
        );
      })}
    </SettingsGroup>
  );
}

function ScheduleGroup({ onOpenTimeZonePicker }: { onOpenTimeZonePicker: () => void }) {
  const { messages, locale } = useLocale();
  const { userSettings, effectiveTimeZone } = useSettings();
  const currentZone = describeTimeZoneOption(effectiveTimeZone, locale);

  return (
    <SettingsGroup title={messages.scheduleTitle}>
      <button type="button" className="settings-row settings-row--link" onClick={onOpenTimeZonePicker}>
        <span className="settings-row__text">
          <span className="settings-row__label">{messages.timeZoneTitle}</span>
          <span className="settings-row__description">
            {userSettings.timeZonePreference.mode === "system" ? messages.timeZoneFollowsComputer : messages.timeZoneChosenByYou}
          </span>
        </span>
        <span className="settings-row__value">
          {currentZone.cityName} · {currentZone.offsetLabel}
        </span>
        <ChevronRightIcon size={14} />
      </button>
    </SettingsGroup>
  );
}

function AboutGroup() {
  const { messages } = useLocale();
  const newsSources = useNewsSources();
  const [appVersion, setAppVersion] = useState<string | null>(null);
  useEffect(() => {
    getAppVersion()
      .then(setAppVersion)
      .catch(() => setAppVersion(null));
  }, []);

  const dataSources = [
    ...(newsSources ?? []).map((newsSource) => ({ name: newsSource.displayName, detail: messages.newsSourceDetail, url: newsSource.homepageUrl })),
    { name: "Jolpica F1", detail: messages.calendarSourceDetail, url: "https://github.com/jolpica/jolpica-f1" },
    { name: "f1-circuits", detail: messages.circuitSourceDetail, url: "https://github.com/bacinger/f1-circuits" },
    { name: "Open-Meteo", detail: messages.weatherSourceDetail, url: "https://open-meteo.com" },
  ];

  return (
    <SettingsGroup title={messages.aboutTitle} footer={messages.trademarkNotice}>
      <div className="settings-row">
        <span className="settings-row__label">{messages.versionLabel}</span>
        <span className="settings-row__value">{appVersion ?? messages.developmentBuild}</span>
      </div>
      <button type="button" className="settings-row settings-row--link" onClick={() => openExternalUrl(REPOSITORY_URL)}>
        <span className="settings-row__text">
          <span className="settings-row__label">{messages.sourceCodeLabel}</span>
          <span className="settings-row__description">{messages.licenseDescription}</span>
        </span>
        <ChevronRightIcon size={14} />
      </button>
      <details className="settings-disclosure">
        <summary className="settings-row settings-row--link">
          <span className="settings-row__label">{messages.dataSourcesTitle}</span>
          <ChevronRightIcon size={14} />
        </summary>
        <ul className="data-source-list">
          {dataSources.map((dataSource) => (
            <li key={dataSource.name}>
              <button type="button" className="link-button" onClick={() => dataSource.url && openExternalUrl(dataSource.url)}>
                {dataSource.name}
              </button>
              <small>{dataSource.detail}</small>
            </li>
          ))}
        </ul>
      </details>
    </SettingsGroup>
  );
}

// ---------- time zone picker ----------

function TimeZonePicker({ onSave, onClose }: { onSave: SaveSettings; onClose: () => void }) {
  const { messages, locale } = useLocale();
  const { userSettings, systemTimeZone } = useSettings();
  const [searchQuery, setSearchQuery] = useState("");
  const scrollTargetRowRef = useRef<HTMLButtonElement>(null);

  const timeZonePreference = userSettings.timeZonePreference;
  const selectedManualZone = timeZonePreference.mode === "manual" ? timeZonePreference.timeZoneId : null;
  // Where the list opens: the chosen zone, or the computer's one in automatic mode.
  const scrollTargetZone = selectedManualZone ?? systemTimeZone;
  // ~400 zones described once (≈75 ms), not on every keystroke.
  const allTimeZoneOptions = useMemo(() => buildTimeZoneOptions(listSupportedTimeZones(), locale), [locale]);
  const systemZone = describeTimeZoneOption(systemTimeZone, locale);
  // Grouped by country, alphabetically, with the computer's country on top.
  const countryGroups = useMemo(
    () => groupByCountry(filterTimeZoneOptions(allTimeZoneOptions, searchQuery), locale, systemZone.countryCode),
    [allTimeZoneOptions, searchQuery, locale, systemZone.countryCode],
  );

  // Open the list at the current zone instead of at "GMT−11" (Pacific islands).
  useLayoutEffect(() => {
    scrollTargetRowRef.current?.scrollIntoView?.({ block: "center" });
  }, []);

  function choose(nextPreference: UserSettings["timeZonePreference"]) {
    onSave({ timeZonePreference: nextPreference });
    onClose();
  }

  return (
    <div className="time-zone-picker">
      <div className="time-zone-picker__header">
        <button type="button" className="icon-button icon-button--labeled" onClick={onClose}>
          <BackIcon size={14} /> {messages.settings}
        </button>
        <h2 className="settings-panel__title">{messages.timeZoneTitle}</h2>
      </div>

      <input
        className="time-zone-picker__search"
        type="search"
        placeholder={messages.timeZoneSearchPlaceholder}
        aria-label={messages.timeZoneSearchPlaceholder}
        value={searchQuery}
        onChange={(changeEvent) => setSearchQuery(changeEvent.target.value)}
        autoFocus
      />

      <button
        type="button"
        className="time-zone-option time-zone-option--automatic"
        aria-pressed={timeZonePreference.mode === "system"}
        onClick={() => choose({ mode: "system" })}
      >
        <span className="time-zone-option__text">
          <span className="time-zone-option__city">{messages.useComputerTimeZone}</span>
          <span className="time-zone-option__detail">
            {systemZone.cityName} · {systemZone.genericName}
          </span>
        </span>
        <span className="time-zone-option__offset">{systemZone.offsetLabel}</span>
      </button>

      {countryGroups.length === 0 && <p className="empty-state">{messages.noTimeZoneFound}</p>}
      <div className="time-zone-picker__list" role="list">
        {countryGroups.map((countryGroup) => (
          <div key={countryGroup.countryName} role="listitem">
            <h4 className="time-zone-picker__country-heading">{countryGroup.countryName}</h4>
            {countryGroup.timeZoneOptions.map((timeZoneOption) => {
              const isSelected = timeZoneOption.timeZoneId === selectedManualZone;
              return (
                <button
                  key={timeZoneOption.timeZoneId}
                  ref={timeZoneOption.timeZoneId === scrollTargetZone ? scrollTargetRowRef : undefined}
                  type="button"
                  className="time-zone-option"
                  aria-pressed={isSelected}
                  onClick={() => choose({ mode: "manual", timeZoneId: timeZoneOption.timeZoneId })}
                >
                  <span className="time-zone-option__text">
                    <span className="time-zone-option__city">{timeZoneOption.cityName}</span>
                    <span className="time-zone-option__detail">{timeZoneOption.genericName}</span>
                  </span>
                  <span className="time-zone-option__offset">{timeZoneOption.offsetLabel}</span>
                  {isSelected && <span className="time-zone-option__check">✓</span>}
                </button>
              );
            })}
          </div>
        ))}
      </div>
    </div>
  );
}
