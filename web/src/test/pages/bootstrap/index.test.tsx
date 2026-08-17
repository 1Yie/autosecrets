import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { BootstrapPage } from "../../../pages/bootstrap";
import { useSessionStore } from "../../../stores/session-store";

describe("BootstrapPage", () => {
  it("requires code, username, and a 12+ char password", async () => {
    const qc = new QueryClient();
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter>
          <BootstrapPage />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    const button = screen.getByRole("button", { name: "创建管理员" });
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

  it("does not ask for an organization name", async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <BootstrapPage />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(screen.queryByTestId("organization-name")).not.toBeInTheDocument();
  });

  it("activates the Administrator and stores the new Session without an MFA wizard", async () => {
    useSessionStore.getState().clearSession();
    const qc = new QueryClient();
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter>
          <BootstrapPage />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.type(screen.getByTestId("code"), "code-123");
    await user.type(screen.getByTestId("username"), "admin");
    await user.type(screen.getByTestId("password"), "correct-horse-42");
    await user.click(screen.getByRole("button", { name: "创建管理员" }));

    expect(
      await screen.findByRole("heading", { name: "初始化 AutoSecrets" }),
    ).toBeVisible();
    expect(useSessionStore.getState().csrfToken).toBe("csrf-test-token");
    expect(qc.getQueryData(["me"])).toMatchObject({
      bootstrap_required: false,
      member: { username: "admin", role: "administrator" },
      totp_login_required: false,
      auth_method: "local",
    });
    expect(screen.queryByText("验证动态验证码")).toBeNull();
  });
});
