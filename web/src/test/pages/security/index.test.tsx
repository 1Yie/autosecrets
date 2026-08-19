import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { SecurityPage } from "../../../pages/security";
import { ToastProvider } from "../../../components/ui/toast";
import { server } from "../../server";

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <MemoryRouter initialEntries={["/dashboard/settings"]}>
      <ToastProvider position="top-center">
        <QueryClientProvider client={client}>
          <SecurityPage />
        </QueryClientProvider>
      </ToastProvider>
    </MemoryRouter>,
  );
}

async function openSection(name: string) {
  const user = userEvent.setup();
  await user.click(await screen.findByRole("tab", { name }));
  return user;
}

describe("SecurityPage", () => {
  it("shows password-only enablement when local TOTP is disabled", async () => {
    server.use(
      http.get("/api/v1/auth/security", () =>
        HttpResponse.json({
          totp_login_required: false,
          oidc: { available: true, bound: false },
        }),
      ),
    );
    renderPage();
    await openSection("TOTP");
    expect(await screen.findByText("本地 TOTP")).toBeVisible();
    expect(screen.getByText("已停用")).toBeVisible();
    expect(screen.queryByLabelText("当前动态验证码")).toBeNull();
    expect(
      screen.getByRole("button", { name: "启用本地 TOTP" }),
    ).toBeDisabled();
  });

  it("requires current TOTP for disablement and binding changes when enabled", async () => {
    server.use(
      http.get("/api/v1/auth/security", () =>
        HttpResponse.json({
          totp_login_required: true,
          oidc: {
            available: true,
            bound: true,
            issuer: "https://id.example",
            display_name: "Administrator",
          },
        }),
      ),
    );
    renderPage();
    await openSection("TOTP");
    expect(await screen.findByText("本地 TOTP")).toBeVisible();
    expect(screen.getByText("已启用")).toBeVisible();
    expect(screen.getByTestId("local-totp")).toBeVisible();
    expect(
      screen.getByRole("button", { name: "停用本地 TOTP" }),
    ).toBeDisabled();
    await openSection("外部登录");
    expect(screen.getByRole("button", { name: "解除绑定" })).toBeDisabled();
  });

  it("shows a sanitized configuration diagnostic after authentication", async () => {
    server.use(
      http.get("/api/v1/auth/security", () =>
        HttpResponse.json({
          totp_login_required: false,
          oidc: {
            available: false,
            bound: false,
            configuration_error: "OIDC is not configured",
          },
        }),
      ),
    );
    renderPage();
    await openSection("外部登录");
    expect(await screen.findByText("尚未配置 OpenID Connect")).toBeVisible();
    expect(screen.getByText("尚未配置 OAuth")).toBeVisible();
    expect(screen.queryByText("OIDC is not configured")).toBeNull();
    expect(
      screen.queryByRole("button", { name: "绑定 External Identity" }),
    ).toBeNull();
  });

  it("starts fresh TOTP enrollment only after current-password proof", async () => {
    server.use(
      http.get("/api/v1/auth/security", () =>
        HttpResponse.json({
          totp_login_required: false,
          oidc: { available: false, bound: false },
        }),
      ),
      http.post("/api/v1/auth/totp/enrollment", () =>
        HttpResponse.json(
          {
            username: "admin",
            enrollment_token: "enrollment-token",
            totp_uri:
              "otpauth://totp/AutoSecrets:admin?secret=JBSWY3DPEHPK3PXP",
          },
          { status: 201 },
        ),
      ),
    );
    renderPage();
    const user = await openSection("TOTP");
    await user.type(
      await screen.findByLabelText("输入当前密码以启用"),
      "correct-horse-42",
    );
    await user.click(screen.getByRole("button", { name: "启用本地 TOTP" }));
    expect(
      await screen.findByRole("heading", { name: "验证动态验证码" }),
    ).toBeVisible();
  });

  it("renames Username after current-password proof and stays on the page", async () => {
    server.use(
      http.get("/api/v1/auth/security", () =>
        HttpResponse.json({
          totp_login_required: false,
          oidc: { available: false, bound: false },
        }),
      ),
      http.post("/api/v1/auth/username", async ({ request }) => {
        const body = (await request.json()) as { username: string };
        return HttpResponse.json({
          csrf_token: "csrf-renamed",
          username: body.username,
          id: "admin-1",
          role: "administrator",
          expires_at: "2026-08-12T12:00:00Z",
        });
      }),
    );
    renderPage();
    const user = userEvent.setup();
    const username = await screen.findByTestId("change-username");
    await user.clear(username);
    await user.type(username, "alice");
    await user.type(
      screen.getByTestId("username-password"),
      "correct-horse-42",
    );
    await user.click(screen.getByRole("button", { name: "更新用户名" }));
    expect(await screen.findByText("用户名已更新")).toBeVisible();
    expect(screen.getByRole("heading", { name: "设置" })).toBeVisible();
  });

  it("updates the password after current-password proof and stays on the page", async () => {
    server.use(
      http.get("/api/v1/auth/security", () =>
        HttpResponse.json({
          totp_login_required: false,
          oidc: { available: false, bound: false },
        }),
      ),
      http.post("/api/v1/auth/password", () =>
        HttpResponse.json({
          csrf_token: "csrf-password",
          username: "admin",
          id: "admin-1",
          role: "administrator",
          expires_at: "2026-08-12T12:00:00Z",
        }),
      ),
    );
    renderPage();
    const user = userEvent.setup();
    await user.type(
      await screen.findByTestId("password-current"),
      "correct-horse-42",
    );
    await user.type(
      screen.getByTestId("password-new"),
      "correct-horse-battery-42",
    );
    await user.click(screen.getByRole("button", { name: "更新密码" }));
    expect(await screen.findByText("密码已更新")).toBeVisible();
    expect(screen.getByRole("heading", { name: "设置" })).toBeVisible();
  });

  it("keeps password login on until an External Identity Binding is usable", async () => {
    server.use(
      http.get("/api/v1/auth/security", () =>
        HttpResponse.json({
          totp_login_required: false,
          password_login_enabled: true,
          password_login_available: true,
          oidc: { available: true, bound: false },
          oauth: { available: false, bound: false },
        }),
      ),
    );
    renderPage();
    await openSection("登录配置");
    expect(await screen.findByText("密码登录")).toBeVisible();
    expect(screen.getByRole("switch", { name: "密码登录" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    expect(
      screen.getByText("需要先绑定可用的 OAuth 或 OpenID Connect"),
    ).toBeVisible();
  });

  it("closes password login after current-password proof", async () => {
    let enabled = true;
    server.use(
      http.get("/api/v1/auth/security", () =>
        HttpResponse.json({
          totp_login_required: false,
          password_login_enabled: enabled,
          password_login_available: enabled,
          oidc: {
            available: true,
            bound: true,
            issuer: "https://id.example",
          },
          oauth: { available: false, bound: false },
        }),
      ),
      http.put("/api/v1/auth/password-login", async ({ request }) => {
        const body = (await request.json()) as { enabled: boolean };
        enabled = body.enabled;
        return HttpResponse.json({
          password_login_enabled: enabled,
          password_login_available: enabled,
        });
      }),
    );
    renderPage();
    const user = await openSection("登录配置");
    await user.click(await screen.findByRole("switch", { name: "密码登录" }));
    await user.type(
      await screen.findByTestId("password-login-password"),
      "correct-horse-42",
    );
    await user.click(screen.getByRole("button", { name: "关闭密码登录" }));
    expect(await screen.findByText("已关闭密码登录")).toBeVisible();
  });
});
