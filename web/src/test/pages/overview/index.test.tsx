import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { OverviewPage } from "../../../pages/overview";

describe("OverviewPage", () => {
  it("renders counts, attention items, and generated time", async () => {
    const qc = new QueryClient();
    render(
      <QueryClientProvider client={qc}>
        <OverviewPage />
      </QueryClientProvider>,
    );
    expect(await screen.findByText("概览")).toBeVisible();
    expect(screen.getByText("应用")).toBeVisible();
    expect(screen.getByText("收敛失败")).toBeVisible();
    expect(screen.getByTestId("generated-at")).toBeVisible();
  });
});
