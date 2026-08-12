import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { server } from "../../server";
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

  it("prompts for step-up when the server rejects with step_up_required", async () => {
    let calls = 0;
    server.use(
      http.post("/api/v1/applications/:appId/environments/:envId/publish", () => {
        calls += 1;
        if (calls === 1) {
          return HttpResponse.json(
            { error: "current password confirmation is required", code: "step_up_required" },
            { status: 403 },
          );
        }
        return HttpResponse.json(
          {
            id: "rev-2",
            draft_version: 3,
            file_count: 1,
            created_by: "admin:admin",
            created_at: "2026-08-12T12:01:00Z",
            operation_reason: { category: "other", explanation: "" },
          },
          { status: 201 },
        );
      }),
    );
    render(
      <Wrapper>
        <DraftPanel appId="app-1" envId="env-1" />
      </Wrapper>,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "发布" }));

    expect(await screen.findByTestId("step-up-password")).toBeVisible();
    await user.type(screen.getByTestId("step-up-password"), "correct-horse-42");
    await user.click(screen.getByRole("button", { name: "确认" }));

    await waitFor(() => expect(screen.queryByTestId("step-up-password")).toBeNull());
    expect(await screen.findByTestId("publish-success")).toBeVisible();
  });
});
