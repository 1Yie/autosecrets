import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";

export const handlers = [
	http.get("/api/v1/me", () =>
		HttpResponse.json({
			bootstrap_required: false,
			member: { id: "admin-1", username: "admin", role: "administrator" },
			csrf_token: "csrf-test-token",
			session_expires_at: "2026-08-12T12:00:00Z",
			idle_expires_at: "2026-08-12T12:30:00Z",
			totp_login_required: false,
			auth_method: "local",
		}),
	),
	http.get("/api/v1/health", () =>
		HttpResponse.json({
			status: "ok",
			service: "core",
			version: "884f0021718f592f0025d4afcf65c87aa2b1137a",
		}),
	),
	http.post("/api/v1/auth/login", () =>
		HttpResponse.json({
			csrf_token: "csrf-test-token",
			username: "admin",
			id: "admin-1",
			role: "administrator",
			expires_at: "2026-08-12T12:00:00Z",
		}),
	),
	http.post("/api/v1/bootstrap", () =>
		HttpResponse.json({
			id: "member-1",
			username: "admin",
			status: "active",
			csrf_token: "csrf-test-token",
			role: "administrator",
			expires_at: "2026-08-12T12:00:00Z",
		}),
	),
	http.get("/api/v1/auth/oidc/status", () =>
		HttpResponse.json({
			available: false,
			bound: false,
			login_available: false,
			oidc: { available: false, bound: false, login_available: false },
			oauth: { available: false, bound: false, login_available: false },
		}),
	),
	http.get("/api/v1/auth/security", () =>
		HttpResponse.json({
			totp_login_required: false,
			password_login_enabled: true,
			password_login_available: true,
			oidc: { available: false, bound: false },
			oauth: { available: false, bound: false },
		}),
	),
	http.post("/api/v1/auth/mfa-enrollment/verify", () =>
		HttpResponse.json({
			confirmation_token: "confirmation-token-1",
			recovery_codes: ["ABCD-EFGH-JKLM", "NOPQ-RSTU-VWXY"],
		}),
	),
	http.post("/api/v1/auth/mfa-enrollment/confirm", () =>
		HttpResponse.json({ id: "member-1", username: "admin", status: "active" }),
	),
	http.post("/api/v1/nodes/install-command", () =>
		HttpResponse.json({
			command:
				'curl -fsSL https://agent.example.com/agent/v1/install.sh | sudo bash -s -- --server https://agent.example.com --token one-time-token-abc --name "web-1"',
			expires_at: "2026-08-12T02:00:00Z",
		}),
	),
	http.post("/api/v1/nodes/:nodeId/install-command", () =>
		HttpResponse.json({
			command:
				'curl -fsSL https://agent.example.com/agent/v1/install.sh | sudo bash -s -- --server https://agent.example.com --token one-time-token-abc --name "web-1"',
			expires_at: "2026-08-12T02:00:00Z",
		}),
	),
	http.get("/api/v1/nodes", () =>
		HttpResponse.json({ items: [], next_cursor: "", total: 0 }),
	),
	http.post("/api/v1/nodes", async ({ request }) => {
		const body = (await request.json()) as {
			name?: string;
			bundle_dir?: string;
		};
		return HttpResponse.json(
			{
				id: "node-new",
				name: body.name ?? "node",
				serial: "",
				created_at: "2026-08-12T12:00:00Z",
				last_seen_at: null,
				desired_etag: "",
				observed_revision: "",
				last_result: "",
				state: "never_online",
				unassigned: true,
				poll_interval_seconds: 15,
				bundle_dir: body.bundle_dir ?? "",
				enrolled: false,
			},
			{ status: 201 },
		);
	}),
	http.patch("/api/v1/nodes/:nodeId", async ({ params, request }) => {
		const body = (await request.json()) as {
			name?: string;
			bundle_dir?: string;
			poll_interval_seconds?: number;
		};
		return HttpResponse.json({
			id: String(params.nodeId),
			name: body.name ?? "web-1",
			serial: "serial-1",
			created_at: "2026-08-12T12:00:00Z",
			last_seen_at: "2026-08-12T12:00:00Z",
			desired_etag: "",
			observed_revision: "",
			last_result: "",
			state: "healthy",
			unassigned: false,
			poll_interval_seconds: body.poll_interval_seconds ?? 15,
			bundle_dir: body.bundle_dir ?? "",
			enrolled: true,
		});
	}),
	http.delete(
		"/api/v1/nodes/:nodeId",
		() => new HttpResponse(null, { status: 204 }),
	),
	http.get("/api/v1/node-groups", () =>
		HttpResponse.json({ items: [], next_cursor: "", total: 0 }),
	),
	http.get("/api/v1/assignments", () =>
		HttpResponse.json({ items: [], next_cursor: "", total: 0 }),
	),
	http.post("/api/v1/assignments/:assignmentId/unassign", () =>
		HttpResponse.json(
			{
				id: "asg-1",
				status: "removing",
				tasks: [],
			},
			{ status: 202 },
		),
	),
	http.get("/api/v1/applications", () =>
		HttpResponse.json({
			items: [
				{ id: "app-1", name: "payments", created_at: "2026-08-12T12:00:00Z" },
			],
			next_cursor: "",
			total: 1,
		}),
	),
	http.get("/api/v1/applications/:appId", ({ params }) =>
		HttpResponse.json({
			id: params.appId,
			name: "payments",
			environments: [
				{
					id: "env-1",
					name: "production",
					application_id: params.appId,
				},
				{
					id: "env-2",
					name: "staging",
					application_id: params.appId,
				},
			],
		}),
	),
	http.get("/api/v1/applications/:appId/environments/:envId/secrets", () =>
		HttpResponse.json([]),
	),
	http.get("/api/v1/search", () =>
		HttpResponse.json({
			results: [{ type: "application", id: "app-1", name: "payments" }],
		}),
	),
	http.get("/api/v1/overview", () =>
		HttpResponse.json({
			generated_at: "2026-08-12T12:00:00Z",
			counts: {
				applications: 1,
				environments: 1,
				secrets: 2,
				nodes: 3,
				node_groups: 1,
				assignments: 1,
				audit_events: 4,
			},
			attention: [{ kind: "failed_convergence", count: 1 }],
			recent_publishes: [],
			recent_audit: [],
		}),
	),
	http.get("/api/v1/audit-events", () =>
		HttpResponse.json({
			items: [
				{
					id: 1,
					actor: "member:admin",
					action: "application.create",
					resource: "app-1",
					result: "ok",
					correlation_id: "c1",
					created_at: "2026-08-12T12:00:00Z",
					actor_type: "member",
					actor_id: "admin",
					actor_display: "member:admin",
					resource_type: "application",
					resource_id: "app-1",
					resource_display: "app-1",
					outcome: "ok",
					operation_reason_category: "",
					operation_reason_explanation: "",
					operation_reason_external_ref: "",
				},
			],
			next_cursor: "",
			total: 1,
		}),
	),
	http.get("/api/v1/applications/:appId/environments/:envId/draft", () =>
		HttpResponse.json({
			version: 3,
			selections: [
				{
					secret_id: "secret-1",
					name: "db_pass",
					version_seq: 2,
					binding: { path: "db_pass", uid: 0, gid: 0, mode: "0400" },
				},
			],
		}),
	),
	http.get("/api/v1/applications/:appId/environments/:envId/revisions", () =>
		HttpResponse.json([
			{
				id: "rev-1",
				draft_version: 3,
				file_count: 1,
				created_by: "admin:admin",
				created_at: "2026-08-12T12:00:00Z",
				operation_reason: {
					category: "maintenance",
					explanation: "rotate the database password",
				},
			},
		]),
	),
	http.post("/api/v1/applications/:appId/environments/:envId/publish", () =>
		HttpResponse.json(
			{
				id: "rev-2",
				draft_version: 3,
				file_count: 1,
				created_by: "admin:admin",
				created_at: "2026-08-12T12:01:00Z",
				operation_reason: {
					category: "maintenance",
					explanation: "rotate the database password",
				},
			},
			{ status: 201 },
		),
	),
	http.post("/api/v1/applications/:appId/environments/:envId/rollback", () =>
		HttpResponse.json(
			{
				id: "rev-3",
				draft_version: 1,
				file_count: 1,
				created_by: "admin:admin",
				created_at: "2026-08-12T12:02:00Z",
				operation_reason: {
					category: "incident_response",
					explanation: "restore the previous working state",
				},
			},
			{ status: 201 },
		),
	),
];

export const server = setupServer(...handlers);
