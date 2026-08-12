import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { BootstrapPage } from "../../../pages/bootstrap";

describe("BootstrapPage", () => {
  it("requires code, organization name, username, and a 12+ char password", async () => {
    const qc = new QueryClient();
    render(
      <QueryClientProvider client={qc}>
        <BootstrapPage />
      </QueryClientProvider>,
    );
    const button = screen.getByRole("button", { name: "创建管理员" });
    expect(button).toBeDisabled();

    const user = userEvent.setup();
    await user.type(screen.getByTestId("code"), "code-123");
    await user.type(screen.getByTestId("organization-name"), "Acme");
    await user.type(screen.getByTestId("username"), "admin");
    await user.type(screen.getByTestId("password"), "short");
    expect(button).toBeDisabled();

    await user.clear(screen.getByTestId("password"));
    await user.type(screen.getByTestId("password"), "correct-horse-42");
    expect(button).toBeEnabled();
  });

  it("walks the MFA wizard after enrollment", async () => {
    const qc = new QueryClient();
    render(
      <QueryClientProvider client={qc}>
        <BootstrapPage />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.type(screen.getByTestId("code"), "code-123");
    await user.type(screen.getByTestId("organization-name"), "Acme");
    await user.type(screen.getByTestId("username"), "admin");
    await user.type(screen.getByTestId("password"), "correct-horse-42");
    await user.click(screen.getByRole("button", { name: "创建管理员" }));

    expect(await screen.findByText("验证动态验证码")).toBeVisible();
    await user.type(screen.getByTestId("totp-code"), "123456");
    await user.click(screen.getByRole("button", { name: "验证" }));

    expect(await screen.findByText("保存恢复码")).toBeVisible();
    expect(screen.getByTestId("recovery-codes").textContent).toContain("ABCD-EFGH-JKLM");
    await user.click(screen.getByTestId("recovery-ack"));
    await user.click(screen.getByRole("button", { name: "完成注册" }));

    expect(await screen.findByText("注册完成")).toBeVisible();
  });
});
