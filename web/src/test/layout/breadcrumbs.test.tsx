import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { http, HttpResponse } from "msw";
import { DashboardLayout } from "../../layout/dashboard-layout";
import { server } from "../server";

function renderLayoutAt(path: string) {
	const qc = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	return render(
		<QueryClientProvider client={qc}>
			<MemoryRouter initialEntries={[path]}>
				<Routes>
					<Route path="/dashboard/*" element={<DashboardLayout />} />
				</Routes>
			</MemoryRouter>
		</QueryClientProvider>,
	);
}

describe("DashboardLayout breadcrumbs", () => {
	it("shows the section label for a top-level page", async () => {
		renderLayoutAt("/dashboard/nodes");
		await screen.findByTestId("current-user");
		const nav = await screen.findByLabelText("面包屑");
		expect(nav).toHaveTextContent("节点");
	});

	it("shows 应用 / <name> on an application detail page", async () => {
		server.use(
			http.get("/api/v1/applications/:appId", ({ params }) =>
				HttpResponse.json({
					id: params.appId,
					name: "payments",
					environments: [],
				}),
			),
		);
		renderLayoutAt("/dashboard/apps/app-1");
		await screen.findByTestId("current-user");
		const nav = await screen.findByLabelText("面包屑");
		expect(nav).toHaveTextContent("应用");
		expect(await screen.findByText("payments")).toBeInTheDocument();
	});

	it("shows the settings section label", async () => {
		renderLayoutAt("/dashboard/settings");
		await screen.findByTestId("current-user");
		const nav = await screen.findByLabelText("面包屑");
		expect(nav).toHaveTextContent("设置");
	});

	it("renders nothing on an unknown dashboard path", async () => {
		renderLayoutAt("/dashboard/unknown");
		await screen.findByTestId("current-user");
		expect(screen.queryByLabelText("面包屑")).not.toBeInTheDocument();
	});

	it("renders nothing on the retired security path", async () => {
		renderLayoutAt("/dashboard/security");
		await screen.findByTestId("current-user");
		expect(screen.queryByLabelText("面包屑")).not.toBeInTheDocument();
	});
});
