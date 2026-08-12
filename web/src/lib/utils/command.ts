// Pure helpers for parsing the generated Install Command. Kept testable and
// shared between the UI and the E2E fixture.
export interface InstallCommand {
  server: string;
  token: string;
  name: string;
}

export function parseInstallCommand(command: string): InstallCommand {
  const fields = command.split(/\s+/);
  const valueOf = (flag: string): string => {
    const idx = fields.indexOf(flag);
    return idx >= 0 && idx + 1 < fields.length ? fields[idx + 1] : "";
  };
  return {
    server: valueOf("--server"),
    token: valueOf("--token"),
    name: valueOf("--name").replace(/^"|"$/g, ""),
  };
}
