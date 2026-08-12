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

export function NodesPage() {
  const nodes = useNodes();
  const groups = useNodeGroups();
  const assignments = useAssignments();

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-bold">Nodes</h1>
      <ErrorBoundary>
        <InstallCommandCard />
      </ErrorBoundary>

      <section className="rounded border p-4">
        <h2 className="font-semibold">Managed nodes</h2>
        {nodes.isLoading && <p className="text-sm opacity-60">Loading…</p>}
        {nodes.isError && <p className="text-sm text-red-500">Nodes 加载失败</p>}
        <NodeTable nodes={nodes.data ?? []} />
      </section>

      <section className="rounded border p-4">
        <h2 className="font-semibold">Node groups</h2>
        <CreateNodeGroupForm />
        {groups.isLoading && <p className="text-sm opacity-60">Loading…</p>}
        {groups.isError && <p className="text-sm text-red-500">Node groups 加载失败</p>}
        <ul className="mt-2 space-y-1 text-sm">
          {groups.data?.map((g) => (
            <NodeGroupRow key={g.id} group={g} nodes={nodes.data ?? []} />
          ))}
        </ul>
      </section>

      <ErrorBoundary>
        <AssignmentForm groups={groups.data ?? []} assignments={assignments} />
      </ErrorBoundary>
    </div>
  );
}

function NodeTable({ nodes }: { nodes: ManagedNode[] }) {
  return (
    <Table className="mt-2 w-full text-left text-sm">
      <TableHeader>
        <TableRow className="border-b opacity-60">
          <TableHead className="p-2">Name</TableHead>
          <TableHead className="p-2">Observed</TableHead>
          <TableHead className="p-2">Last result</TableHead>
          <TableHead className="p-2">Last seen</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {nodes.map((n) => (
          <TableRow key={n.id} className="border-b">
            <TableCell className="p-2 font-mono">{n.name}</TableCell>
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
