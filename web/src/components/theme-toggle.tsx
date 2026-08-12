import { Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";

/** Three-state theme toggle: light / dark / system, persisted locally. */
export function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme();
  const next = resolvedTheme === "dark" ? "light" : "dark";
  return (
    <button
      className="flex size-6 items-center justify-center rounded opacity-70 hover:opacity-100"
      onClick={() => setTheme(next)}
      aria-label={resolvedTheme === "dark" ? "切换到浅色" : "切换到深色"}
      data-testid="theme-toggle"
    >
      {resolvedTheme === "dark" ? <Sun className="size-4" /> : <Moon className="size-4" />}
    </button>
  );
}
