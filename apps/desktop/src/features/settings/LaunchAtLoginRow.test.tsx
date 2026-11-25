// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "../../shared/i18n/LocaleProvider";
import { LaunchAtLoginRow } from "./LaunchAtLoginRow";

const bridgeMocks = vi.hoisted(() => ({
  isLaunchAtLoginEnabled: vi.fn<() => Promise<boolean | null>>(),
  setLaunchAtLogin: vi.fn<(isEnabled: boolean) => Promise<void>>(),
}));
vi.mock("../../shared/platform/tauriBridge", () => bridgeMocks);

async function renderRow() {
  render(
    <LocaleProvider preferredLocale="pt-BR">
      <LaunchAtLoginRow />
    </LocaleProvider>,
  );
  await act(async () => undefined);
}

beforeEach(() => {
  bridgeMocks.isLaunchAtLoginEnabled.mockResolvedValue(true);
  bridgeMocks.setLaunchAtLogin.mockResolvedValue(undefined);
});

describe("LaunchAtLoginRow", () => {
  it("is hidden in the browser", async () => {
    bridgeMocks.isLaunchAtLoginEnabled.mockResolvedValue(null);
    await renderRow();
    expect(screen.queryByRole("switch")).not.toBeInTheDocument();
  });

  it("shows the OS state and turns it off", async () => {
    const user = userEvent.setup();
    await renderRow();
    const launchSwitch = screen.getByRole("switch", { name: /Iniciar com o computador/ });
    expect(launchSwitch).toBeChecked();

    await user.click(launchSwitch);

    expect(bridgeMocks.setLaunchAtLogin).toHaveBeenCalledWith(false);
    expect(launchSwitch).not.toBeChecked();
  });

  it("goes back to the real state when the OS refuses", async () => {
    bridgeMocks.setLaunchAtLogin.mockRejectedValue(new Error("denied"));
    const user = userEvent.setup();
    await renderRow();

    await user.click(screen.getByRole("switch"));

    expect(screen.getByRole("switch")).toBeChecked();
  });
});
