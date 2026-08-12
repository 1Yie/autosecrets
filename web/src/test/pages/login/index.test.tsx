import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { server } from "../../server";
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

  it("offers MFA enrollment for legacy accounts and completes the flow", async () => {
    server.use(
      http.post("/api/v1/auth/login", () =>
        HttpResponse.json(
          { error: "MFA enrollment is required before login", code: "mfa_enrollment_required" },
          { status: 403 },
        ),
      ),
    );
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={qc}>
        <LoginPage />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.type(screen.getByTestId("username"), "dev");
    await user.type(screen.getByTestId("password"), "correct-horse-42");
    await user.type(screen.getByTestId("totp-code"), "123456");
    await user.click(screen.getByRole("button", { name: "登录" }));

    expect(await screen.findByText("此账号需要先完成 MFA 注册")).toBeVisible();
    await user.click(screen.getByTestId("resume-mfa"));
    expect(await screen.findByText("验证动态验证码")).toBeVisible();
    await user.type(screen.getByTestId("totp-code"), "123456");
    await user.click(screen.getByRole("button", { name: "验证" }));
    expect(await screen.findByText("保存恢复码")).toBeVisible();
    await user.click(screen.getByTestId("recovery-ack"));
    await user.click(screen.getByRole("button", { name: "完成注册" }));
    expect(await screen.findByRole("heading", { name: "登录" })).toBeVisible();
  });
});
