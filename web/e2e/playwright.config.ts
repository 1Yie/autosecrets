import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  testMatch: "**/*.e2e.ts",
  timeout: 360_000,
  retries: 0,
  workers: 1,
  use: {
    actionTimeout: 15_000,
    baseURL: "http://127.0.0.1:5199",
    trace: "retain-on-failure",
  },
});
