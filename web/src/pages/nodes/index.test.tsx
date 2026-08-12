import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { NodesPage } from "./index";

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
    await user.click(screen.getByRole("button", { name: "Generate" }));

    const command = await screen.findByTestId("install-command");
    expect(command.textContent).toContain("--token one-time-token-abc");
    expect(command.textContent).toContain("--server https://agent.example.com");
  });

  it("shows the command once and never re-renders the token after dismissal", async () => {
    renderPage();
    const user = userEvent.setup();
    await user.type(screen.getByTestId("node-name"), "web-1");
    await user.click(screen.getByRole("button", { name: "Generate" }));
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
    await user.click(screen.getByRole("button", { name: "Generate" }));
    await screen.findByTestId("install-command");
    fireEvent.click(screen.getByRole("button", { name: "Copy command" }));
    await waitFor(() => expect(writeText).toHaveBeenCalled());
    expect(writeText.mock.calls[0][0]).toContain("one-time-token-abc");
  });
});
