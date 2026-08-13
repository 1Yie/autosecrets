import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { describe, expect, it, vi } from "vitest";
import { NodesPage } from "../../../pages/nodes";
import { server } from "../../server";

function renderPage() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={qc}>
      <NodesPage />
    </QueryClientProvider>,
  );
}

describe("NodesPage install command", () => {
  it("renders the one-time install command with the token, once", async () => {
    renderPage();
    const user = userEvent.setup();
    await user.type(screen.getByTestId("node-name"), "web-1");
    await user.click(screen.getByRole("button", { name: "生成" }));

    const command = await screen.findByTestId("install-command");
    expect(command.textContent).toContain("--token one-time-token-abc");
    expect(command.textContent).toContain("--server https://agent.example.com");
  });

  it("accepts an explicit Materialized Bundle deployment path", async () => {
    let requestBody: unknown;
    server.use(http.post("/api/v1/nodes/install-command", async ({ request }) => {
      requestBody = await request.json();
      return HttpResponse.json({
        command: "curl install.sh --token one-time-token-abc",
        expires_at: "2026-08-12T02:00:00Z",
      });
    }));
    renderPage();
    const user = userEvent.setup();
    await user.clear(screen.getByTestId("node-name"));
    await user.type(screen.getByTestId("node-name"), "web-1");
    await user.clear(screen.getByTestId("node-bundle-dir"));
    await user.type(screen.getByTestId("node-bundle-dir"), "/srv/autosecrets");
    await user.click(screen.getByRole("button", { name: "生成" }));
    await screen.findByTestId("install-command");
    expect(requestBody).toEqual({ name: "web-1", bundle_dir: "/srv/autosecrets" });
  });

  it("shows the command once and never re-renders the token after dismissal", async () => {
    renderPage();
    const user = userEvent.setup();
    await user.type(screen.getByTestId("node-name"), "web-1");
    await user.click(screen.getByRole("button", { name: "生成" }));
    const command = await screen.findByTestId("install-command");
    expect(screen.getAllByTestId("install-command")).toHaveLength(1);
    expect(command.textContent).toContain("one-time-token-abc");
  });

  it("offers a copy affordance for the command", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    renderPage();
    const user = userEvent.setup();
    vi.stubGlobal("navigator", { ...navigator, clipboard: { writeText } });
    await user.type(screen.getByTestId("node-name"), "web-1");
    await user.click(screen.getByRole("button", { name: "生成" }));
    await screen.findByTestId("install-command");
    fireEvent.click(screen.getByRole("button", { name: "复制命令" }));
    await waitFor(() => expect(writeText).toHaveBeenCalled());
    expect(writeText.mock.calls[0][0]).toContain("one-time-token-abc");
  });
});
