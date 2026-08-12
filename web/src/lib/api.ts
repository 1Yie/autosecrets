// Transport layer: the only place that touches fetch. Components and Hooks
// go through TanStack Query wrappers; the CSRF token lives in the Zustand
// session store and is attached to every mutation.
import { useSessionStore } from "../stores/session-store";

export class ApiError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export async function api<T = unknown>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (method !== "GET") {
    const csrfToken = useSessionStore.getState().csrfToken;
    if (csrfToken) headers["X-CSRF-Token"] = csrfToken;
  }
  const res = await fetch(path, {
    method,
    headers,
    credentials: "same-origin",
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  let data: unknown;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = text;
  }
  if (!res.ok) {
    const body = data && typeof data === "object" ? (data as { error?: string; code?: string }) : null;
    const message = body?.error ?? `HTTP ${res.status}`;
    const code = body?.code ?? "unknown";
    throw new ApiError(res.status, code, message);
  }
  return data as T;
}

export const apiGet = <T>(path: string) => api<T>("GET", path);
export const apiPost = <T>(path: string, body?: unknown) => api<T>("POST", path, body);
export const apiPut = <T>(path: string, body?: unknown) => api<T>("PUT", path, body);
export const apiDelete = <T>(path: string) => api<T>("DELETE", path);
