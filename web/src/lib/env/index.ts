// Typed environment access. Components must never read import.meta.env or
// process.env directly; add an exported constant here for every variable.
export const API_BASE = import.meta.env.VITE_API_BASE ?? "";
