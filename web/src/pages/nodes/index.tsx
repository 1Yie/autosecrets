import { useState } from "react";
import { useDocumentTitle } from "../../hooks/use-document-title";
import { useNodes } from "../../hooks/fleet/use-nodes";
import { useNodeGroups } from "../../hooks/fleet/use-node-groups";
import {
	type Assignment,
	useAssignments,
} from "../../hooks/fleet/use-assignments";
import { useDeleteNodeGroup } from "../../hooks/fleet/use-delete-node-group";
import { useDeleteNode } from "../../hooks/fleet/use-delete-node";
import { CreateNodeForm } from "./create-node-form";
import { EditNodeDialog } from "./edit-node-dialog";
import { InstallCommandCard } from "./install-command-card";
import { CreateNodeGroupForm } from "./create-node-group-form";
import { NodeGroupSheet } from "./node-group-sheet";
import { ConfirmDelete } from "../../components/confirm-delete";
import { ErrorBoundary } from "../../components/error-boundary";
import type { ManagedNode } from "../../hooks/fleet/use-nodes";
import { pollIntervalLabel } from "../../lib/constants/poll-interval";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "../../components/ui/table";
import { Skeleton } from "../../components/ui/skeleton";
import { Frame } from "../../components/ui/frame";
import { TablePagination } from "../../components/table-pagination";
import { StatusBadge } from "../../components/status-badge";
import { Tabs, TabsContent, TabsList, TabsTab } from "../../components/ui/tabs";
import { Badge } from "../../components/ui/badge";
import { toastError, toastSuccess } from "../../lib/toast";
import type { NodeGroup } from "../../hooks/fleet/use-node-groups";

export function NodesPage() {
	const nodes = useNodes();
	useDocumentTitle("节点");
	const groups = useNodeGroups();
	const assignments = useAssignments(100);
	const [tab, setTab] = useState("nodes");

	return (
		<div className="flex flex-col gap-6">
			<h1 className="text-xl font-bold">节点</h1>

			<Tabs value={tab} onValueChange={setTab}>
				<div className="flex items-center justify-between gap-3">
					<TabsList>
						<TabsTab value="nodes">托管节点</TabsTab>
						<TabsTab value="groups">节点组</TabsTab>
					</TabsList>
					{tab === "nodes" ? (
						<ErrorBoundary>
							<CreateNodeForm />
						</ErrorBoundary>
					) : (
						<CreateNodeGroupForm />
					)}
				</div>

				<TabsContent value="nodes" className="pt-2">
					{nodes.isLoading && <Skeleton className="h-24 w-full" />}
					{nodes.isError && <p className="text-sm text-red-500">节点列表加载失败</p>}
					{nodes.items.length === 0 && !nodes.isLoading && (
						<p className="opacity-60">
							还没有托管节点，点右侧「添加服务器」登记一台。
						</p>
					)}
					{nodes.items.length > 0 && (
						<Frame className="w-full">
							<NodeTable nodes={nodes.items} />
							<TablePagination noun="节点" page={nodes} />
						</Frame>
					)}
				</TabsContent>

				<TabsContent value="groups" className="flex flex-col gap-3 pt-2">
					{groups.isLoading && <Skeleton className="h-8 w-48" />}
					{groups.isError && <p className="text-sm text-red-500">节点组加载失败</p>}
					{groups.items.length === 0 && !groups.isLoading && (
						<p className="opacity-60">还没有节点组，点右侧「新建节点组」开始。</p>
					)}
					{groups.items.length > 0 && (
						<Frame className="w-full">
							<Table variant="card">
								<TableHeader>
									<TableRow className="hover:bg-transparent">
										<TableHead>节点组</TableHead>
										<TableHead>节点数</TableHead>
										<TableHead>分配</TableHead>
										<TableHead className="text-right">操作</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{groups.items.map((group) => (
										<TableRow key={group.id}>
											<TableCell className="font-medium">{group.name}</TableCell>
											<TableCell>
												<Badge variant="outline">{group.member_ids.length}</Badge>
											</TableCell>
											<TableCell>
												<Badge variant="outline">
													{activeAssignmentCount(assignments.items, group.id)}
												</Badge>
											</TableCell>
											<TableCell className="text-right">
												<div className="flex items-center justify-end gap-2">
													<NodeGroupSheet group={group} />
													<DeleteNodeGroupButton group={group} />
												</div>
											</TableCell>
										</TableRow>
									))}
								</TableBody>
							</Table>
							<TablePagination noun="节点组" page={groups} />
						</Frame>
					)}
				</TabsContent>
			</Tabs>
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
					<TableHead className="p-2">更新频率</TableHead>
					<TableHead className="p-2">最后在线</TableHead>
					<TableHead className="p-2 text-right">操作</TableHead>
				</TableRow>
			</TableHeader>
			<TableBody>
				{nodes.map((n) => (
					<TableRow key={n.id} className="border-b">
						<TableCell className="p-2 font-mono">{n.name}</TableCell>
						<TableCell className="p-2">
							<StatusBadge status={n.state} unassigned={n.unassigned} />
						</TableCell>
						<TableCell className="p-2 font-mono">
							{n.observed_revision ? `${n.observed_revision.slice(0, 8)}…` : "—"}
						</TableCell>
						<TableCell className="p-2">{n.last_result}</TableCell>
						<TableCell className="p-2">
							{pollIntervalLabel(n.poll_interval_seconds)}
						</TableCell>
						<TableCell className="p-2">
							{n.last_seen_at ? new Date(n.last_seen_at).toLocaleTimeString() : "从未"}
						</TableCell>
						<TableCell className="p-2">
							<NodeActions node={n} />
						</TableCell>
					</TableRow>
				))}
			</TableBody>
		</Table>
	);
}

