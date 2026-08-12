import { useState } from "react";
import { useAuditEvents, type AuditFilters } from "../../hooks/audit/use-audit-events";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Skeleton } from "../../components/ui/skeleton";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../components/ui/table";

const reasonCategories = ["maintenance", "incident_response", "access_change", "configuration_correction", "other"];

export function AuditPage() {
  const audit = useAuditEvents();
  const [draft, setDraft] = useState<AuditFilters>(audit.filters);

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
      <form
        className="flex flex-wrap items-end gap-2 text-sm"
        onSubmit={(event) => {
          event.preventDefault();
          audit.applyFilters(draft);
        }}
        data-testid="audit-filters"
      >
        <FilterInput label="操作者" value={draft.actor} onChange={(v) => setDraft({ ...draft, actor: v })} testId="filter-actor" />
        <FilterInput label="动作" value={draft.action} onChange={(v) => setDraft({ ...draft, action: v })} testId="filter-action" />
        <FilterInput label="资源" value={draft.resource} onChange={(v) => setDraft({ ...draft, resource: v })} testId="filter-resource" />
        <label className="flex flex-col gap-1">
          <span className="opacity-60">结果</span>
          <Select value={draft.outcome || "all"} onValueChange={(v) => setDraft({ ...draft, outcome: v === "all" ? "" : v ?? "" })}>
            <SelectTrigger className="w-28" data-testid="filter-outcome">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部</SelectItem>
              <SelectItem value="ok">ok</SelectItem>
              <SelectItem value="denied">denied</SelectItem>
              <SelectItem value="failed">failed</SelectItem>
            </SelectContent>
          </Select>
        </label>
        <label className="flex flex-col gap-1">
          <span className="opacity-60">原因类别</span>
          <Select value={draft.reason_category || "all"} onValueChange={(v) => setDraft({ ...draft, reason_category: v === "all" ? "" : v ?? "" })}>
            <SelectTrigger className="w-40" data-testid="filter-reason">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部</SelectItem>
              {reasonCategories.map((category) => (
                <SelectItem key={category} value={category}>{category}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>
        <Button variant="default" size="sm" type="submit">
          应用筛选
        </Button>
      </form>

      {audit.items.length === 0 ? (
        <p className="text-sm opacity-60">暂无匹配的审计事件。</p>
      ) : (
        <Table variant="card" className="w-full">
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead>时间</TableHead>
              <TableHead>操作者</TableHead>
              <TableHead>动作</TableHead>
              <TableHead>结果</TableHead>
              <TableHead>原因</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {audit.items.map((event) => (
              <TableRow key={event.id}>
                <TableCell className="whitespace-nowrap text-muted-foreground">
                  {new Date(event.created_at).toLocaleString()}
                </TableCell>
                <TableCell className="font-mono text-xs">{event.actor_display || event.actor}</TableCell>
                <TableCell>{event.action}</TableCell>
                <TableCell>{event.outcome || event.result}</TableCell>
                <TableCell className="max-w-48 truncate text-muted-foreground">
                  {event.operation_reason_category || "—"}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      <div className="flex items-center gap-2 text-sm">
        <Button variant="outline" size="sm" disabled={audit.isFirstPage} onClick={audit.prev}>
          上一页
        </Button>
        <Button variant="outline" size="sm" disabled={!audit.nextCursor} onClick={audit.next}>
          下一页
        </Button>
      </div>
    </div>
  );
}

function FilterInput({ label, value, onChange, testId }: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  testId: string;
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="opacity-60">{label}</span>
      <Input className="w-40" value={value} onChange={(event) => onChange(event.target.value)} data-testid={testId} />
    </label>
  );
}

export default AuditPage;
