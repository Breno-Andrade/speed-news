import { render } from "@testing-library/react";
import type { ReactElement } from "react";
import { SettingsProvider } from "../features/settings/SettingsProvider";
import { ApiClientProvider } from "../shared/api/ApiProvider";
import { LocaleProvider } from "../shared/i18n/LocaleProvider";
import { asApiClient, createFakeApiClient, type FakeApiClient } from "./fakeApiClient";

/** Renders like main.tsx does, with the fake API instead of the Go sidecar. */
export function renderWithProviders(componentUnderTest: ReactElement, fakeApiClient: FakeApiClient = createFakeApiClient()) {
  const renderResult = render(
    <LocaleProvider preferredLocale="pt-BR">
      <ApiClientProvider apiClient={asApiClient(fakeApiClient)}>
        <SettingsProvider>{componentUnderTest}</SettingsProvider>
      </ApiClientProvider>
    </LocaleProvider>,
  );
  return { ...renderResult, fakeApiClient };
}