function NodeActions({ node }: { node: ManagedNode }) {
	const remove = useDeleteNode();
	return (
		<div className="flex items-center justify-end gap-2">
			<InstallCommandCard node={node} />
			<EditNodeDialog node={node} />
			<ConfirmDelete
				label="删除"
				title={`删除服务器 ${node.name}？`}
				description="会删除该节点及其预留的安装令牌。已在节点组中的成员关系也会一并移除。"
				pending={remove.isPending}
				error={remove.isError ? String((remove.error as Error).message) : undefined}
				onConfirm={() =>
					remove.mutate(node.id, {
						onSuccess: () => toastSuccess("服务器已删除"),
					})
				}
			/>
		</div>
	);
}

function activeAssignmentCount(items: Assignment[], groupId: string) {
	return items.filter(
		(assignment) =>
			assignment.group_id === groupId && assignment.status === "active",
	).length;
}

function readableDeleteNodeGroupError(error: unknown): string {
	const message = error instanceof Error ? error.message : String(error);
	if (
		message === "node group still has an active assignment" ||
		message.includes("still has an active assignment")
	) {
		return "该节点组仍有应用分配，请先在「分配」里解除";
	}
	return message || "删除节点组失败";
}

function DeleteNodeGroupButton({ group }: { group: NodeGroup }) {
	const removeGroup = useDeleteNodeGroup();
	const [open, setOpen] = useState(false);
	return (
		<ConfirmDelete
			description="会删除该节点组及其成员关系。如果还有应用分配，先打开「管理 → 分配」解除。"
			label="删除"
			open={open}
			pending={removeGroup.isPending}
			title={`删除节点组 ${group.name}？`}
			onConfirm={() =>
				removeGroup.mutate(group.id, {
					onSuccess: () => {
						setOpen(false);
						toastSuccess("节点组已删除");
					},
					onError: (error) => {
						setOpen(false);
						toastError(readableDeleteNodeGroupError(error));
					},
				})
			}
			onOpenChange={setOpen}
		/>
	);
}

export default NodesPage;
