import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";

export const handlers = [
  http.get("/api/v1/me", () =>
    HttpResponse.json({
      bootstrap_required: false,
      organization: { display_name: "Test Organization" },
      member: { id: "admin-1", username: "admin", role: "administrator" },
      csrf_token: "csrf-test-token",
      session_expires_at: "2026-08-12T12:00:00Z",
      idle_expires_at: "2026-08-12T12:30:00Z",
      step_up: true,
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
      status: "pending_mfa",
      enrollment_token: "enrollment-token-1",
      totp_uri: "otpauth://totp/AutoSecrets:admin?secret=JBSWY3DPEHPK3PXP",
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
  http.post("/api/v1/auth/step-up", () =>
    HttpResponse.json({ expires_at: "2026-08-12T12:05:00Z" }),
  ),
  http.post("/api/v1/nodes/install-command", () =>
    HttpResponse.json({
      command:
        'curl -fsSL https://agent.example.com/agent/v1/install.sh | sudo bash -s -- --server https://agent.example.com --token one-time-token-abc --name "web-1"',
      expires_at: "2026-08-12T02:00:00Z",
    }),
  ),
  http.get("/api/v1/nodes", () => HttpResponse.json([])),
  http.get("/api/v1/node-groups", () => HttpResponse.json([])),
  http.get("/api/v1/assignments", () => HttpResponse.json([])),
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
  http.get("/api/v1/audit-events", () => HttpResponse.json([])),
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
        operation_reason: { category: "maintenance", explanation: "rotate the database password" },
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
        operation_reason: { category: "maintenance", explanation: "rotate the database password" },
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
        operation_reason: { category: "incident_response", explanation: "restore the previous working state" },
      },
      { status: 201 },
    ),
  ),
];

export const server = setupServer(...handlers);
