import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { SecretTableRow } from "../../../pages/app/binding-row";
import { ToastProvider } from "../../../components/ui/toast";
import type { SecretRow } from "../../../hooks/applications/use-secrets";

const secret: SecretRow = {
	id: "s-1",
	name: "DB_PASSWORD",
	binding: { path: "db_password", uid: 0, gid: 0, mode: "0600" },
	latest_version: 3,
	selected_version: 2,
};

const nested: SecretRow = {
	id: "s-2",
	name: "API_TOKEN",
	binding: { path: "A/1", uid: 0, gid: 0, mode: "0600" },
	latest_version: 1,
	selected_version: 1,
};

function renderRow(row: SecretRow) {
	const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
	render(
		<ToastProvider position="top-center">
			<QueryClientProvider client={qc}>
				<table>
					<tbody>
						<SecretTableRow secret={row} appId="app-1" envId="env-1" />
					</tbody>
				</table>
			</QueryClientProvider>
		</ToastProvider>,
	);
}

async function openBindingEditor(
	user: ReturnType<typeof userEvent.setup>,
	name: string,
) {
	await user.click(screen.getByRole("button", { name: `${name} 更多操作` }));
	await user.click(await screen.findByRole("menuitem", { name: "编辑绑定" }));
}

describe("SecretTableRow", () => {
	it("shows the binding path as text and keeps save out of the table", () => {
		renderRow(secret);
		expect(screen.getByTestId("binding-path-DB_PASSWORD")).toHaveTextContent(
			"db_password",
		);
		expect(screen.getByText("0600（仅所有者可读写）")).toBeVisible();
		expect(screen.queryByRole("button", { name: "保存" })).toBeNull();
		expect(screen.getByRole("button", { name: "轮换" })).toBeVisible();
		expect(screen.getByTestId("version-DB_PASSWORD")).toHaveTextContent("v2");
		expect(screen.getByText("最新 v3")).toBeVisible();
	});

	it("edits a nested path in the binding dialog", async () => {
		renderRow(nested);
		const user = userEvent.setup();
		await openBindingEditor(user, "API_TOKEN");

		const dir = (await screen.findByTestId(
			"binding-API_TOKEN-dir-0",
		)) as HTMLInputElement;
		const file = screen.getByTestId("binding-API_TOKEN") as HTMLInputElement;
		expect(dir.value).toBe("A");
		expect(file.value).toBe("1");

		const save = screen.getByRole("button", { name: "保存" });
		expect(save).toBeDisabled();
		await user.clear(file);
		await user.type(file, "token");
		expect(save).toBeEnabled();
	});

	it("exposes the permission mode in the binding dialog", async () => {
		renderRow(secret);
		const user = userEvent.setup();
		expect(screen.queryByTestId("mode-DB_PASSWORD")).toBeNull();

		await openBindingEditor(user, "DB_PASSWORD");
		expect(screen.getByTestId("mode-DB_PASSWORD")).toBeInTheDocument();
		expect(screen.getByTestId("mode-DB_PASSWORD")).toHaveTextContent(
			"0600（仅所有者可读写）",
		);
		await user.click(screen.getByTestId("mode-DB_PASSWORD"));
		expect(
			await screen.findByRole("option", {
				name: "0644（所有者可读写，其他人可读）",
			}),
		).toBeInTheDocument();
	});
});
