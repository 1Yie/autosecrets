import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { AppPage } from "../../../pages/app";
import { ToastProvider } from "../../../components/ui/toast";
import { server } from "../../server";

function renderPage() {
	const qc = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	return render(
		<ToastProvider position="top-center">
			<QueryClientProvider client={qc}>
				<MemoryRouter initialEntries={["/dashboard/apps/app-1"]}>
					<Routes>
						<Route path="/dashboard/apps/:appId" element={<AppPage />} />
					</Routes>
				</MemoryRouter>
			</QueryClientProvider>
		</ToastProvider>,
	);
}

describe("AppPage environment switcher", () => {
	it("shows an empty state instead of a header-only secrets table", async () => {
		renderPage();
		expect(await screen.findByTestId("secrets-empty")).toBeVisible();
		expect(screen.getByText("还没有密钥")).toBeVisible();
		expect(
			screen.queryByRole("columnheader", { name: "名称" }),
		).not.toBeInTheDocument();
	});

	it("publishes the current environment from the page header", async () => {
		const user = userEvent.setup();
		renderPage();
		await screen.findByRole("button", { name: "发布" });
		await waitFor(() => {
			expect(screen.getByRole("button", { name: "发布" })).toBeEnabled();
		});
		await user.click(screen.getByRole("button", { name: "发布" }));
		expect(await screen.findByText("已发布，节点将自动同步")).toBeVisible();
	});

	it("toasts when the draft has nothing to publish", async () => {
		server.use(
			http.post("/api/v1/applications/:appId/environments/:envId/publish", () =>
				HttpResponse.json(
					{ error: "草稿没有需要发布的变更", code: "conflict" },
					{ status: 409 },
				),
			),
		);
		const user = userEvent.setup();
		renderPage();
		await waitFor(() => {
			expect(screen.getByRole("button", { name: "发布" })).toBeEnabled();
		});
		await user.click(screen.getByRole("button", { name: "发布" }));
		expect(await screen.findByText("草稿没有需要发布的变更")).toBeVisible();
	});

	it("hides delete behind the environment more menu", async () => {
		const user = userEvent.setup();
		renderPage();
		const production = await screen.findByTestId("env-production");
		expect(production).toHaveAttribute("aria-selected", "true");
		expect(screen.getByLabelText("production 更多操作")).toBeVisible();
		expect(screen.getByLabelText("staging 更多操作")).toBeVisible();
		expect(
			screen.queryByRole("button", { name: "删除环境" }),
		).not.toBeInTheDocument();

		await user.click(screen.getByTestId("env-staging"));
		expect(screen.getByTestId("env-staging")).toHaveAttribute(
			"aria-selected",
			"true",
		);
		await user.click(screen.getByLabelText("staging 更多操作"));
		expect(
			await screen.findByRole("menuitem", { name: "删除环境" }),
		).toBeVisible();
		await user.click(screen.getByRole("menuitem", { name: "删除环境" }));
		expect(
			screen.getByRole("heading", { name: "删除环境 staging？" }),
		).toBeVisible();
	});

	it("opens the more menu of a non-selected environment without switching", async () => {
		const user = userEvent.setup();
		renderPage();
		const production = await screen.findByTestId("env-production");
		expect(production).toHaveAttribute("aria-selected", "true");

		await user.click(screen.getByLabelText("staging 更多操作"));
		expect(
			await screen.findByRole("menuitem", { name: "删除环境" }),
		).toBeVisible();
		await user.click(screen.getByRole("menuitem", { name: "删除环境" }));
		expect(
			screen.getByRole("heading", { name: "删除环境 staging？" }),
		).toBeVisible();
		// 打开未激活环境的菜单不会切换选中状态
		expect(production).toHaveAttribute("aria-selected", "true");
	});
});
