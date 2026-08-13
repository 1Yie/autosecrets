import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { server } from "../../server";
import { LoginPage } from "../../../pages/login";

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><LoginPage /></MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("LoginPage", () => {
  it("shows only username and password before Core requests a second factor", async () => {
    renderPage();
    expect(screen.getByTestId("username")).toBeVisible();
    expect(screen.getByTestId("password")).toBeVisible();
    expect(screen.queryByTestId("totp-code")).toBeNull();
    expect(screen.queryByTestId("recovery-code")).toBeNull();
  });

  it("moves to a dedicated second-factor step after the password challenge", async () => {
    server.use(
      http.post("/api/v1/auth/login", () =>
        HttpResponse.json({ status: "second_factor_required", code: "second_factor_required" }, { status: 202 }),
      ),
      http.post("/api/v1/auth/login/second-factor", () =>
        HttpResponse.json({ csrf_token: "csrf", username: "admin", id: "admin-1", role: "administrator", expires_at: "2026-08-12T12:00:00Z" }),
      ),
    );
    renderPage();
    const user = userEvent.setup();
    await user.type(screen.getByTestId("username"), "admin");
    await user.type(screen.getByTestId("password"), "correct-horse-42");
    await user.click(screen.getByRole("button", { name: "登录" }));

    expect(await screen.findByRole("heading", { name: "验证第二因子" })).toBeVisible();
    expect(screen.getByTestId("totp-code")).toBeVisible();
    await user.type(screen.getByTestId("totp-code"), "12345");
    await user.click(screen.getByRole("button", { name: "继续" }));
    expect(await screen.findByText("请输入 6 位动态验证码")).toBeVisible();
  });

  it("switches from TOTP to a Recovery Code only on the challenge step", async () => {
    server.use(
      http.post("/api/v1/auth/login", () =>
        HttpResponse.json({ status: "second_factor_required", code: "second_factor_required" }, { status: 202 }),
      ),
    );
    renderPage();
    const user = userEvent.setup();
    await user.type(screen.getByTestId("username"), "admin");
    await user.type(screen.getByTestId("password"), "correct-horse-42");
    await user.click(screen.getByRole("button", { name: "登录" }));
    await user.click(await screen.findByTestId("factor-recovery"));
    expect(screen.getByTestId("recovery-code")).toBeVisible();
  });

  it("shows OIDC login only when Core reports a usable binding", async () => {
    server.use(
      http.get("/api/v1/auth/oidc/status", () =>
        HttpResponse.json({ available: true, bound: true, login_available: true }),
      ),
    );
    renderPage();
    expect(await screen.findByRole("button", { name: "使用 OpenID Connect 登录" })).toBeVisible();
  });

  it("returns to password entry and focuses username when the challenge expires", async () => {
    server.use(
      http.post("/api/v1/auth/login", () =>
        HttpResponse.json({ status: "second_factor_required", code: "second_factor_required" }, { status: 202 }),
      ),
      http.post("/api/v1/auth/login/second-factor", () =>
        HttpResponse.json({ error: "login challenge expired", code: "challenge_expired" }, { status: 401 }),
      ),
    );
    renderPage();
    const user = userEvent.setup();
    await user.type(screen.getByTestId("username"), "admin");
    await user.type(screen.getByTestId("password"), "correct-horse-42");
    await user.click(screen.getByRole("button", { name: "登录" }));
    await user.type(await screen.findByTestId("totp-code"), "123456");
    await user.click(screen.getByRole("button", { name: "继续" }));

    expect(await screen.findByText("登录验证已过期，请重新输入用户名和密码。")).toBeVisible();
    expect(screen.getByTestId("username")).toHaveFocus();
  });
});
