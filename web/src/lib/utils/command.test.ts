import { describe, expect, it } from "vitest";
import { parseInstallCommand } from "./command";

describe("parseInstallCommand", () => {
  it("extracts server, token, and quoted name", () => {
    const parsed = parseInstallCommand(
      'curl -fsSL https://agent.test/agent/v1/install.sh | sudo bash -s -- --server https://agent.test --token tok-123 --name "web-1"',
    );
    expect(parsed).toEqual({
      server: "https://agent.test",
      token: "tok-123",
      name: "web-1",
    });
  });

  it("returns empty strings for missing flags", () => {
    expect(parseInstallCommand("curl something")).toEqual({
      server: "",
      token: "",
      name: "",
    });
  });
});
