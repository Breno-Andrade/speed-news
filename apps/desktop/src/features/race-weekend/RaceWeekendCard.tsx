import { useState } from "react";
import type { RaceEvent, RaceSession } from "../../shared/api/types";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import { describeTimeZone, formatCountdown, formatTimeOfDay, groupSessionsByDay } from "../../shared/time/timeZone";
import { useSettings } from "../settings/SettingsProvider";
import { CircuitMap } from "./CircuitMap";
import { AnimatedHeight } from "../../shared/ui/AnimatedHeight";
import { useDelayedFlag } from "../../shared/ui/Skeleton";
import { CircuitPanel, ForecastPanel, LastRacePanel } from "./InsightPanels";
import { RaceCardSkeleton } from "./RaceCardSkeletons";
import { classifySessions, estimatedEndsAt, type SessionStatus } from "./sessionStatus";
import { useCurrentInstant, useRaceWeekend } from "./useRaceWeekend";

type RaceCardTab = "schedule" | "lastRace" | "circuit" | "weather";

export function RaceWeekendCard({ onChangeTimeZone }: { onChangeTimeZone: () => void }) {
  const { messages, locale, translateRaceName } = useLocale();
  const { effectiveTimeZone } = useSettings();
  const raceWeekendState = useRaceWeekend();
  const currentInstant = useCurrentInstant();
  const [selectedTab, setSelectedTab] = useState<RaceCardTab>("schedule");
  const showsSkeleton = useDelayedFlag(raceWeekendState.status === "loading");

  if (raceWeekendState.status === "loading") {
    return showsSkeleton ? <RaceCardSkeleton label={messages.loadingPanel} /> : <div className="race-card-pending" />;
  }
  if (raceWeekendState.status === "failed" || !raceWeekendState.raceWeekend) {
    return <section className="race-card race-card--empty fade-in">{messages.noUpcomingRace}</section>;
  }

  const { raceEvent, circuitLayout, isRaceWeekendNow } = raceWeekendState.raceWeekend;
  const tabLabels: Record<RaceCardTab, string> = {
    schedule: messages.scheduleTab,
    lastRace: messages.lastRaceTab,
    circuit: messages.circuitTab,
    weather: messages.weatherTab,
  };
  const showsLocalTimes = selectedTab === "schedule" || selectedTab === "weather";

  return (
    <section className={`race-card fade-in ${isRaceWeekendNow ? "race-card--weekend" : ""}`}>
      <header className="race-card__header">
        <div>
          <p className="race-card__eyebrow">
            {isRaceWeekendNow ? messages.raceWeekendNow : messages.nextRace} · {messages.round(raceEvent.round)}
          </p>
          <h2 className="race-card__title">{translateRaceName(raceEvent.eventName)}</h2>
          <p className="race-card__circuit">
            {raceEvent.circuitName} · {raceEvent.locality}
          </p>
        </div>
        {circuitLayout && <CircuitMap circuitLayout={circuitLayout} circuitName={raceEvent.circuitName} />}
      </header>

      <div className="race-card__tabs" role="tablist">
        {(Object.keys(tabLabels) as RaceCardTab[]).map((raceCardTab) => (
          <button
            key={raceCardTab}
            type="button"
            role="tab"
            aria-selected={selectedTab === raceCardTab}
            className="race-card__tab"
            onClick={() => setSelectedTab(raceCardTab)}
          >
            {tabLabels[raceCardTab]}
          </button>
        ))}
      </div>

      {/* Tabs have different heights (and the footer comes and goes): the card's bottom edge
          glides to the new size instead of jumping. */}
      <AnimatedHeight>
        {/* key restarts the fade on every tab change */}
        <div role="tabpanel" key={selectedTab} className="fade-in">
          {selectedTab === "schedule" && <SessionSchedule raceEvent={raceEvent} currentInstant={currentInstant} />}
          {selectedTab === "lastRace" && <LastRacePanel />}
          {selectedTab === "circuit" && <CircuitPanel />}
          {selectedTab === "weather" && <ForecastPanel />}
        </div>

        {showsLocalTimes && (
          <footer className="race-card__footer">
            {messages.timesShownIn(describeTimeZone(effectiveTimeZone, locale, currentInstant))} ·{" "}
            <button type="button" className="link-button" onClick={onChangeTimeZone}>
              {messages.change}
            </button>
          </footer>
        )}
      </AnimatedHeight>
    </section>
  );
}

/** Right column of a session row: always says something, so the row reads end to end. */
function SessionRowStatus({ raceSession, sessionStatus, currentInstant }: { raceSession: RaceSession; sessionStatus: SessionStatus; currentInstant: Date }) {
  const { messages, locale } = useLocale();
  const { effectiveTimeZone } = useSettings();
  if (sessionStatus === "live") return <span className="session-row__badge session-row__badge--live">● {messages.liveNow}</span>;
  if (sessionStatus === "finished") return <span className="session-row__meta">✓ {messages.sessionFinished}</span>;
  if (sessionStatus === "next") {
    return <span className="session-row__badge">{messages.startsIn(formatCountdown(new Date(raceSession.startsAt).getTime() - currentInstant.getTime()))}</span>;
  }
  if (!raceSession.startTimeConfirmed) return null;
  return (
    <span className="session-row__meta" title={messages.estimatedEndHint}>
      {messages.endsAround(formatTimeOfDay(estimatedEndsAt(raceSession), effectiveTimeZone, locale))}
    </span>
  );
}

function SessionSchedule({ raceEvent, currentInstant }: { raceEvent: RaceEvent; currentInstant: Date }) {
  const { messages, locale } = useLocale();
  const { effectiveTimeZone } = useSettings();
  const dayGroups = groupSessionsByDay(raceEvent.sessions, effectiveTimeZone, locale);
  const statusBySession = classifySessions(raceEvent.sessions, currentInstant);

  return (
    <ol className="session-schedule">
      {dayGroups.map((dayGroup) => (
        <li key={dayGroup.dayKey} className="session-schedule__day">
          <h3 className="session-schedule__day-label" aria-label={dayGroup.dayLabel} title={dayGroup.dayLabel}>
            <span className="session-schedule__weekday" aria-hidden="true">
              {dayGroup.weekdayShort}
            </span>
            <span className="session-schedule__day-number" aria-hidden="true">
              {dayGroup.dayOfMonth}
            </span>
          </h3>
          <ul>
            {dayGroup.sessions.map((raceSession) => {
              const sessionStatus = statusBySession.get(raceSession) ?? "upcoming";
              return (
                <li key={raceSession.sessionKind} className={`session-row session-row--${sessionStatus}`}>
                  <span className="session-row__time">
                    {raceSession.startTimeConfirmed ? formatTimeOfDay(raceSession.startsAt, effectiveTimeZone, locale) : messages.timeToBeConfirmed}
                  </span>
                  <span className="session-row__name">{messages.sessionLabels[raceSession.sessionKind]}</span>
                  <SessionRowStatus raceSession={raceSession} sessionStatus={sessionStatus} currentInstant={currentInstant} />
                </li>
              );
            })}
          </ul>
        </li>
      ))}
    </ol>
  );
}
