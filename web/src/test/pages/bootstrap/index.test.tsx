import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { BootstrapPage } from "../../../pages/bootstrap";

describe("BootstrapPage", () => {
  it("requires a bootstrap code, username, and a 10+ char password", async () => {
    const qc = new QueryClient();
    render(
      <QueryClientProvider client={qc}>
        <BootstrapPage />
      </QueryClientProvider>,
    );
    const button = screen.getByRole("button", { name: "Create administrator" });
    expect(button).toBeDisabled();

    const user = userEvent.setup();
    await user.type(screen.getByTestId("code"), "code-123");
    await user.type(screen.getByTestId("username"), "admin");
    await user.type(screen.getByTestId("password"), "short");
    expect(button).toBeDisabled();

    await user.clear(screen.getByTestId("password"));
    await user.type(screen.getByTestId("password"), "correct-horse-42");
    expect(button).toBeEnabled();
  });
});
