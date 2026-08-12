import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";

export const handlers = [
  http.get("/api/v1/me", () =>
    HttpResponse.json({
      bootstrap_required: false,
      admin: { id: "admin-1", username: "admin" },
    }),
  ),
  http.post("/api/v1/auth/login", () =>
    HttpResponse.json({ csrf_token: "csrf-test-token", username: "admin", id: "admin-1" }),
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
];

export const server = setupServer(...handlers);
