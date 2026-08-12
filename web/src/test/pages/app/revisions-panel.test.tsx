import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { RevisionsPanel } from "../../../pages/app/revisions-panel";

function Wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("RevisionsPanel", () => {
  it("rolls back to a snapshot in one click", async () => {
    render(
      <Wrapper>
        <RevisionsPanel appId="app-1" envId="env-1" />
      </Wrapper>,
    );
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "回滚到此版本" }));
    expect(await screen.findByTestId("rollback-success")).toBeVisible();
  });
});
