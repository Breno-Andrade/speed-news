// @vitest-environment jsdom
import { act } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test/renderWithProviders";
import { createFakeApiClient } from "../../test/fakeApiClient";

const bridgeMocks = vi.hoisted(() => ({
  reportSystemLocale: vi.fn<(systemLocale: string) => Promise<void>>(),
  mirrorUnreadCountInTitle: vi.fn<(indicatedCount: number) => void>(),
  isRunningInTauri: () => true,
}));
vi.mock("../../shared/platform/tauriBridge", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../shared/platform/tauriBridge")>()),
  ...bridgeMocks,
}));

beforeEach(() => {
  bridgeMocks.reportSystemLocale.mockReset().mockResolvedValue(undefined);
});

describe("SettingsProvider and the native shell", () => {
  it("reports the detected system locale so the tray texts match the popover", async () => {
    vi.spyOn(navigator, "language", "get").mockReturnValue("en-GB");
    renderWithProviders(<p>content</p>, createFakeApiClient());
    await act(async () => undefined);
    expect(bridgeMocks.reportSystemLocale).toHaveBeenCalledWith("en-GB");
  });

  it("reports a changed system locale when the popover regains focus", async () => {
    const languageGetter = vi.spyOn(navigator, "language", "get").mockReturnValue("pt-BR");
    renderWithProviders(<p>content</p>, createFakeApiClient());
    await act(async () => undefined);

    languageGetter.mockReturnValue("en-US");
    await act(async () => {
      window.dispatchEvent(new Event("focus"));
    });
    expect(bridgeMocks.reportSystemLocale).toHaveBeenLastCalledWith("en-US");
  });

  it("keeps working when the shell cannot be reached", async () => {
    bridgeMocks.reportSystemLocale.mockRejectedValue(new Error("no shell"));
    renderWithProviders(<p>content</p>, createFakeApiClient());
    await act(async () => undefined);
    expect(bridgeMocks.reportSystemLocale).toHaveBeenCalled();
  });
});
