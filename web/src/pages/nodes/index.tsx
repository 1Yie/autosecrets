import { useNodes } from "../../hooks/fleet/use-nodes";
import { useNodeGroups } from "../../hooks/fleet/use-node-groups";
import { useAssignments } from "../../hooks/fleet/use-assignments";
import { InstallCommandCard } from "./install-command-card";
import { NodeGroupRow } from "./node-group-row";
import { AssignmentForm } from "./assignment-form";
import { CreateNodeGroupForm } from "./create-node-group-form";
import { ErrorBoundary } from "../../components/error-boundary";
import type { ManagedNode } from "../../hooks/fleet/use-nodes";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../components/ui/table";
import { Skeleton } from "../../components/ui/skeleton";
import { Button } from "../../components/ui/button";
import { Frame, FrameFooter } from "../../components/ui/frame";
import { StatusBadge } from "../../components/status-badge";

export function NodesPage() {
  const nodes = useNodes();
  const groups = useNodeGroups();
  const assignments = useAssignments();

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-bold">节点</h1>
      <ErrorBoundary>
        <InstallCommandCard />
      </ErrorBoundary>

      <section className="space-y-2">
        <h2 className="font-semibold">托管节点</h2>
        {nodes.isLoading && <Skeleton className="h-24 w-full" />}
        {nodes.isError && <p className="text-sm text-red-500">节点列表加载失败</p>}
        {nodes.items.length > 0 && (
          <Frame className="mt-2 w-full">
            <NodeTable nodes={nodes.items} />
            <FrameFooter className="p-2">
              <div className="flex items-center justify-between gap-2">
                <p className="text-muted-foreground text-sm">
                  共 <strong className="font-medium text-foreground">{nodes.items.length}</strong> 个节点
                </p>
                <div className="flex items-center gap-2">
                  <Button variant="outline" size="sm" disabled={nodes.isFirstPage} onClick={nodes.prev}>
                    上一页
                  </Button>
                  <Button variant="outline" size="sm" disabled={!nodes.nextCursor} onClick={nodes.next}>
                    下一页
                  </Button>
                </div>
              </div>
            </FrameFooter>
          </Frame>
        )}
      </section>

      <section className="space-y-2">
        <h2 className="font-semibold">节点组</h2>
        <CreateNodeGroupForm />
        {groups.isLoading && <Skeleton className="h-8 w-48" />}
        {groups.isError && <p className="text-sm text-red-500">节点组加载失败</p>}
        <ul className="mt-2 space-y-1 text-sm">
          {groups.items.map((g) => (
            <NodeGroupRow key={g.id} group={g} nodes={nodes.items} />
          ))}
        </ul>
      </section>

      <ErrorBoundary>
        <AssignmentForm groups={groups.items} assignments={assignments} />
      </ErrorBoundary>
    </div>
  );
}

function NodeTable({ nodes }: { nodes: ManagedNode[] }) {
  return (
    <Table variant="card">
      <TableHeader>
        <TableRow className="border-b opacity-60">
          <TableHead className="p-2">名称</TableHead>
          <TableHead className="p-2">状态</TableHead>
          <TableHead className="p-2">已同步版本</TableHead>
          <TableHead className="p-2">最近结果</TableHead>
          <TableHead className="p-2">最后在线</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {nodes.map((n) => (
          <TableRow key={n.id} className="border-b">
            <TableCell className="p-2 font-mono">{n.name}</TableCell>
            <TableCell className="p-2">
              <StatusBadge status={n.state} unassigned={n.unassigned} />
            </TableCell>
            <TableCell className="p-2 font-mono">{n.observed_revision.slice(0, 8)}…</TableCell>
            <TableCell className="p-2">{n.last_result}</TableCell>
            <TableCell className="p-2">
              {n.last_seen_at ? new Date(n.last_seen_at).toLocaleTimeString() : "never"}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}


export default NodesPage;
