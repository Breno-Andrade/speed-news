import { useCallback, useEffect, useState } from "react";
import { getGnomeExtensionStatus, installGnomeExtension, type GnomeExtensionStatus } from "../../shared/platform/tauriBridge";

export type GnomeExtensionStage =
  /** Not GNOME, or not the desktop app: nothing to offer. */
  | "notApplicable"
  /** GNOME without the panel button: offer to install it. */
  | "notInstalled"
  /** Installed (or just installed) but GNOME Shell has not loaded it yet: explain the reload. */
  | "needsShellReload"
  /** The button is on the panel. */
  | "active";

export function gnomeExtensionStage(extensionStatus: GnomeExtensionStatus | null): GnomeExtensionStage {
  if (!extensionStatus?.isGnomeSession) return "notApplicable";
  if (extensionStatus.isActive) return "active";
  return extensionStatus.isInstalled ? "needsShellReload" : "notInstalled";
}

/** Status of the GNOME panel button, refreshed whenever the popover regains focus. */
export function useGnomeExtension() {
  const [extensionStatus, setExtensionStatus] = useState<GnomeExtensionStatus | null>(null);
  const [isInstalling, setIsInstalling] = useState(false);
  const [installFailed, setInstallFailed] = useState(false);

  useEffect(() => {
    const refreshStatus = () => {
      getGnomeExtensionStatus()
        .then(setExtensionStatus)
        .catch(() => undefined);
    };
    refreshStatus();
    window.addEventListener("focus", refreshStatus);
    return () => window.removeEventListener("focus", refreshStatus);
  }, []);

  const install = useCallback(async () => {
    setIsInstalling(true);
    setInstallFailed(false);
    try {
      setExtensionStatus(await installGnomeExtension());
    } catch {
      setInstallFailed(true);
    } finally {
      setIsInstalling(false);
    }
  }, []);

  return { extensionStatus, stage: gnomeExtensionStage(extensionStatus), isInstalling, installFailed, install };
}
