// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ApiProvider, useApiClient } from "./ApiProvider";

function ConnectedProbe() {
  const apiClient = useApiClient();
  return <p>{typeof apiClient.listArticles === "function" ? "connected" : "broken"}</p>;
}

describe("ApiProvider", () => {
  it("shows the connecting fallback, then the app once the backend address is known", async () => {
    render(
      <ApiProvider connectingFallback={(errorMessage) => <p>{errorMessage ? `failed: ${errorMessage}` : "connecting"}</p>}>
        <ConnectedProbe />
      </ApiProvider>,
    );
    expect(screen.getByText("connecting")).toBeInTheDocument();
    expect(await screen.findByText("connected")).toBeInTheDocument();
  });

  it("refuses to be used outside the provider", () => {
    const renderOutsideProvider = () => render(<ConnectedProbe />);
    // React logs the thrown error; the important part is that it throws with a clear message.
    expect(renderOutsideProvider).toThrow("useApiClient must be used inside <ApiProvider>");
  });
});
