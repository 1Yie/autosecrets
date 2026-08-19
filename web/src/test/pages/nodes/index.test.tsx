import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { describe, expect, it, vi } from "vitest";
import { NodesPage } from "../../../pages/nodes";
import { ToastProvider } from "../../../components/ui/toast";
import { server } from "../../server";

const pendingNode = {
	id: "node-1",
	name: "web-1",
	serial: "",
	created_at: "2026-08-12T12:00:00Z",
	last_seen_at: null,
	desired_etag: "",
	observed_revision: "",
	last_result: "",
	state: "never_online",
	unassigned: true,
	poll_interval_seconds: 15,
	bundle_dir: "~/.autosecrets",
	enrolled: false,
};

const enrolledNode = {
	...pendingNode,
	serial: "serial-1",
	last_seen_at: "2026-08-12T12:00:00Z",
	observed_revision: "rev-1",
	last_result: "ok",
	state: "healthy",
	unassigned: false,
	enrolled: true,
};

function renderPage() {
	const qc = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	return render(
		<ToastProvider position="top-center">
			<QueryClientProvider client={qc}>
				<NodesPage />
			</QueryClientProvider>
		</ToastProvider>,
	);
}

function stubNodes(items: Array<typeof pendingNode | typeof enrolledNode>) {
	server.use(
		http.get("/api/v1/nodes", () =>
			HttpResponse.json({ items, next_cursor: "", total: items.length }),
		),
	);
}

describe("NodesPage add server", () => {
	it("registers a server without showing an install command", async () => {
		let items: Array<typeof pendingNode> = [];
		let created: unknown;
		server.use(
			http.get("/api/v1/nodes", () =>
				HttpResponse.json({ items, next_cursor: "", total: items.length }),
			),
			http.post("/api/v1/nodes", async ({ request }) => {
				created = await request.json();
				items = [pendingNode];
				return HttpResponse.json(pendingNode, { status: 201 });
			}),
		);
		renderPage();
		const user = userEvent.setup();
		await user.click(screen.getByRole("button", { name: "添加服务器" }));
		await user.type(await screen.findByTestId("node-name"), "web-1");
		await user.click(screen.getByRole("button", { name: "添加" }));
		expect(await screen.findByText("服务器已添加")).toBeVisible();
		expect(created).toEqual({
			name: "web-1",
			bundle_dir: "~/.autosecrets",
		});
		expect(screen.queryByTestId("install-command")).not.toBeInTheDocument();
		expect(await screen.findByText("web-1")).toBeInTheDocument();
	});
});

describe("NodesPage install command", () => {
	it("renders the one-time install command from an existing server", async () => {
		stubNodes([pendingNode]);
		renderPage();
		const user = userEvent.setup();
		await user.click(await screen.findByRole("button", { name: "生成连接" }));

		const command = await screen.findByTestId("install-command");
		expect(command.textContent).toContain("--token one-time-token-abc");
		expect(command.textContent).toContain("--server https://agent.example.com");
	});

	it("sends the stored bundle directory when generating a connection", async () => {
		let requestBody: unknown;
		stubNodes([{ ...pendingNode, bundle_dir: "/srv/autosecrets" }]);
		server.use(
			http.post("/api/v1/nodes/:nodeId/install-command", async ({ request }) => {
				requestBody = await request.json();
				return HttpResponse.json({
					command: "curl install.sh --token one-time-token-abc",
					expires_at: "2026-08-12T02:00:00Z",
				});
			}),
		);
		renderPage();
		const user = userEvent.setup();
		await user.click(await screen.findByRole("button", { name: "生成连接" }));
		await screen.findByTestId("install-command");
		expect(requestBody).toEqual({ bundle_dir: "/srv/autosecrets" });
	});

	it("shows the command once and never re-renders the token after dismissal", async () => {
		stubNodes([pendingNode]);
		renderPage();
		const user = userEvent.setup();
		await user.click(await screen.findByRole("button", { name: "生成连接" }));
		const command = await screen.findByTestId("install-command");
		expect(screen.getAllByTestId("install-command")).toHaveLength(1);
		expect(command.textContent).toContain("one-time-token-abc");
	});

	it("offers a copy affordance for the command", async () => {
		const writeText = vi.fn().mockResolvedValue(undefined);
		stubNodes([pendingNode]);
		renderPage();
		const user = userEvent.setup();
		vi.stubGlobal("navigator", { ...navigator, clipboard: { writeText } });
		await user.click(await screen.findByRole("button", { name: "生成连接" }));
		await screen.findByTestId("install-command");
		fireEvent.click(screen.getByRole("button", { name: "复制命令" }));
		await waitFor(() => expect(writeText).toHaveBeenCalled());
		expect(writeText.mock.calls[0][0]).toContain("one-time-token-abc");
	});
});

