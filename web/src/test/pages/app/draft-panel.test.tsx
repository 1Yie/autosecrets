import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { DraftPanel } from "../../../pages/app/draft-panel";

function Wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("DraftPanel", () => {
  it("publishes in one click", async () => {
    render(
      <Wrapper>
        <DraftPanel appId="app-1" envId="env-1" />
      </Wrapper>,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "发布" }));
    expect(await screen.findByTestId("publish-success")).toBeVisible();
  });

});
