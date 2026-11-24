import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { NewsListSkeleton } from "./features/news/NewsListSkeleton";
import { RaceCardSkeleton } from "./features/race-weekend/RaceCardSkeletons";
import { SkeletonBlock, useDelayedFlag } from "./shared/ui/Skeleton";
import { SettingsProvider } from "./features/settings/SettingsProvider";
import { ApiProvider } from "./shared/api/ApiProvider";
import { LocaleProvider, useLocale } from "./shared/i18n/LocaleProvider";
import "./styles.css";

/** While the backend starts: a skeleton of the whole app, so the real screen simply fills in. */
function ConnectionScreen({ errorMessage }: { errorMessage?: string }) {
  const { messages } = useLocale();
  const showsSkeleton = useDelayedFlag(!errorMessage);

  if (errorMessage) {
    return (
      <div className="connection-screen fade-in">
        <p>{messages.connectionFailed}</p>
        <small>{errorMessage}</small>
      </div>
    );
  }
  if (!showsSkeleton) return null;
  return (
    <div className="app-shell fade-in">
      <header className="app-header">
        <SkeletonBlock width={110} height={16} />
        <SkeletonBlock width={76} height={16} />
      </header>
      <main className="app-content">
        <div className="app-view">
          <RaceCardSkeleton label={messages.connecting} />
          <NewsListSkeleton label={messages.connecting} />
        </div>
      </main>
    </div>
  );
}

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("#root element not found");

createRoot(rootElement).render(
  <StrictMode>
    <LocaleProvider>
      <ApiProvider connectingFallback={(errorMessage) => <ConnectionScreen errorMessage={errorMessage} />}>
        <SettingsProvider>
          <App />
        </SettingsProvider>
      </ApiProvider>
    </LocaleProvider>
  </StrictMode>,
);
