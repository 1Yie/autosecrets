import { useState } from "react";
import { useDocumentTitle } from "../../hooks/use-document-title";
import { useNodes } from "../../hooks/fleet/use-nodes";
import { useNodeGroups } from "../../hooks/fleet/use-node-groups";
import { useDeleteNodeGroup } from "../../hooks/fleet/use-delete-node-group";
import { useUpdateNode } from "../../hooks/fleet/use-update-node";
import { InstallCommandCard } from "./install-command-card";
import { CreateNodeGroupForm } from "./create-node-group-form";
import { NodeGroupSheet } from "./node-group-sheet";
import { ConfirmDelete } from "../../components/confirm-delete";
import { ErrorBoundary } from "../../components/error-boundary";
import type { ManagedNode } from "../../hooks/fleet/use-nodes";
import {
	POLL_INTERVAL_OPTIONS,
	pollIntervalLabel,
} from "../../lib/constants/poll-interval";
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
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "../../components/ui/select";
import { Button } from "../../components/ui/button";
import { toastSuccess } from "../../lib/toast";

export function NodesPage() {
	const nodes = useNodes();
	useDocumentTitle("节点");
	const groups = useNodeGroups();
	const removeGroup = useDeleteNodeGroup();
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
							<InstallCommandCard />
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
							还没有托管节点，点右侧「添加服务器」生成安装命令。
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
											<TableCell className="text-right">
												<div className="flex items-center justify-end gap-2">
													<NodeGroupSheet group={group} nodes={nodes.items} />
													<ConfirmDelete
														label="删除"
														title={`删除节点组 ${group.name}？`}
														description="会删除该节点组及其成员关系。若仍绑定应用环境，需要先解除分配。"
														pending={removeGroup.isPending}
														error={
															removeGroup.isError
																? String((removeGroup.error as Error).message)
																: undefined
														}
														onConfirm={() => removeGroup.mutate(group.id)}
													/>
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
							{n.observed_revision.slice(0, 8)}…
						</TableCell>
						<TableCell className="p-2">{n.last_result}</TableCell>
						<TableCell className="p-2">
							<PollIntervalSelect node={n} />
						</TableCell>
						<TableCell className="p-2">
							{n.last_seen_at
								? new Date(n.last_seen_at).toLocaleTimeString()
								: "never"}
						</TableCell>
					</TableRow>
				))}
			</TableBody>
		</Table>
	);
}

function PollIntervalSelect({ node }: { node: ManagedNode }) {
	const update = useUpdateNode(node.id);
	const [draft, setDraft] = useState(String(node.poll_interval_seconds));
	const dirty = Number(draft) !== node.poll_interval_seconds;
	const options = POLL_INTERVAL_OPTIONS.includes(
		node.poll_interval_seconds as (typeof POLL_INTERVAL_OPTIONS)[number],
	)
		? POLL_INTERVAL_OPTIONS
		: [...POLL_INTERVAL_OPTIONS, node.poll_interval_seconds];

	const save = () => {
		update.mutate(Number(draft), {
			onSuccess: () => toastSuccess("轮询间隔已更新"),
		});
	};

	return (
		<div className="flex items-center gap-1.5">
			<Select
				value={draft}
				onValueChange={(value) => {
					setDraft(value ?? String(node.poll_interval_seconds));
				}}
			>
				<SelectTrigger
					size="sm"
					className="w-28"
					data-testid={`poll-interval-${node.id}`}
				>
					<SelectValue>
						{pollIntervalLabel(Number(draft) || node.poll_interval_seconds)}
					</SelectValue>
				</SelectTrigger>
				<SelectContent>
					{options.map((seconds) => (
						<SelectItem key={seconds} value={String(seconds)}>
							{pollIntervalLabel(seconds)}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
			<Button
				size="xs"
				variant="outline"
				className="shrink-0"
				disabled={!dirty || update.isPending}
				onClick={save}
				data-testid={`poll-interval-save-${node.id}`}
			>
				{update.isPending ? "保存中…" : "保存"}
			</Button>
		</div>
	);
}

export default NodesPage;
