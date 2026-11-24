import type { ReactNode } from "react";
import type { ApiClient } from "../../shared/api/apiClient";
import type { CircuitProfile, DriverResult, FastestLap, RaceResultSummary, WeekendForecast } from "../../shared/api/types";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import { openExternalUrl } from "../../shared/platform/tauriBridge";
import { formatTimeOfDay } from "../../shared/time/timeZone";
import { useSettings } from "../settings/SettingsProvider";
import { useDelayedFlag } from "../../shared/ui/Skeleton";
import { describeWeatherCode, formatMeters, formatRaceDistance, formatRaceTime } from "./raceFormatting";
import { CircuitSkeleton, ForecastSkeleton, PodiumSkeleton } from "./RaceCardSkeletons";
import { useRaceInsight, type InsightState } from "./useRaceInsight";

// Module-level loaders keep a stable identity for useRaceInsight.
const loadLastRace = (apiClient: ApiClient) => apiClient.fetchLastRace();
const loadCircuitProfile = (apiClient: ApiClient) => apiClient.fetchCircuitProfile();
const loadWeekendForecast = (apiClient: ApiClient) => apiClient.fetchWeekendForecast();

type InsightBoundaryProps<Payload> = {
  insightState: InsightState<Payload>;
  skeleton: ReactNode;
  emptyMessage?: string;
  children: (payload: NonNullable<Payload>) => ReactNode;
};

/** Skeleton while loading (only if it takes long enough to notice), then the content fades in. */
function InsightBoundary<Payload>({ insightState, skeleton, emptyMessage, children }: InsightBoundaryProps<Payload>) {
  const { messages } = useLocale();
  const isLoading = insightState.status === "idle" || insightState.status === "loading";
  const showsSkeleton = useDelayedFlag(isLoading);

  if (isLoading) return showsSkeleton ? <>{skeleton}</> : <div className="insight-pending" />;
  if (insightState.status === "failed") return <p className="insight-message fade-in">{messages.panelUnavailable}</p>;
  if (insightState.payload === null || insightState.payload === undefined) {
    return <p className="insight-message fade-in">{emptyMessage ?? messages.panelUnavailable}</p>;
  }
  return <div className="fade-in">{children(insightState.payload as NonNullable<Payload>)}</div>;
}

function PodiumList({ podium }: { podium: DriverResult[] }) {
  const { messages, locale } = useLocale();
  const podiumMedals = ["🥇", "🥈", "🥉"];
  return (
    <ol className="podium-list">
      {podium.map((driverResult) => (
        <li key={driverResult.driverCode} className="podium-row">
          <span className="podium-row__medal" aria-label={`${driverResult.position}º`}>
            {podiumMedals[driverResult.position - 1] ?? driverResult.position}
          </span>
          <span className="podium-row__driver">
            {driverResult.driverName}
            <small>
              {driverResult.teamName}
              {driverResult.gridPosition > 3 && ` · ${messages.startedFrom(driverResult.gridPosition)}`}
            </small>
          </span>
          <span className="podium-row__time">{formatRaceTime(driverResult, locale, messages)}</span>
        </li>
      ))}
    </ol>
  );
}

function RaceHighlights({ poleSitter, fastestLap }: { poleSitter?: DriverResult; fastestLap?: FastestLap }) {
  const { messages } = useLocale();
  return (
    <dl className="race-highlights">
      {poleSitter && (
        <div>
          <dt>{messages.polePosition}</dt>
          <dd>{poleSitter.driverName}</dd>
        </div>
      )}
      {fastestLap && (
        <div>
          <dt>⏱ {messages.fastestLap}</dt>
          <dd>
            {fastestLap.driverName} · {fastestLap.lapTime} <small>({messages.onLap(fastestLap.lapNumber)})</small>
          </dd>
        </div>
      )}
    </dl>
  );
}

export function LastRacePanel() {
  const { messages, locale, translateRaceName } = useLocale();
  const insightState = useRaceInsight(loadLastRace, true);
  return (
    <InsightBoundary insightState={insightState} skeleton={<PodiumSkeleton label={messages.loadingPanel} />}>
      {(lastRace: RaceResultSummary) => (
        <div className="insight-panel">
          <p className="insight-panel__title">
            {translateRaceName(lastRace.eventName)}
            <small>{new Intl.DateTimeFormat(locale, { day: "numeric", month: "long", timeZone: "UTC" }).format(new Date(`${lastRace.raceDate}T12:00:00Z`))}</small>
          </p>
          <PodiumList podium={lastRace.podium} />
          <RaceHighlights poleSitter={lastRace.poleSitter} fastestLap={lastRace.fastestLap} />
        </div>
      )}
    </InsightBoundary>
  );
}

