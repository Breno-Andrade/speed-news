import { SkeletonBlock, SkeletonGroup } from "../../shared/ui/Skeleton";

/** Placeholder with the race card's real layout: header, map, tabs and a few session rows. */
export function RaceCardSkeleton({ label }: { label: string }) {
  return (
    <SkeletonGroup label={label} className="race-card">
      <div className="race-card__header">
        <div className="race-card__skeleton-heading">
          <SkeletonBlock width={150} height={10} />
          <SkeletonBlock width={180} height={20} />
          <SkeletonBlock width={200} height={11} />
        </div>
        <SkeletonBlock width={96} height={96} radius={12} />
      </div>
      <SkeletonBlock height={26} radius={6} className="race-card__skeleton-tabs" />
      <SessionRowsSkeleton />
    </SkeletonGroup>
  );
}

function SessionRowsSkeleton() {
  return (
    <div className="race-card__skeleton-rows">
      {[0, 1, 2].map((dayIndex) => (
        // Same shape as the schedule: day column on the left, sessions beside it.
        <div key={dayIndex} className="session-schedule__day">
          <SkeletonBlock width={32} height={30} radius={6} />
          <div className="race-card__skeleton-day">
            <SkeletonBlock width="85%" height={13} />
            {dayIndex < 2 && <SkeletonBlock width="70%" height={13} />}
          </div>
        </div>
      ))}
    </div>
  );
}

export function PodiumSkeleton({ label }: { label: string }) {
  return (
    <SkeletonGroup label={label} className="insight-panel">
      <SkeletonBlock width={160} height={13} />
      {[0, 1, 2].map((podiumIndex) => (
        <div key={podiumIndex} className="podium-row">
          <SkeletonBlock width={20} height={20} radius={10} />
          <span className="podium-row__driver">
            <SkeletonBlock width={130} height={13} />
            <SkeletonBlock width={70} height={9} />
          </span>
          <SkeletonBlock width={64} height={12} />
        </div>
      ))}
      <SkeletonBlock width="75%" height={11} />
    </SkeletonGroup>
  );
}

export function CircuitSkeleton({ label }: { label: string }) {
  return (
    <SkeletonGroup label={label} className="insight-panel">
      <div className="circuit-facts">
        {[0, 1, 2, 3].map((factIndex) => (
          <SkeletonBlock key={factIndex} height={44} radius={6} />
        ))}
      </div>
      <div className="insight-section">
        <SkeletonBlock width={70} height={12} />
        <SkeletonBlock width="85%" height={13} />
        <SkeletonBlock width="60%" height={11} />
      </div>
    </SkeletonGroup>
  );
}

export function ForecastSkeleton({ label }: { label: string }) {
  return (
    <SkeletonGroup label={label} className="insight-panel">
      {[0, 1, 2, 3, 4].map((sessionIndex) => (
        <div key={sessionIndex} className="forecast-row">
          <SkeletonBlock width="80%" height={12} />
          <SkeletonBlock width={20} height={20} radius={10} />
          <SkeletonBlock width={36} height={14} />
          <SkeletonBlock width="90%" height={11} />
        </div>
      ))}
    </SkeletonGroup>
  );
}
