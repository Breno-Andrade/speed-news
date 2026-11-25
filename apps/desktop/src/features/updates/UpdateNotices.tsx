import { useEffect, useState } from "react";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import { getAppVersion, getPendingUpdate, installPendingUpdate, onUpdateReady, type UpdateReady } from "../../shared/platform/tauriBridge";

const LAST_SEEN_VERSION_KEY = "speed-news:last-seen-version";
const UPDATED_NOTICE_MS = 6000;

// localStorage is only a per-device convenience here: if it is unavailable, the notice is skipped.
function readLastSeenVersion(): string | null {
  try {
    return window.localStorage.getItem(LAST_SEEN_VERSION_KEY);
  } catch {
    return null;
  }
}

function saveLastSeenVersion(appVersion: string) {
  try {
    window.localStorage.setItem(LAST_SEEN_VERSION_KEY, appVersion);
  } catch {
    // Private mode or blocked storage: nothing to remember.
  }
}

/**
 * "Updated to vX" once after a silent update, and, for .deb/.rpm installs, the offer to install a
 * downloaded update (it asks for the admin password, so the user chooses when).
 */
export function UpdateNotices() {
  const { messages } = useLocale();
  const [updatedToVersion, setUpdatedToVersion] = useState<string | null>(null);
  const [readyUpdate, setReadyUpdate] = useState<UpdateReady | null>(null);
  const [isInstalling, setIsInstalling] = useState(false);

  useEffect(() => {
    getAppVersion()
      .then((appVersion) => {
        if (!appVersion) return;
        const lastSeenVersion = readLastSeenVersion();
        // First run ever: nothing was "updated", just remember the version.
        if (lastSeenVersion && lastSeenVersion !== appVersion) setUpdatedToVersion(appVersion);
        saveLastSeenVersion(appVersion);
      })
      .catch(() => undefined);
    // The webview may have been created after the download finished: ask, then keep listening.
    getPendingUpdate()
      .then((pendingUpdate) => {
        if (pendingUpdate) setReadyUpdate(pendingUpdate);
      })
      .catch(() => undefined);
    return onUpdateReady(setReadyUpdate);
  }, []);

  useEffect(() => {
    if (!updatedToVersion) return;
    const hideTimer = window.setTimeout(() => setUpdatedToVersion(null), UPDATED_NOTICE_MS);
    return () => window.clearTimeout(hideTimer);
  }, [updatedToVersion]);

  async function installNow() {
    setIsInstalling(true);
    try {
      await installPendingUpdate(); // restarts the app on success
    } catch {
      setIsInstalling(false); // password prompt cancelled or install failed: keep the offer
    }
  }

  if (readyUpdate) {
    return (
      <div className="update-notice fade-in" role="status">
        <span>{messages.updateReady(readyUpdate.version)}</span>
        <button type="button" className="update-notice__action" onClick={installNow} disabled={isInstalling}>
          {messages.installUpdate}
        </button>
        {readyUpdate.requiresPassword && <small className="update-notice__hint">{messages.installUpdatePasswordHint}</small>}
      </div>
    );
  }
  if (updatedToVersion) {
    return (
      <div className="update-notice update-notice--done fade-in" role="status">
        <span>✓ {messages.updatedTo(updatedToVersion)}</span>
        <button type="button" className="update-notice__dismiss" aria-label={messages.dismiss} onClick={() => setUpdatedToVersion(null)}>
          ×
        </button>
      </div>
    );
  }
  return null;
}
