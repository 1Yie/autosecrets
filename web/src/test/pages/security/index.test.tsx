import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { SecurityPage } from "../../../pages/security";
import { server } from "../../server";

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <SecurityPage />
    </QueryClientProvider>,
  );
}

describe("SecurityPage", () => {
  it("shows password-only enablement when local TOTP is disabled", async () => {
    server.use(
      http.get("/api/v1/auth/security", () => HttpResponse.json({
        totp_login_required: false,
        oidc: { available: true, bound: false },
      })),
    );
    renderPage();
    expect(await screen.findByText("当前本地登录只需要用户名和密码。")).toBeVisible();
    expect(screen.queryByLabelText("当前动态验证码")).toBeNull();
    expect(screen.getByRole("button", { name: "启用本地 TOTP" })).toBeDisabled();
  });

  it("requires current TOTP for disablement and binding changes when enabled", async () => {
    server.use(
      http.get("/api/v1/auth/security", () => HttpResponse.json({
        totp_login_required: true,
        oidc: { available: true, bound: true, issuer: "https://id.example", display_name: "Administrator" },
      })),
    );
    renderPage();
    expect(await screen.findByText("用户名和密码验证后还需要动态验证码或恢复码。")).toBeVisible();
    expect(screen.getAllByLabelText("当前动态验证码")).toHaveLength(2);
    expect(screen.getByRole("button", { name: "停用本地 TOTP" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "解除绑定" })).toBeDisabled();
  });

  it("shows a sanitized configuration diagnostic after authentication", async () => {
    server.use(
      http.get("/api/v1/auth/security", () => HttpResponse.json({
        totp_login_required: false,
        oidc: { available: false, bound: false, configuration_error: "provider discovery timed out" },
      })),
    );
    renderPage();
    expect(await screen.findByText("provider discovery timed out")).toBeVisible();
    expect(screen.queryByRole("button", { name: "绑定 External Identity" })).toBeNull();
  });

  it("starts fresh TOTP enrollment only after current-password proof", async () => {
    server.use(
      http.get("/api/v1/auth/security", () => HttpResponse.json({
        totp_login_required: false,
        oidc: { available: false, bound: false },
      })),
      http.post("/api/v1/auth/totp/enrollment", () => HttpResponse.json({
        username: "admin",
        enrollment_token: "enrollment-token",
        totp_uri: "otpauth://totp/AutoSecrets:admin?secret=JBSWY3DPEHPK3PXP",
      }, { status: 201 })),
    );
    renderPage();
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText("输入当前密码以启用"), "correct-horse-42");
    await user.click(screen.getByRole("button", { name: "启用本地 TOTP" }));
    expect(await screen.findByRole("heading", { name: "验证动态验证码" })).toBeVisible();
  });
});
