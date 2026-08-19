// Typed environment access. Components must never read import.meta.env or
// process.env directly; add an exported constant here for every variable.

export function resolveApiBase(envBase: string | undefined): string {
	return (envBase ?? "").trim().replace(/\/$/, "");
}

export const API_BASE = resolveApiBase(import.meta.env.VITE_API_BASE);

export const APP_VERSION = (import.meta.env.VITE_APP_VERSION ?? "0.0.0").trim();

export function apiURL(path: string): string {
	if (/^https?:\/\//.test(path)) {
		return path;
	}
	return `${API_BASE}${path}`;
}
