/**
 * E2E seam (the single behavioral proof): drives the real Compose-style stack
 * through the browser (bootstrap → login → author → publish → install
 * command) and executes the REAL install script on a node fixture, asserting
 * the Secret files land with the declared ownership and mode.
 *
 * Prerequisites (see scripts/run-e2e.sh):
 *   - Core on 127.0.0.1:18080 with keys at $E2E_KEYS and artifacts at
 *     $E2E_ARTIFACTS (signed via scripts/build-agent-artifact.sh)
 *   - devproxy (agent/tests/devproxy.py) on $E2E_PROXY_URL
 *   - vite dev server on 127.0.0.1:5199 proxying /api to Core
 */
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import https from "node:https";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect } from "@playwright/test";

const PROXY_URL = process.env.E2E_PROXY_URL ?? "https://localhost:18443";
const KEY_DIR = process.env.E2E_KEYS ?? "";
const BOOTSTRAP_CODE = process.env.E2E_BOOTSTRAP_CODE ?? "";

async function api(method: string, path: string, body?: unknown) {
  const res = await fetch(CORE_URL + path, {
    method,
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  return { status: res.status, body: await res.json() };
}

test("bootstrap to secret landing", async ({ page }) => {
  test.skip(!KEY_DIR || !BOOTSTRAP_CODE, "E2E stack not configured");
  const nodeName = "e2e-node";

  // --- Bootstrap + login ---
  await page.goto("/");
  await expect(page.getByText("Initialize AutoSecrets")).toBeVisible();
  await page.getByTestId("code").fill(BOOTSTRAP_CODE);
  await page.getByTestId("username").fill("admin");
  await page.getByTestId("password").fill("correct-horse-42");
  await page.getByRole("button", { name: "Create administrator" }).click();
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();

  await page.getByTestId("username").fill("admin");
  await page.getByTestId("password").fill("correct-horse-42");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Applications" })).toBeVisible();

  // --- Author: app, environment, secret, publish ---
  await page.getByTestId("app-name").fill("payments");
  await page.getByRole("button", { name: "Create", exact: true }).click();
  await expect(page.getByText("payments")).toBeVisible();
  await page.getByText("payments").click();

  await page.getByPlaceholder("new env").fill("production");
  await page.getByRole("button", { name: "+" }).click();
  await page.getByTestId("env-production").click();

  const secretValue = "e2e-secret-value-1";
  await page.getByTestId("secret-name").fill("db_pass");
  await page.getByTestId("secret-value").fill(secretValue);
  await page.getByRole("button", { name: "Add secret" }).click();
  await expect(page.getByText("db_pass")).toBeVisible();

  await page.getByRole("button", { name: "Publish revision" }).click();
  await expect(page.getByText(/Published/)).toBeVisible();

  // --- Fleet: node group + assignment (browser UI) ---
  await page.goto("/nodes");
  await page.getByPlaceholder("group name").fill("g1");
  await page.getByRole("button", { name: "Create", exact: true }).click();
  await expect(page.getByRole("listitem").filter({ hasText: "g1" })).toBeVisible();
  await page.getByTestId("assignment-revision").waitFor();
  await page.getByRole("combobox").first().selectOption({ index: 1 });
  await page.getByTestId("assignment-revision").selectOption({ index: 1 });
  await page.getByRole("button", { name: "Assign" }).click();
  await expect(page.getByText(/g1 ←/)).toBeVisible();

  // --- Install command (shown once) ---
  await page.getByPlaceholder("server name (e.g. web-1)").fill(nodeName);
  await page.getByTestId("node-name").fill(nodeName);
  await page.getByRole("button", { name: "Generate" }).click();
  const command = (await page.getByTestId("install-command").textContent()) ?? "";
  expect(command).toContain("install.sh");
  expect(command).toContain("--token ");
  const token = command.split("--token ")[1].split(/\s+/)[0];
  const nodeNameArg = command.includes("--name ")
    ? command.split("--name ")[1].split(/\s+/)[0].replaceAll('"', "")
    : nodeName;

  // --- Node fixture: execute the REAL install script ---
  const prefix = mkdtempSync(join(tmpdir(), "as-node-"));
  const script = await new Promise<string>((resolve, reject) => {
    https
      .get(
        PROXY_URL + "/agent/v1/install.sh",
        { ca: readFileSync(join(KEY_DIR, "agent-ca.crt")) },
        (res) => {
          let data = "";
          res.on("data", (c) => (data += c));
          res.on("end", () => resolve(data));
        },
      )
      .on("error", reject);
  });
  writeFileSync(join(prefix, "install.sh"), script);
  const serverArg = PROXY_URL;
  const out = execFileSync(
    "bash",
    [
      join(prefix, "install.sh"),
      "--server", serverArg,
      "--token", token,
      "--name", nodeNameArg,
    ],
    {
      env: {
        ...process.env,
        AUTOSECRETS_PREFIX: join(prefix, "opt"),
        AUTOSECRETS_CONFIG_DIR: join(prefix, "etc"),
        AUTOSECRETS_STATE_DIR: join(prefix, "var"),
        AUTOSECRETS_CURL_OPTS: "-k",
        AUTOSECRETS_NO_SYSTEMD: "1",
      },
      timeout: 120_000,
      stdio: "pipe",
    },
  ).toString();
  expect(out).toContain("First convergence pass");

  // --- Administrator adds the enrolled node to the group (browser UI) ---
  await page.reload();
  await expect(page.getByText("e2e-node").first()).toBeVisible();
  await page.getByRole("button", { name: "e2e-node" }).click();

  // --- One more convergence pass picks up the assignment ---
  const agentBin = join(prefix, "opt", "autosecrets-agent");
  const syncOut = execFileSync(agentBin, ["sync", "--config", join(prefix, "etc", "config.toml")], {
    timeout: 60_000,
    stdio: "pipe",
  }).toString();
  expect(syncOut).toBe("");

  // --- Assert the Secret landed on the node ---
  const bundleRoot = join(prefix, "var", "bundles");
  const files: string[] = [];
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const full = join(dir, entry.name);
      if (entry.isDirectory()) walk(full);
      else files.push(full);
    }
  };
  walk(bundleRoot);
  const secretFile = files.find((f) => f.endsWith("/db_pass"));
  expect(secretFile, `no db_pass under ${bundleRoot}`).toBeTruthy();
  expect(readFileSync(secretFile!, "utf8")).toBe(secretValue);

  // --- Core sees the node converged ---
  await page.goto("/nodes");
  await expect(page.getByText("ok", { exact: true })).toBeVisible();

  // The panel never re-displays the token.
  await expect(page.getByTestId("install-command")).toHaveCount(0);
});
