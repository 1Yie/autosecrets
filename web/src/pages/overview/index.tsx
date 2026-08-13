import { History, RefreshCw, ScrollText, ShieldCheck } from "lucide-react";
import { useOverview, type OverviewAttention } from "../../hooks/fleet/use-overview";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Skeleton } from "../../components/ui/skeleton";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "../../components/ui/empty";
import { Frame } from "../../components/ui/frame";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../components/ui/table";
import { Tabs, TabsList, TabsPanel, TabsTab } from "../../components/ui/tabs";

const attentionLabels: Record<string, string> = {
  failed_convergence: "密钥同步失败",
  offline_node: "节点离线",
  unassigned_node: "未分配节点",
  unclassified_environment: "未分类环境",
  cleanup_failed: "卸载清理失败",
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

      <Tabs defaultValue="attention">
        <TabsList>
          <TabsTab value="attention">需要关注</TabsTab>
          <TabsTab value="updates">最近更新</TabsTab>
          <TabsTab value="audit">最近审计</TabsTab>
        </TabsList>

        <TabsPanel value="attention">
          <Frame className="w-full">
            <Table variant="card" className="w-full text-left text-sm">
              <TableHeader>
                <TableRow className="border-b opacity-60">
                  <TableHead className="p-2">事项</TableHead>
                  <TableHead className="p-2 text-right">数量</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody data-testid="attention-list">
                {data.attention.length === 0 ? (
                  <TableRow className="border-b">
                    <TableCell colSpan={2} className="p-2">
                      <Empty className="py-8" data-testid="attention-empty">
                        <EmptyHeader>
                          <EmptyMedia variant="icon">
                            <ShieldCheck aria-hidden="true" />
                          </EmptyMedia>
                          <EmptyTitle>当前没有需要关注的事项</EmptyTitle>
                          <EmptyDescription>所有节点与密钥状态正常。</EmptyDescription>
                        </EmptyHeader>
                      </Empty>
                    </TableCell>
                  </TableRow>
                ) : (
                  data.attention.map((item: OverviewAttention, index) => (
                    <TableRow key={`${item.kind}-${index}`} className="border-b">
                      <TableCell className="p-2">
                        <Badge variant="warning" size="sm">
                          {attentionLabels[item.kind] ?? item.kind}
                        </Badge>
                      </TableCell>
                      <TableCell className="p-2 text-right tabular-nums">{item.count}</TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </Frame>
        </TabsPanel>

        <TabsPanel value="updates">
          <Frame className="w-full">
            <Table variant="card" className="w-full text-left text-sm">
              <TableHeader>
                <TableRow className="border-b opacity-60">
                  <TableHead className="p-2">版本</TableHead>
                  <TableHead className="p-2">类别</TableHead>
                  <TableHead className="p-2">操作人</TableHead>
                  <TableHead className="p-2 text-right">时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.recent_publishes.length === 0 ? (
                  <TableRow className="border-b">
                    <TableCell colSpan={4} className="p-2">
                      <Empty className="py-8">
                        <EmptyHeader>
                          <EmptyMedia variant="icon">
                            <History aria-hidden="true" />
                          </EmptyMedia>
                          <EmptyTitle>暂无更新记录</EmptyTitle>
                          <EmptyDescription>还没有发布任何变更。</EmptyDescription>
                        </EmptyHeader>
                      </Empty>
                    </TableCell>
                  </TableRow>
                ) : (
                  data.recent_publishes.map((revision) => (
                    <TableRow key={revision.id} className="border-b">
                      <TableCell className="p-2 font-mono text-xs text-muted-foreground">
                        {revision.id.slice(0, 8)}
                      </TableCell>
                      <TableCell className="p-2">
                        <Badge variant="outline" size="sm">{revision.operation_reason.category}</Badge>
                      </TableCell>
                      <TableCell className="p-2">{revision.created_by}</TableCell>
                      <TableCell className="p-2 text-right text-xs text-muted-foreground">
                        {new Date(revision.created_at).toLocaleString()}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </Frame>
        </TabsPanel>

        <TabsPanel value="audit">
          <Frame className="w-full">
            <Table variant="card" className="w-full text-left text-sm">
              <TableHeader>
                <TableRow className="border-b opacity-60">
                  <TableHead className="p-2">操作人</TableHead>
                  <TableHead className="p-2">动作</TableHead>
                  <TableHead className="p-2 text-right">时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.recent_audit.length === 0 ? (
                  <TableRow className="border-b">
                    <TableCell colSpan={3} className="p-2">
                      <Empty className="py-8">
                        <EmptyHeader>
                          <EmptyMedia variant="icon">
                            <ScrollText aria-hidden="true" />
                          </EmptyMedia>
                          <EmptyTitle>暂无审计事件</EmptyTitle>
                          <EmptyDescription>还没有审计记录。</EmptyDescription>
                        </EmptyHeader>
                      </Empty>
                    </TableCell>
                  </TableRow>
                ) : (
                  data.recent_audit.map((event) => (
                    <TableRow key={event.id} className="border-b">
                      <TableCell className="p-2 font-mono text-xs text-muted-foreground">{event.actor}</TableCell>
                      <TableCell className="p-2">
                        <Badge variant="outline" size="sm">{event.action}</Badge>
                      </TableCell>
                      <TableCell className="p-2 text-right text-xs text-muted-foreground">
                        {new Date(event.created_at).toLocaleString()}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </Frame>
        </TabsPanel>
      </Tabs>
    </div>
  );
}

export default OverviewPage;
