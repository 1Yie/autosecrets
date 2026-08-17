import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { DashboardLayout } from "../../layout/dashboard-layout";

function renderLayout() {
	const qc = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	return render(
		<QueryClientProvider client={qc}>
			<MemoryRouter initialEntries={["/dashboard/overview"]}>
				<Routes>
					<Route path="/dashboard/*" element={<DashboardLayout />} />
				</Routes>
			</MemoryRouter>
		</QueryClientProvider>,
	);
}

describe("DashboardLayout about dialog", () => {
	it("opens the about dialog from the sidebar user menu", async () => {
		const user = userEvent.setup();
		renderLayout();
		await screen.findByTestId("current-user");

		await user.click(screen.getByTestId("current-user"));
		await user.click(await screen.findByTestId("about"));

		expect(
			await screen.findByRole("heading", { name: "关于 AutoSecrets" }),
		).toBeVisible();
		// 版本来自 /api/v1/health 的 commit hash
		expect(await screen.findByText("884f002")).toBeVisible();
		expect(screen.getByText("core 服务")).toBeVisible();
		expect(screen.getByRole("heading", { name: "GitHub 贡献者" })).toBeVisible();
		expect(screen.getByText("1Yie")).toBeVisible();
		expect(screen.getByText("作者")).toBeVisible();
		expect(screen.getByRole("heading", { name: "感谢" })).toBeVisible();
		expect(screen.getByText("kmou424")).toBeVisible();
		expect(screen.getByText("原始作者")).toBeVisible();
		expect(screen.getByText("github.com/1Yie/autosecrets")).toBeVisible();
		expect(document.querySelectorAll('[data-slot="avatar"]')).toHaveLength(2);

		await user.click(screen.getByRole("button", { name: "关闭" }));
		await waitFor(() => {
			expect(
				screen.queryByRole("heading", { name: "关于 AutoSecrets" }),
			).not.toBeInTheDocument();
		});
	});
});