describe("NodesPage edit and delete", () => {
	it("renames a server", async () => {
		let patchBody: unknown;
		stubNodes([pendingNode]);
		server.use(
			http.patch("/api/v1/nodes/:nodeId", async ({ request }) => {
				patchBody = await request.json();
				return HttpResponse.json({
					...pendingNode,
					name: "ingstar",
				});
			}),
		);
		renderPage();
		const user = userEvent.setup();
		await user.click(await screen.findByRole("button", { name: "修改" }));
		const name = await screen.findByTestId("edit-node-name-node-1");
		await user.clear(name);
		await user.type(name, "ingstar");
		await user.click(screen.getByRole("button", { name: "保存" }));
		await waitFor(() =>
			expect(patchBody).toEqual({
				name: "ingstar",
				bundle_dir: "~/.autosecrets",
				poll_interval_seconds: 15,
			}),
		);
		expect(await screen.findByText("服务器已更新")).toBeVisible();
	});

	it("deletes a server after confirmation", async () => {
		let deleted = false;
		let items = [pendingNode];
		server.use(
			http.get("/api/v1/nodes", () =>
				HttpResponse.json({ items, next_cursor: "", total: items.length }),
			),
			http.delete("/api/v1/nodes/:nodeId", () => {
				deleted = true;
				items = [];
				return new HttpResponse(null, { status: 204 });
			}),
		);
		renderPage();
		const user = userEvent.setup();
		await user.click(await screen.findByRole("button", { name: "删除" }));
		await user.click(screen.getByRole("button", { name: "删除" }));
		await waitFor(() => expect(deleted).toBe(true));
		expect(await screen.findByText("服务器已删除")).toBeVisible();
	});
});

describe("NodesPage tab actions", () => {
	it("swaps the tab-row action when switching tabs", async () => {
		renderPage();
		const user = userEvent.setup();
		expect(
			screen.getByRole("button", { name: "添加服务器" }),
		).toBeInTheDocument();
		expect(
			screen.queryByRole("button", { name: "新建节点组" }),
		).not.toBeInTheDocument();

		await user.click(screen.getByRole("tab", { name: "节点组" }));
		expect(
			screen.getByRole("button", { name: "新建节点组" }),
		).toBeInTheDocument();
		expect(
			screen.queryByRole("button", { name: "添加服务器" }),
		).not.toBeInTheDocument();

		await user.click(screen.getByRole("button", { name: "新建节点组" }));
		await screen.findByTestId("node-group-name");
	});
});

describe("NodesPage poll interval", () => {
	const node = enrolledNode;

	it("shows the interval as read-only in the table", async () => {
		stubNodes([node]);
		renderPage();
		expect(await screen.findByText("15秒")).toBeInTheDocument();
		expect(
			screen.queryByTestId("poll-interval-save-node-1"),
		).not.toBeInTheDocument();
	});

	it("PATCHes a preset from the edit dialog", async () => {
		let patchBody: unknown;
		server.use(
			http.get("/api/v1/nodes", () =>
				HttpResponse.json({
					items: [{ ...node, poll_interval_seconds: 15 }],
					next_cursor: "",
					total: 1,
				}),
			),
			http.patch("/api/v1/nodes/:nodeId", async ({ request }) => {
				patchBody = await request.json();
				return HttpResponse.json({
					...node,
					...(patchBody as object),
				});
			}),
		);
		renderPage();
		const user = userEvent.setup();
		await user.click(await screen.findByRole("button", { name: "修改" }));
		await user.click(await screen.findByTestId("edit-poll-interval-node-1"));
		await user.click(await screen.findByRole("option", { name: "1分钟" }));
		expect(patchBody).toBeUndefined();
		await user.click(screen.getByRole("button", { name: "保存" }));
		await waitFor(() =>
			expect(patchBody).toEqual({
				name: node.name,
				bundle_dir: node.bundle_dir,
				poll_interval_seconds: 60,
			}),
		);
		expect(await screen.findByText("服务器已更新")).toBeVisible();
	});

	it("offers the current interval when it is not a preset", async () => {
		server.use(
			http.get("/api/v1/nodes", () =>
				HttpResponse.json({
					items: [{ ...node, poll_interval_seconds: 45 }],
					next_cursor: "",
					total: 1,
				}),
			),
		);
		renderPage();
		const user = userEvent.setup();

		expect(await screen.findByText("45秒")).toBeInTheDocument();
		await user.click(await screen.findByRole("button", { name: "修改" }));
		await user.click(await screen.findByTestId("edit-poll-interval-node-1"));
		await screen.findByRole("option", { name: "45秒" });
	});
});
