import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { describe, expect, it, vi } from "vitest";
import { NodesPage } from "../../../pages/nodes";
import { ToastProvider } from "../../../components/ui/toast";
import { server } from "../../server";

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

async function openInstallDialog(user: ReturnType<typeof userEvent.setup>) {
	await user.click(screen.getByRole("button", { name: "添加服务器" }));
	await screen.findByTestId("node-name");
}

describe("NodesPage install command", () => {
	it("renders the one-time install command with the token, once", async () => {
		renderPage();
		const user = userEvent.setup();
		await openInstallDialog(user);
		await user.type(screen.getByTestId("node-name"), "web-1");
		await user.click(screen.getByRole("button", { name: "生成" }));

		const command = await screen.findByTestId("install-command");
		expect(command.textContent).toContain("--token one-time-token-abc");
		expect(command.textContent).toContain("--server https://agent.example.com");
	});

	it("accepts an explicit Materialized Bundle deployment path", async () => {
		let requestBody: unknown;
		server.use(
			http.post("/api/v1/nodes/install-command", async ({ request }) => {
				requestBody = await request.json();
				return HttpResponse.json({
					command: "curl install.sh --token one-time-token-abc",
					expires_at: "2026-08-12T02:00:00Z",
				});
			}),
		);
		renderPage();
		const user = userEvent.setup();
		await openInstallDialog(user);
		await user.clear(screen.getByTestId("node-name"));
		await user.type(screen.getByTestId("node-name"), "web-1");
		await user.clear(screen.getByTestId("node-bundle-dir"));
		await user.type(screen.getByTestId("node-bundle-dir"), "/srv/autosecrets");
		await user.click(screen.getByRole("button", { name: "生成" }));
		await screen.findByTestId("install-command");
		expect(requestBody).toEqual({
			name: "web-1",
			bundle_dir: "/srv/autosecrets",
		});
	});

	it("shows the command once and never re-renders the token after dismissal", async () => {
		renderPage();
		const user = userEvent.setup();
		await openInstallDialog(user);
		await user.type(screen.getByTestId("node-name"), "web-1");
		await user.click(screen.getByRole("button", { name: "生成" }));
		const command = await screen.findByTestId("install-command");
		expect(screen.getAllByTestId("install-command")).toHaveLength(1);
		expect(command.textContent).toContain("one-time-token-abc");
	});

	it("offers a copy affordance for the command", async () => {
		const writeText = vi.fn().mockResolvedValue(undefined);
		renderPage();
		const user = userEvent.setup();
		vi.stubGlobal("navigator", { ...navigator, clipboard: { writeText } });
		await openInstallDialog(user);
		await user.type(screen.getByTestId("node-name"), "web-1");
		await user.click(screen.getByRole("button", { name: "生成" }));
		await screen.findByTestId("install-command");
		fireEvent.click(screen.getByRole("button", { name: "复制命令" }));
		await waitFor(() => expect(writeText).toHaveBeenCalled());
		expect(writeText.mock.calls[0][0]).toContain("one-time-token-abc");
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
	const node = {
		id: "node-1",
		name: "web-1",
		serial: "serial-1",
		created_at: "2026-08-12T12:00:00Z",
		last_seen_at: "2026-08-12T12:00:00Z",
		desired_etag: '"etag"',
		observed_revision: "rev-1",
		last_result: "ok",
		state: "healthy",
		unassigned: false,
		poll_interval_seconds: 15,
	};

	it("renders the current interval label and PATCHes a preset only on save", async () => {
		let patchBody: unknown;
		let currentInterval = 15;
		server.use(
			http.get("/api/v1/nodes", () =>
				HttpResponse.json({
					items: [{ ...node, poll_interval_seconds: currentInterval }],
					next_cursor: "",
					total: 1,
				}),
			),
			http.patch("/api/v1/nodes/:nodeId", async ({ request }) => {
				patchBody = await request.json();
				currentInterval = (patchBody as { poll_interval_seconds: number })
					.poll_interval_seconds;
				return HttpResponse.json({
					id: "node-1",
					poll_interval_seconds: currentInterval,
				});
			}),
		);
		renderPage();
		const user = userEvent.setup();

		expect(await screen.findByText("15秒")).toBeInTheDocument();
		const save = screen.getByTestId("poll-interval-save-node-1");
		expect(save).toBeDisabled();

		await user.click(screen.getByTestId("poll-interval-node-1"));
		await user.click(await screen.findByRole("option", { name: "1分钟" }));
		expect(patchBody).toBeUndefined();
		expect(save).toBeEnabled();

		await user.click(save);
		await waitFor(() => expect(patchBody).toEqual({ poll_interval_seconds: 60 }));
		expect(await screen.findByText("轮询间隔已更新")).toBeVisible();
	});

	it("enables save again after the interval changes", async () => {
		let currentInterval = 15;
		server.use(
			http.get("/api/v1/nodes", () =>
				HttpResponse.json({
					items: [{ ...node, poll_interval_seconds: currentInterval }],
					next_cursor: "",
					total: 1,
				}),
			),
			http.patch("/api/v1/nodes/:nodeId", async ({ request }) => {
				const body = (await request.json()) as { poll_interval_seconds: number };
				currentInterval = body.poll_interval_seconds;
				return HttpResponse.json({
					id: "node-1",
					poll_interval_seconds: currentInterval,
				});
			}),
		);
		renderPage();
		const user = userEvent.setup();
		await screen.findByText("15秒");

		await user.click(screen.getByTestId("poll-interval-node-1"));
		await user.click(await screen.findByRole("option", { name: "30秒" }));
		await user.click(screen.getByTestId("poll-interval-save-node-1"));
		expect(await screen.findByText("轮询间隔已更新")).toBeVisible();

		await user.click(screen.getByTestId("poll-interval-node-1"));
		await user.click(await screen.findByRole("option", { name: "1分钟" }));
		expect(screen.getByTestId("poll-interval-save-node-1")).toBeEnabled();
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
		await user.click(screen.getByTestId("poll-interval-node-1"));
		await screen.findByRole("option", { name: "45秒" });
	});
});
