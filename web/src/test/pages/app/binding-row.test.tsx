import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { SecretTableRow } from "../../../pages/app/binding-row";
import type { SecretRow } from "../../../hooks/applications/use-secrets";

const secret: SecretRow = {
  id: "s-1",
  name: "DB_PASSWORD",
  binding: { path: "db_password", uid: 0, gid: 0, mode: "0600" },
  latest_version: 3,
  selected_version: 2,
};

const nested: SecretRow = {
  id: "s-2",
  name: "API_TOKEN",
  binding: { path: "A/1", uid: 0, gid: 0, mode: "0600" },
  latest_version: 1,
  selected_version: 1,
};

function renderRow(row: SecretRow) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <table>
        <tbody>
          <SecretTableRow secret={row} appId="app-1" envId="env-1" />
        </tbody>
      </table>
    </QueryClientProvider>,
  );
}

describe("SecretTableRow", () => {
  it("enables 保存 after editing the binding path", async () => {
    renderRow(secret);
    const user = userEvent.setup();
    const save = screen.getByRole("button", { name: "保存" });
    expect(save).toBeDisabled();

    const input = screen.getByTestId("binding-DB_PASSWORD");
    await user.clear(input);
    await user.type(input, "db_password_v2");
    expect(save).toBeEnabled();
  });

  it("renders a nested path as directory + filename segments", () => {
    renderRow(nested);
    const dir = screen.getByTestId("binding-API_TOKEN-dir-0") as HTMLInputElement;
    const file = screen.getByTestId("binding-API_TOKEN") as HTMLInputElement;
    expect(dir.value).toBe("A");
    expect(file.value).toBe("1");
  });

  it("keeps the permission mode collapsed until the row is expanded", async () => {
    renderRow(secret);
    expect(screen.queryByTestId("mode-DB_PASSWORD")).toBeNull();
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "高级设置" }));
    expect(screen.getByTestId("mode-DB_PASSWORD")).toBeInTheDocument();
  });
});
