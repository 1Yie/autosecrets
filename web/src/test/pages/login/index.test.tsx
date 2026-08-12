import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { LoginPage } from "../../../pages/login";

describe("LoginPage", () => {
  it("requires username, password, and a 6-digit TOTP code", async () => {
    const qc = new QueryClient();
    render(
      <QueryClientProvider client={qc}>
        <LoginPage />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    const button = screen.getByRole("button", { name: "登录" });

    await user.type(screen.getByTestId("username"), "admin");
    await user.type(screen.getByTestId("password"), "correct-horse-42");
    await user.type(screen.getByTestId("totp-code"), "12345");
    await user.click(button);
    expect(await screen.findByText("请输入 6 位动态验证码")).toBeVisible();

    await user.clear(screen.getByTestId("totp-code"));
    await user.type(screen.getByTestId("totp-code"), "123456");
    await user.click(button);
    expect(screen.queryByText("请输入 6 位动态验证码")).toBeNull();
  });

  it("switches to the recovery code factor", async () => {
    const qc = new QueryClient();
    render(
      <QueryClientProvider client={qc}>
        <LoginPage />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.click(screen.getByTestId("factor-recovery"));
    expect(screen.getByTestId("recovery-code")).toBeVisible();
  });
});
