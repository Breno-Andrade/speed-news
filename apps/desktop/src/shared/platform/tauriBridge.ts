// Every call to the native shell goes through here, so the UI also runs in a plain browser during development.
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
