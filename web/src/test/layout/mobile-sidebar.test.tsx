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

describe("DashboardLayout mobile sidebar", () => {
	it("opens navigation from the header trigger and closes after a link is chosen", async () => {
		const user = userEvent.setup();
		renderLayout();
		await screen.findByTestId("current-user");

		const trigger = screen.getByRole("button", { name: "打开侧边栏" });
		expect(trigger).toBeInTheDocument();

		await user.click(trigger);
		const dialog = await screen.findByRole("dialog");
		expect(dialog).toHaveTextContent("节点");

		await user.click(screen.getByRole("link", { name: "节点" }));
		await waitFor(() => {
			expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
		});
	});
});
