import { describe, expect, it } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useInstallCommand } from "../../../hooks/fleet/use-install-command";

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("useInstallCommand", () => {
  it("returns the one-time install command", async () => {
    const { result } = renderHook(() => useInstallCommand("node-1"), {
      wrapper,
    });
    result.current.mutate({});
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.command).toContain("install.sh");
    expect(result.current.data?.command).toContain(
      "--token one-time-token-abc",
    );
    expect(result.current.data?.command).toContain('--name "web-1"');
  });
});
