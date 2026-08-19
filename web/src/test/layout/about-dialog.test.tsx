import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { DashboardLayout } from "../../layout/dashboard-layout";
import { SecurityPage } from "../../pages/security";
import { ToastProvider } from "../../components/ui/toast";

function renderLayout() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={qc}>
      <ToastProvider position="top-center">
        <MemoryRouter initialEntries={["/dashboard/overview"]}>
          <Routes>
            <Route path="/dashboard" element={<DashboardLayout />}>
              <Route path="overview" element={<div>overview</div>} />
              <Route path="settings" element={<SecurityPage />} />
            </Route>
          </Routes>
        </MemoryRouter>
      </ToastProvider>
    </QueryClientProvider>,
  );
}

describe("DashboardLayout about settings", () => {
  it("opens about from settings instead of the user menu", async () => {
    const user = userEvent.setup();
    renderLayout();
    await screen.findByTestId("current-user");

    await user.click(screen.getByTestId("current-user"));
    expect(screen.queryByTestId("about")).toBeNull();
    await user.click(await screen.findByTestId("account-security"));

    expect(await screen.findByRole("heading", { name: "设置" })).toBeVisible();
    await user.click(screen.getByRole("tab", { name: "关于" }));
    expect(
      await screen.findByRole("heading", { name: "关于 AutoSecrets" }),
    ).toBeVisible();
    expect(await screen.findByText("0.0.0")).toBeVisible();
    expect(screen.getByText("core 服务")).toBeVisible();
    expect(
      screen.getByRole("heading", { name: "GitHub 贡献者" }),
    ).toBeVisible();
    expect(screen.getByText("1Yie")).toBeVisible();
    expect(screen.getByText("作者")).toBeVisible();
    expect(screen.getByRole("heading", { name: "感谢" })).toBeVisible();
    expect(screen.getByText("kmou424")).toBeVisible();
    expect(screen.getByText("原始作者")).toBeVisible();
    expect(screen.getByText("github.com/1Yie/autosecrets")).toBeVisible();
    expect(document.querySelectorAll('[data-slot="avatar"]')).toHaveLength(2);
    expect(screen.queryByRole("button", { name: "关闭" })).toBeNull();
  });
});
