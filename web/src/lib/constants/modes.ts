// Safe file modes accepted for File Bindings (must stay in sync with Core's
// allowlist in core/internal/app/secrets.go).
export const SAFE_BINDING_MODES = [
	"0400",
	"0440",
	"0444",
	"0600",
	"0640",
	"0644",
] as const;

export type SafeBindingMode = (typeof SAFE_BINDING_MODES)[number];

export const DEFAULT_BINDING_MODE: SafeBindingMode = "0400";

export const BINDING_MODE_LABELS: Record<SafeBindingMode, string> = {
	"0400": "0400（仅所有者可读）",
	"0440": "0440（所有者与组可读）",
	"0444": "0444（所有人可读）",
	"0600": "0600（仅所有者可读写）",
	"0640": "0640（所有者可读写，组可读）",
	"0644": "0644（所有者可读写，其他人可读）",
};

export function bindingModeLabel(mode: string): string {
	return BINDING_MODE_LABELS[mode as SafeBindingMode] ?? mode;
}
