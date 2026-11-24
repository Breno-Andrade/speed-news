// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { hideMainWindow, isRunningInTauri, onPopoverShown, openExternalUrl, quitApp, setCloseOnFocusLoss, updateTray } from "./tauriBridge";

afterEach(() => vi.restoreAllMocks());

// In the browser (dev:web, tests) every native call must degrade gracefully.
describe("tauriBridge outside Tauri", () => {
  it("detects the browser", () => {
    expect(isRunningInTauri()).toBe(false);
  });

  it("mirrors the unread count in the tab title", async () => {
    await updateTray({ unreadCount: 4, tooltipText: "", openMenuLabel: "", quitMenuLabel: "" });
    expect(document.title).toBe("(4) Speed News");
    await updateTray({ unreadCount: 0, tooltipText: "", openMenuLabel: "", quitMenuLabel: "" });
    expect(document.title).toBe("Speed News");
  });

  it("opens links in a new tab without leaking the opener", async () => {
    const windowOpen = vi.spyOn(window, "open").mockReturnValue(null);
    await openExternalUrl("https://www.formula1.com");
    expect(windowOpen).toHaveBeenCalledWith("https://www.formula1.com", "_blank", "noopener,noreferrer");
  });

  it("turns native-only actions into no-ops", async () => {
    await expect(quitApp()).resolves.toBeUndefined();
    await expect(hideMainWindow()).resolves.toBeUndefined();
    await expect(setCloseOnFocusLoss(false)).resolves.toBeUndefined();
    const stopListening = onPopoverShown(() => undefined);
    expect(() => stopListening()).not.toThrow();
  });
});
