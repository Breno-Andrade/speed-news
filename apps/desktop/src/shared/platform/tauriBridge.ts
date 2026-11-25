// Every call to the native shell goes through here, so the UI also runs in a plain browser during development.
import { getVersion } from "@tauri-apps/api/app";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { openUrl } from "@tauri-apps/plugin-opener";
import type { ApiConnection } from "../api/types";

export function isRunningInTauri(): boolean {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

/** Returns null while the Go sidecar is still starting. */
export async function fetchSidecarConnection(): Promise<ApiConnection | null> {
  return invoke<ApiConnection | null>("get_api_connection");
}

export type TrayState = {
  unreadCount: number;
  tooltipText: string;
  openMenuLabel: string;
  quitMenuLabel: string;
};

/** Count next to the tray icon plus localized tooltip and menu. In the browser, mirrors the count in the tab title. */
export async function updateTray(trayState: TrayState): Promise<void> {
  if (!isRunningInTauri()) {
    document.title = trayState.unreadCount > 0 ? `(${trayState.unreadCount}) Speed News` : "Speed News";
    return;
  }
  await invoke("update_tray", trayState);
}

/** Mirrors the "close when clicking outside" setting into the native shell. */
export async function setCloseOnFocusLoss(isEnabled: boolean): Promise<void> {
  if (isRunningInTauri()) {
    await invoke("set_close_on_focus_loss", { isEnabled });
  }
}

/**
 * Calls `onShown` every time the native shell shows the popover (from the panel or tray).
 * Returns a function that stops listening. No-op in the browser.
 */
export function onPopoverShown(onShown: () => void): () => void {
  if (!isRunningInTauri()) return () => undefined;
  let stopListening: (() => void) | undefined;
  let isStopped = false;
  listen("popover-shown", onShown)
    .then((unlisten) => {
      if (isStopped) unlisten();
      else stopListening = unlisten;
    })
    .catch(() => undefined);
  return () => {
    isStopped = true;
    stopListening?.();
  };
}

/** Installed app version (from package.json via tauri.conf.json); null in the browser. */
export async function getAppVersion(): Promise<string | null> {
  if (!isRunningInTauri()) return null;
  return getVersion();
}

export type UpdateReady = { version: string; requiresPassword: boolean };

/**
 * .deb/.rpm installs only: an update was downloaded and needs the admin password to install,
 * so the UI asks first (other installs update silently while the popover is hidden).
 */
export function onUpdateReady(onReady: (updateReady: UpdateReady) => void): () => void {
  if (!isRunningInTauri()) return () => undefined;
  let stopListening: (() => void) | undefined;
  let isStopped = false;
  listen<UpdateReady>("update-ready", (updateEvent) => onReady(updateEvent.payload))
    .then((unlisten) => {
      if (isStopped) unlisten();
      else stopListening = unlisten;
    })
    .catch(() => undefined);
  return () => {
    isStopped = true;
    stopListening?.();
  };
}

/** Installs the downloaded update (system password prompt on .deb/.rpm), then restarts. */
export async function installPendingUpdate(): Promise<void> {
  if (isRunningInTauri()) {
    await invoke("install_pending_update");
  }
}

export type GnomeExtensionStatus = {
  isGnomeSession: boolean;
  sessionType: "x11" | "wayland" | "unknown";
  isInstalled: boolean;
  /** The panel button is connected to the app right now. */
  isActive: boolean;
};

/** null outside the desktop app (browser) — there is nothing to install there. */
export async function getGnomeExtensionStatus(): Promise<GnomeExtensionStatus | null> {
  if (!isRunningInTauri()) return null;
  return invoke<GnomeExtensionStatus>("gnome_extension_status");
}

/** Copies the bundled GNOME extension and enables it; active after the next GNOME Shell reload. */
export async function installGnomeExtension(): Promise<GnomeExtensionStatus> {
  return invoke<GnomeExtensionStatus>("install_gnome_extension");
}

/** Start at login (the OS keeps this, not the backend). null in the browser. */
export async function isLaunchAtLoginEnabled(): Promise<boolean | null> {
  if (!isRunningInTauri()) return null;
  return invoke<boolean>("is_launch_at_login_enabled");
}

export async function setLaunchAtLogin(isEnabled: boolean): Promise<void> {
  if (!isRunningInTauri()) return;
  await invoke("set_launch_at_login", { isEnabled });
}

/** Esc key: hides the popover (the app keeps running in the panel). */
export async function hideMainWindow(): Promise<void> {
  if (isRunningInTauri()) {
    await invoke("hide_main_window");
  }
}

/** Quits the app (and its Go sidecar). Only meaningful inside Tauri. */
export async function quitApp(): Promise<void> {
  if (isRunningInTauri()) {
    await invoke("quit_app");
  }
}

export async function openExternalUrl(externalUrl: string): Promise<void> {
  if (isRunningInTauri()) {
    await openUrl(externalUrl);
    return;
  }
  window.open(externalUrl, "_blank", "noopener,noreferrer");
}
