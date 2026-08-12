import { describe, expect, it, beforeEach } from "vitest";
import { useSessionStore } from "../../stores/session-store";

describe("session store", () => {
  beforeEach(() => useSessionStore.getState().clearSession());

  it("stores and clears the CSRF token", () => {
    useSessionStore.getState().setCsrfToken("tok-1");
    expect(useSessionStore.getState().csrfToken).toBe("tok-1");
    useSessionStore.getState().clearSession();
    expect(useSessionStore.getState().csrfToken).toBe("");
  });
});
