import { describe, expect, it } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { server } from "../../server";
import { useLogin } from "../../../hooks/auth/use-login";
import { useSessionStore } from "../../../stores/session-store";

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("useLogin", () => {
  it("stores the CSRF token from the login response", async () => {
    server.use(
      http.post("/api/v1/auth/login", () =>
        HttpResponse.json({ csrf_token: "csrf-from-login", username: "admin", id: "a1" }),
      ),
    );
    useSessionStore.getState().clearSession();
    const { result } = renderHook(() => useLogin(), { wrapper });

    result.current.mutate({ username: "admin", password: "correct-horse-42" });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(useSessionStore.getState().csrfToken).toBe("csrf-from-login");
  });
});
