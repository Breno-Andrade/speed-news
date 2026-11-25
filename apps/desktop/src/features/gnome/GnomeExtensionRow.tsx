import { useLocale } from "../../shared/i18n/LocaleProvider";
import { shellReloadInstruction } from "./GnomeExtensionNotice";
import { useGnomeExtension } from "./useGnomeExtension";

/** Settings → General: state of the GNOME panel button, with install when it is missing. Hidden outside GNOME. */
export function GnomeExtensionRow() {
  const { messages } = useLocale();
  const { extensionStatus, stage, isInstalling, installFailed, install } = useGnomeExtension();
  if (stage === "notApplicable" || !extensionStatus) return null;

  let description = messages.gnomeExtensionRowHint;
  if (stage === "needsShellReload") description = `${messages.gnomeExtensionAlmostThere} ${shellReloadInstruction(messages, extensionStatus.sessionType)}`;
  if (installFailed) description = messages.gnomeExtensionInstallFailed;

  return (
    <div className="settings-row">
      <span className="settings-row__text">
        <span className="settings-row__label">{messages.gnomeExtensionRowLabel}</span>
        <span className="settings-row__description">{description}</span>
      </span>
      {stage === "active" && <span className="settings-row__value">{messages.gnomeExtensionActive}</span>}
      {stage === "notInstalled" && (
        <button type="button" className="secondary-button" onClick={install} disabled={isInstalling}>
          {messages.gnomeExtensionInstall}
        </button>
      )}
    </div>
  );
}
