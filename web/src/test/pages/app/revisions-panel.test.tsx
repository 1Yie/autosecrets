import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { RevisionsPanel } from "../../../pages/app/revisions-panel";

function Wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("RevisionsPanel", () => {
  it("shows the operation reason and rolls back to a snapshot", async () => {
    render(
      <Wrapper>
        <RevisionsPanel appId="app-1" envId="env-1" />
      </Wrapper>,
    );
    expect(await screen.findByText(/rotate the database password/)).toBeVisible();
    expect(screen.getByText(/maintenance/)).toBeVisible();

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "回滚到此版本" }));
    expect(screen.getByTestId("rollback-form")).toBeVisible();
    const confirm = screen.getByRole("button", { name: "确认回滚" });
    expect(confirm).toBeDisabled();
    await user.type(screen.getByTestId("reason-explanation"), "restore the previous working state");
    expect(confirm).toBeEnabled();
    await user.click(confirm);

    await waitFor(() => expect(screen.queryByTestId("rollback-form")).toBeNull());
  });
});
