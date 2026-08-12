import { useAuditEvents } from "../../hooks/audit/use-audit-events";
import { Button } from "../../components/ui/button";
import { Skeleton } from "../../components/ui/skeleton";

export function AuditPage() {
  const audit = useAuditEvents();

  if (audit.isLoading) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-8 w-32" />
        <Skeleton className="h-64" />
      </div>
    );
  }
  if (audit.isError || !audit.data) {
    return (
      <div className="space-y-3">
        <h1 className="text-xl font-bold">审计</h1>
        <p className="text-sm text-red-500">审计事件加载失败</p>
        <Button variant="default" onClick={() => audit.refetch()}>
          重试
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold">审计</h1>
      {audit.data.length === 0 ? (
        <p className="text-sm opacity-60">暂无审计事件。</p>
      ) : (
        <div className="overflow-x-auto rounded-lg border">
          <table className="w-full text-sm">
            <thead className="border-b bg-muted/40 text-left">
              <tr>
                <th className="px-3 py-2 font-medium">时间</th>
                <th className="px-3 py-2 font-medium">操作者</th>
                <th className="px-3 py-2 font-medium">动作</th>
                <th className="px-3 py-2 font-medium">资源</th>
                <th className="px-3 py-2 font-medium">结果</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {audit.data.map((event) => (
                <tr key={event.id}>
                  <td className="whitespace-nowrap px-3 py-2 opacity-60">
                    {new Date(event.created_at).toLocaleString()}
                  </td>
                  <td className="px-3 py-2 font-mono text-xs">{event.actor}</td>
                  <td className="px-3 py-2">{event.action}</td>
                  <td className="max-w-48 truncate px-3 py-2 font-mono text-xs opacity-70">
                    {event.resource}
                  </td>
                  <td className="px-3 py-2">{event.result}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

export default AuditPage;
