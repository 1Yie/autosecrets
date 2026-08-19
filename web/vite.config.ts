import { readFileSync } from "node:fs";
import { defineConfig } from "vite";
import path from "path";
import react, { reactCompilerPreset } from "@vitejs/plugin-react";
import babel from "@rolldown/plugin-babel";
import tailwindcss from "@tailwindcss/vite";

function readPackageVersion(): string {
  try {
    const raw = readFileSync(path.resolve(__dirname, "package.json"), "utf8");
    const parsed: unknown = JSON.parse(raw);
    if (
      typeof parsed === "object" &&
      parsed !== null &&
      "version" in parsed &&
      typeof parsed.version === "string" &&
      parsed.version.trim() !== ""
    ) {
      return parsed.version;
    }
  } catch {
    // Fall back below when package.json is missing or not valid JSON.
  }
  return "0.0.0";
}

const version = readPackageVersion();

// https://vite.dev/config/
export default defineConfig({
  define: {
    "import.meta.env.VITE_APP_VERSION": JSON.stringify(version),
  },
  plugins: [
    react(),
    tailwindcss(),
    babel({
      presets: [reactCompilerPreset()],
      exclude: /(?:^|[/\\])Beams\.jsx$/,
    }),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 18008,
    strictPort: true,
    proxy: {
      "/api": {
        target: process.env.CORE_URL ?? "http://127.0.0.1:18080",
        changeOrigin: false,
      },
      "/agent": {
        target: process.env.CORE_URL ?? "http://127.0.0.1:18080",
        changeOrigin: false,
      },
    },
  },
});
