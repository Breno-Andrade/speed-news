import { useEffect, useState, type CSSProperties, type ReactNode } from "react";

/** Most answers come from the backend cache in a few ms: a skeleton that flashes for one frame
 *  feels worse than nothing, so loading placeholders only appear after this delay. */
export const SKELETON_DELAY_MS = 150;

/** True only once `isActive` has stayed true for `delayMs`. */
export function useDelayedFlag(isActive: boolean, delayMs = SKELETON_DELAY_MS): boolean {
  const [isDelayedActive, setIsDelayedActive] = useState(false);
  useEffect(() => {
    if (!isActive) {
      setIsDelayedActive(false);
      return;
    }
    const delayTimer = window.setTimeout(() => setIsDelayedActive(true), delayMs);
    return () => window.clearTimeout(delayTimer);
  }, [isActive, delayMs]);
  return isDelayedActive;
}

type SkeletonBlockProps = {
  width?: CSSProperties["width"];
  height?: CSSProperties["height"];
  radius?: CSSProperties["borderRadius"];
  className?: string;
};

/** A shimmering placeholder shape. Purely decorative: the wrapper announces the loading state. */
export function SkeletonBlock({ width = "100%", height = 12, radius = 4, className = "" }: SkeletonBlockProps) {
  return <span className={`skeleton-block ${className}`} style={{ width, height, borderRadius: radius }} aria-hidden="true" />;
}

/** Wraps a skeleton layout: announced once to screen readers, hidden visually from them otherwise. */
export function SkeletonGroup({ label, children, className = "" }: { label: string; children: ReactNode; className?: string }) {
  return (
    <div className={`skeleton-group ${className}`} role="status" aria-busy="true" aria-label={label}>
      {children}
    </div>
  );
}
