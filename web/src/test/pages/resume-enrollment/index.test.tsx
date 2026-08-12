import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { ResumeEnrollmentPage } from "../../../pages/resume-enrollment";

describe("ResumeEnrollmentPage", () => {
  it("resumes the interrupted Bootstrap with password and completes MFA", async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={qc}>
        <ResumeEnrollmentPage />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    const button = screen.getByRole("button", { name: "继续注册" });
    expect(button).toBeDisabled();

    await user.type(screen.getByTestId("username"), "admin");
    await user.type(screen.getByTestId("password"), "correct-horse-42");
    expect(button).toBeEnabled();
    await user.click(button);

    expect(await screen.findByText("验证动态验证码")).toBeVisible();
    await user.type(screen.getByTestId("totp-code"), "123456");
    await user.click(screen.getByRole("button", { name: "验证" }));
    expect(await screen.findByText("保存恢复码")).toBeVisible();
    await user.click(screen.getByTestId("recovery-ack"));
    await user.click(screen.getByRole("button", { name: "完成注册" }));
    expect(await screen.findByText("完成首次管理员注册")).toBeVisible();
  });
});
