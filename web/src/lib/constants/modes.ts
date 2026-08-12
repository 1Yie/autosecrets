// Safe file modes accepted for File Bindings (must stay in sync with Core's
// allowlist in core/internal/app/secrets.go).
export const SAFE_BINDING_MODES = ["0400", "0440", "0444", "0600", "0640", "0644"] as const;

export type SafeBindingMode = (typeof SAFE_BINDING_MODES)[number];

export const DEFAULT_BINDING_MODE: SafeBindingMode = "0400";
