import { NewsListSkeleton } from "./features/news/NewsListSkeleton";
import { RaceCardSkeleton } from "./features/race-weekend/RaceCardSkeletons";
import { SkeletonBlock } from "./shared/ui/Skeleton";
import { PowerIcon, RefreshIcon, SettingsIcon } from "./shared/ui/icons";

/** Accessible label of the copy inlined in index.html, painted before the language is known. */
export const STATIC_SKELETON_LABEL = "Speed News";

/**
 * The whole popover while nothing is loaded, with the real layout. The popover webview is created
 * on demand, so this is what every first open shows:
 *
 * 1. index.html carries this exact markup, painted with the stylesheet before any script runs
 *    (a test keeps both identical: `npm test` tells how to update index.html);
 * 2. React renders it while it waits for the backend connection;
 * 3. the app replaces each section in place (card, filters, news), so nothing moves.
 */
export function StartupSkeleton({ label }: { label: string }) {
  return (
    <div className="app-shell">
      <header className="app-header">
        <h1 className="app-header__title">Speed News</h1>
        <div className="app-header__actions" aria-hidden="true">
          {/* Same icons the real header shows, not clickable yet. */}
          <span className="icon-button icon-button--placeholder">
            <RefreshIcon />
          </span>
          <span className="icon-button icon-button--placeholder">
            <SettingsIcon />
          </span>
          <span className="icon-button icon-button--placeholder">
            <PowerIcon />
          </span>
        </div>
      </header>
      <main className="app-content">
        <div className="app-view">
          <RaceCardSkeleton label={label} />
          <NewsToolbarSkeleton />
          <NewsListSkeleton label={label} />
        </div>
      </main>
    </div>
  );
}

/** The "Unread / All" switch, whose words depend on the language not known yet. */
export function NewsToolbarSkeleton() {
  return (
    <div className="news-toolbar" aria-hidden="true">
      <SkeletonBlock width={116} height={27} radius={6} />
    </div>
  );
}