export function CircuitPanel() {
  const { messages, locale } = useLocale();
  const insightState = useRaceInsight(loadCircuitProfile, true);
  return (
    <InsightBoundary insightState={insightState} skeleton={<CircuitSkeleton label={messages.loadingPanel} />}>
      {(circuitProfile: CircuitProfile) => {
        const circuitFacts = circuitProfile.facts;
        const raceLaps = circuitProfile.previousEdition?.laps;
        return (
          <div className="insight-panel">
            {circuitFacts && (
              <dl className="circuit-facts">
                <div>
                  <dt>{messages.trackLength}</dt>
                  <dd>{formatMeters(circuitFacts.lengthMeters, locale)}</dd>
                </div>
                {raceLaps ? (
                  <>
                    <div>
                      <dt>{messages.raceLaps}</dt>
                      <dd>{raceLaps}</dd>
                    </div>
                    <div>
                      <dt>{messages.raceDistance}</dt>
                      <dd>{formatRaceDistance(circuitFacts.lengthMeters, raceLaps, locale)}</dd>
                    </div>
                  </>
                ) : null}
                <div>
                  <dt>{messages.firstGrandPrix}</dt>
                  <dd>{circuitFacts.firstGrandPrixYear}</dd>
                </div>
              </dl>
            )}

            {circuitProfile.previousEdition && (
              <section className="insight-section">
                <h4>{messages.previousEditionTitle(circuitProfile.previousEdition.season)}</h4>
                <PodiumList podium={circuitProfile.previousEdition.podium.slice(0, 1)} />
                <RaceHighlights poleSitter={circuitProfile.previousEdition.poleSitter} fastestLap={circuitProfile.previousEdition.fastestLap} />
              </section>
            )}

            {circuitProfile.recentWinners.length > 0 && (
              <section className="insight-section">
                <h4>{messages.recentWinners}</h4>
                <ul className="winners-list">
                  {circuitProfile.recentWinners.map((circuitWinner) => (
                    <li key={circuitWinner.season}>
                      <span className="winners-list__season">{circuitWinner.season}</span>
                      <span>{circuitWinner.driverName}</span>
                      <small>{circuitWinner.teamName}</small>
                    </li>
                  ))}
                </ul>
              </section>
            )}
          </div>
        );
      }}
    </InsightBoundary>
  );
}

export function ForecastPanel() {
  const { messages, locale } = useLocale();
  const { effectiveTimeZone } = useSettings();
  const insightState = useRaceInsight(loadWeekendForecast, true);
  return (
    <InsightBoundary insightState={insightState} skeleton={<ForecastSkeleton label={messages.loadingPanel} />} emptyMessage={messages.forecastNotYet}>
      {(weekendForecast: WeekendForecast) => (
        <div className="insight-panel">
          <ul className="forecast-list">
            {weekendForecast.sessions.map((sessionForecast) => {
              const weatherDescription = describeWeatherCode(sessionForecast.weatherCode);
              const weatherLabel = messages.weatherLabels[weatherDescription.weatherCondition];
              return (
                <li key={sessionForecast.sessionKind} className="forecast-row">
                  <span className="forecast-row__session">
                    {messages.sessionLabels[sessionForecast.sessionKind]}
                    <small>
                      {new Intl.DateTimeFormat(locale, { weekday: "short", timeZone: effectiveTimeZone }).format(new Date(sessionForecast.startsAt))}{" "}
                      {formatTimeOfDay(sessionForecast.startsAt, effectiveTimeZone, locale)}
                    </small>
                  </span>
                  <span className="forecast-row__weather" title={weatherLabel} aria-label={weatherLabel}>
                    {weatherDescription.weatherEmoji}
                  </span>
                  <span className="forecast-row__temperature">{Math.round(sessionForecast.temperatureCelsius)}°C</span>
                  <span className="forecast-row__details">
                    💧 {sessionForecast.precipitationProbability}% {messages.rainChance}
                    <small>
                      💨 {Math.round(sessionForecast.windSpeedKmh)} km/h {messages.wind}
                    </small>
                  </span>
                </li>
              );
            })}
          </ul>
          <button type="button" className="link-button insight-attribution" onClick={() => openExternalUrl(weekendForecast.attributionUrl)}>
            {weekendForecast.attributionText}
          </button>
        </div>
      )}
    </InsightBoundary>
  );
}
