// Preset Agent polling intervals offered in the nodes page. The bounds
// (5s-24h) mirror Core's CHECK constraint on nodes.poll_interval_seconds.
export const POLL_INTERVAL_OPTIONS = [
	5, 10, 15, 30, 60, 300, 600, 1800, 3600,
] as const;

export function pollIntervalLabel(seconds: number): string {
	if (seconds < 60) return `${seconds}秒`;
	if (seconds % 3600 === 0) return `${seconds / 3600}小时`;
	if (seconds % 60 === 0) return `${seconds / 60}分钟`;
	return `${seconds}秒`;
}
