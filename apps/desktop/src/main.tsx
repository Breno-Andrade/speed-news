import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { StartupSkeleton } from "./StartupSkeleton";
import { SettingsProvider } from "./features/settings/SettingsProvider";
import { ApiProvider } from "./shared/api/ApiProvider";
import { LocaleProvider, useLocale } from "./shared/i18n/LocaleProvider";

/**
 * While the backend connection is resolved: the same skeleton index.html painted before the
 * scripts ran, so the first open goes skeleton → content without a blank or a jump.
 */
function ConnectionScreen({ errorMessage }: { errorMessage?: string }) {
  const { messages } = useLocale();

  if (errorMessage) {
    return (
      <div className="connection-screen fade-in">
        <p>{messages.connectionFailed}</p>
        <small>{errorMessage}</small>
      </div>
    );
  }
  return <StartupSkeleton label={messages.connecting} />;
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
