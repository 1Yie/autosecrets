import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";

const statusStyles: Record<string, { dot: string; label: string }> = {
  healthy: { dot: "bg-emerald-500", label: "正常" },
  converging: { dot: "bg-blue-500", label: "同步中" },
  failed: { dot: "bg-red-500", label: "失败" },
  offline: { dot: "bg-amber-500", label: "离线" },
  never_online: { dot: "bg-muted-foreground/64", label: "未上线" },
};

/** Status pill with a semantic color dot (never color-only). */
export function StatusBadge({ status, unassigned = false }: { status: string; unassigned?: boolean }) {
  const style = statusStyles[status] ?? { dot: "bg-muted-foreground/64", label: status };
  return (
    <Badge variant="outline">
      <span aria-hidden="true" className={cn("size-1.5 rounded-full", style.dot)} />
      {style.label}
      {status === "healthy" && unassigned && (
        <span className="text-muted-foreground/72">· 未分配</span>
      )}
    </Badge>
  );
}
