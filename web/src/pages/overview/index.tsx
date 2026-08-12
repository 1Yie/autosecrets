import { RefreshCw } from "lucide-react";
import { useOverview, type OverviewAttention } from "../../hooks/fleet/use-overview";
import { Button } from "../../components/ui/button";
import { Skeleton } from "../../components/ui/skeleton";

const attentionLabels: Record<string, string> = {
  failed_convergence: "收敛失败",
  offline_node: "节点离线",
  unassigned_node: "未分配节点",
  unclassified_environment: "未分类环境",
  cleanup_failed: "清理失败",
  cleanup_unconfirmed: "清理未确认",
};

export function OverviewPage() {
  const overview = useOverview();

  if (overview.isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-48" />
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-20" />
          ))}
        </div>
        <Skeleton className="h-40" />
      </div>
    );
  }
  if (overview.isError || !overview.data) {
    return (
      <div className="space-y-3">
        <h1 className="text-xl font-bold">概览</h1>
        <p className="text-sm text-red-500">概览加载失败</p>
        <Button variant="default" onClick={() => overview.refetch()}>
          重试
        </Button>
      </div>
    );
  }

  const data = overview.data;
  const counts = data.counts;
  const countCards = [
    { label: "应用", value: counts.applications },
    { label: "环境", value: counts.environments },
    { label: "Secret", value: counts.secrets },
    { label: "节点", value: counts.nodes },
    { label: "节点组", value: counts.node_groups },
    { label: "分配", value: counts.assignments },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">概览</h1>
        <div className="flex items-center gap-2 text-xs opacity-60">
          <span data-testid="generated-at">生成于 {new Date(data.generated_at).toLocaleString()}</span>
          <Button variant="ghost" size="sm" onClick={() => overview.refetch()} aria-label="刷新">
            <RefreshCw className="size-3.5" />
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-6">
        {countCards.map((card) => (
          <div key={card.label} className="rounded-lg border p-3">
            <div className="text-2xl font-semibold tabular-nums">{card.value}</div>
            <div className="text-xs opacity-60">{card.label}</div>
          </div>
        ))}
      </div>

      <section className="space-y-2">
        <h2 className="text-sm font-medium">需要关注</h2>
        {data.attention.length === 0 ? (
          <p className="text-sm opacity-60" data-testid="attention-empty">
            当前没有需要关注的事项。
          </p>
        ) : (
          <ul className="space-y-2" data-testid="attention-list">
            {data.attention.map((item: OverviewAttention, index) => (
              <li key={`${item.kind}-${index}`} className="rounded-lg border border-amber-300/40 bg-amber-50 p-3 text-sm dark:bg-amber-950/20">
                <span className="font-medium">{attentionLabels[item.kind] ?? item.kind}</span>
                <span className="ml-2 opacity-70">{item.count} 项</span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="space-y-2">
        <h2 className="text-sm font-medium">最近发布</h2>
        {data.recent_publishes.length === 0 ? (
          <p className="text-sm opacity-60">暂无发布记录。</p>
        ) : (
          <ul className="divide-y rounded-lg border text-sm">
            {data.recent_publishes.map((revision) => (
              <li key={revision.id} className="flex items-center justify-between px-3 py-2">
                <span className="truncate font-mono text-xs opacity-80">{revision.id.slice(0, 8)}</span>
                <span className="opacity-70">
                  {revision.operation_reason.category} · {revision.created_by}
                </span>
                <span className="opacity-50">{new Date(revision.created_at).toLocaleString()}</span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="space-y-2">
        <h2 className="text-sm font-medium">最近审计</h2>
        {data.recent_audit.length === 0 ? (
          <p className="text-sm opacity-60">暂无审计事件。</p>
        ) : (
          <ul className="divide-y rounded-lg border text-sm">
            {data.recent_audit.map((event) => (
              <li key={event.id} className="flex items-center justify-between px-3 py-2">
                <span className="font-mono text-xs opacity-80">{event.actor}</span>
                <span className="opacity-70">{event.action}</span>
                <span className="opacity-50">{new Date(event.created_at).toLocaleString()}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

export default OverviewPage;
