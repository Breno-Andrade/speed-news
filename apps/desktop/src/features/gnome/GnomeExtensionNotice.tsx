import { useState } from "react";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { Messages } from "../../shared/i18n/messages";
import { useGnomeExtension } from "./useGnomeExtension";

const DISMISSED_KEY = "speed-news:gnome-extension-notice-dismissed";

// Per-device convenience: if storage is unavailable the notice simply shows again.
function readDismissed(): boolean {
  try {
    return window.localStorage.getItem(DISMISSED_KEY) === "true";
  } catch {
    return false;
  }
}

function saveDismissed() {
  try {
    window.localStorage.setItem(DISMISSED_KEY, "true");
  } catch {
    // Nothing to remember.
  }
}

export function shellReloadInstruction(messages: Messages, sessionType: "x11" | "wayland" | "unknown"): string {
  if (sessionType === "x11") return messages.reloadGnomeOnX11;
  if (sessionType === "wayland") return messages.reloadGnomeOnWayland;
  return messages.reloadGnomeUnknown;
}

/**
 * First run on GNOME: without the panel button, a single click on the tray icon opens a menu
 * (Ubuntu's AppIndicator behaviour). Offer the one-click install, then explain the reload.
 */
export function GnomeExtensionNotice() {
  const { messages } = useLocale();
  const { extensionStatus, stage, isInstalling, installFailed, install } = useGnomeExtension();
  const [isDismissed, setIsDismissed] = useState(readDismissed);

  function dismiss() {
    saveDismissed();
    setIsDismissed(true);
  }

  if (stage === "notApplicable" || stage === "active" || !extensionStatus) return null;

  if (stage === "needsShellReload") {
    return (
      <div className="update-notice fade-in" role="status">
        <span>{messages.gnomeExtensionAlmostThere}</span>
        <small className="update-notice__hint">{shellReloadInstruction(messages, extensionStatus.sessionType)}</small>
      </div>
    );
  }

  if (isDismissed) return null;
  return (
    <div className="update-notice fade-in" role="status">
      <span>{messages.gnomeExtensionOffer}</span>
      <button type="button" className="update-notice__action" onClick={install} disabled={isInstalling}>
        {messages.gnomeExtensionInstall}
      </button>
      <button type="button" className="update-notice__dismiss" aria-label={messages.dismiss} onClick={dismiss}>
        ×
      </button>
      {installFailed && <small className="update-notice__hint">{messages.gnomeExtensionInstallFailed}</small>}
    </div>
  );
}
