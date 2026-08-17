import { useState, type ReactNode } from "react";
import { useParams } from "react-router-dom";
import { useForm } from "react-hook-form";
import { Lock, MoreVertical, Send } from "lucide-react";
import { useDocumentTitle } from "../../hooks/use-document-title";
import {
	useApplication,
	type Environment,
} from "../../hooks/applications/use-application";
import { useCreateEnvironment } from "../../hooks/applications/use-create-environment";
import { useDeleteEnvironment } from "../../hooks/applications/use-delete-environment";
import { useDraft } from "../../hooks/applications/use-draft";
import { usePublish } from "../../hooks/applications/use-publish";
import { toastError, toastSuccess } from "../../lib/toast";
import { ConfirmDelete } from "../../components/confirm-delete";
import { SecretEditor } from "./secret-editor";
import { ErrorBoundary } from "../../components/error-boundary";
import { PageBackLink } from "../../components/page-back-link";
import { zodResolver } from "@hookform/resolvers/zod";
import { nameSchema } from "../../lib/constants/schemas";
import { cn } from "../../lib/utils";
import { z } from "zod";
import { Button } from "../../components/ui/button";
import {
	Menu,
	MenuItem,
	MenuPopup,
	MenuTrigger,
} from "../../components/ui/menu";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "../../components/ui/empty";
import { Input } from "../../components/ui/input";
import { Skeleton } from "../../components/ui/skeleton";
import { Field, FieldLabel } from "../../components/ui/field";
import {
	Dialog,
	DialogClose,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogPanel,
	DialogPopup,
	DialogTitle,
	DialogTrigger,
} from "../../components/ui/dialog";

const envSchema = z.object({
	name: nameSchema,
});

function AppPublishControls({
	appId,
	envId,
}: {
	appId: string;
	envId: string;
}) {
	const draft = useDraft(appId, envId);
	const publish = usePublish(appId, envId);
	const canPublish = draft.isSuccess && (draft.data?.selections.length ?? 0) > 0;

	return (
		<div className="flex shrink-0 flex-col items-end gap-1">
			<Button
				variant="default"
				className="border-zinc-200 bg-white text-zinc-900 hover:bg-zinc-50 data-pressed:bg-zinc-50 *:data-[slot=button-loading-indicator]:text-zinc-900 dark:border-white/20 dark:bg-white dark:text-zinc-900 dark:hover:bg-white/90 dark:data-pressed:bg-white/90"
				disabled={!canPublish || publish.isPending}
				onClick={() =>
					publish.mutate(undefined, {
						onSuccess: () => toastSuccess("已发布，节点将自动同步"),
						onError: (error) =>
							toastError(error instanceof Error ? error.message : String(error)),
					})
				}
			>
				<Send aria-hidden="true" />
				发布
			</Button>
		</div>
	);
}

export function AppPage() {
	const { appId } = useParams<{ appId: string }>();
	const app = useApplication(appId ?? "");
	const createEnv = useCreateEnvironment(appId ?? "");
	const deleteEnv = useDeleteEnvironment(appId ?? "");
	const [activeEnv, setActiveEnv] = useState<string | null>(null);
	const [envDialogOpen, setEnvDialogOpen] = useState(false);
	const { register, handleSubmit, reset } = useForm<{
		name: string;
	}>({
		resolver: zodResolver(envSchema),
	});

	const environments = app.data?.environments ?? [];
	const selectedId =
		(activeEnv &&
			environments.some((env) => env.id === activeEnv) &&
			activeEnv) ||
		environments[0]?.id ||
		activeEnv;
	const selectedEnv = environments.find((env) => env.id === selectedId);
	useDocumentTitle(
		app.data?.name && selectedEnv
			? `${app.data.name} · ${selectedEnv.name}`
			: app.data?.name,
	);

	if (app.isLoading) {
		return (
			<div className="flex flex-col gap-4">
				<Skeleton className="h-6 w-32" />
				<Skeleton className="h-24 w-full" />
			</div>
		);
	}
	if (app.isError) return <p className="text-red-500">应用不存在</p>;

	const createEnvironment = (
		<Dialog open={envDialogOpen} onOpenChange={setEnvDialogOpen}>
			<DialogTrigger render={<Button variant="default" />}>新建环境</DialogTrigger>
			<DialogPopup>
				<DialogHeader>
					<DialogTitle>新建环境</DialogTitle>
					<DialogDescription>环境是密钥的分组。</DialogDescription>
				</DialogHeader>
				<form
					className="contents"
					onSubmit={handleSubmit((v) => {
						createEnv.mutate(
							{ name: v.name },
							{
								onSuccess: (created) => {
									setEnvDialogOpen(false);
									reset();
									setActiveEnv(created.id);
								},
							},
						);
					})}
				>
					<DialogPanel>
						<div className="flex flex-col gap-3">
							<Field className="w-full">
								<FieldLabel>名称</FieldLabel>
								<Input
									className="w-full"
									placeholder="例如 production"
									data-testid="env-name"
									{...register("name")}
								/>
							</Field>
							{createEnv.isError && (
								<p className="text-sm text-red-500">
									{String((createEnv.error as Error).message)}
								</p>
							)}
						</div>
					</DialogPanel>
					<DialogFooter>
						<DialogClose render={<Button variant="ghost" />}>取消</DialogClose>
						<Button variant="default" type="submit" loading={createEnv.isPending}>
							创建
						</Button>
					</DialogFooter>
				</form>
			</DialogPopup>
		</Dialog>
	);

	return (
		<div className="flex flex-col gap-6">
			<div className="flex items-start justify-between gap-4">
				<div className="flex flex-col gap-1">
					<PageBackLink to="/dashboard/apps">应用</PageBackLink>
					<h1 className="text-xl font-bold">{app.data?.name}</h1>
				</div>
				{selectedId && (
					<AppPublishControls
						key={selectedId}
						appId={appId ?? ""}
						envId={selectedId}
					/>
				)}
			</div>

			<EnvironmentSwitcher
				createAction={createEnvironment}
				environments={environments}
				selectedId={selectedId}
				onSelect={setActiveEnv}
				onDelete={(envId, onDeleted) => {
					deleteEnv.mutate(envId, {
						onSuccess: () => {
							if (activeEnv === envId) setActiveEnv(null);
							onDeleted();
						},
					});
				}}
				deleting={deleteEnv.isPending}
				deleteError={
					deleteEnv.isError ? String((deleteEnv.error as Error).message) : undefined
				}
			/>

			{selectedId && (
				<ErrorBoundary>
					<SecretEditor appId={appId ?? ""} envId={selectedId} />
				</ErrorBoundary>
			)}
		</div>
	);
}

function EnvironmentSwitcher({
	createAction,
	environments,
	selectedId,
	onSelect,
	onDelete,
	deleting,
	deleteError,
}: {
	createAction: ReactNode;
	environments: Environment[];
	selectedId: string | null;
	onSelect: (id: string) => void;
	onDelete: (id: string, onDeleted: () => void) => void;
	deleting: boolean;
	deleteError?: string;
}) {
	const [deleteTarget, setDeleteTarget] = useState<Environment | null>(null);

	return (
		<div className="flex flex-col gap-2">
			<div className="flex items-center justify-between">
				<h2 className="font-semibold">环境</h2>
				{createAction}
			</div>
			{environments.length === 0 ? (
				<Empty className="border-dashed">
					<EmptyHeader>
						<EmptyMedia variant="icon">
							<Lock aria-hidden="true" />
						</EmptyMedia>
						<EmptyTitle>还没有环境</EmptyTitle>
						<EmptyDescription>
							先建一个环境，再在里面编辑和发布密钥。
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			) : (
				<div
					role="listbox"
					aria-label="选择环境"
					className="flex w-fit max-w-full flex-wrap gap-0.5 rounded-lg bg-muted p-0.5"
				>
					{environments.map((env) => {
						const active = env.id === selectedId;
						return (
							<div
								key={env.id}
								onClick={() => onSelect(env.id)}
								className={cn(
									"group inline-flex h-8 shrink-0 cursor-pointer select-none items-center rounded-md transition-colors",
									active
										? "bg-background text-foreground shadow-sm/5"
										: "hover:bg-foreground/8",
								)}
							>
								<button
									type="button"
									role="option"
									aria-selected={active}
									data-testid={`env-${env.name}`}
									className={cn(
										"inline-flex h-8 items-center gap-1.5 ps-2.5 pe-1 text-sm outline-none",
										"focus-visible:ring-2 focus-visible:ring-ring",
										active
											? "text-foreground"
											: "text-muted-foreground hover:text-foreground",
									)}
								>
									<span className="font-medium">{env.name}</span>
								</button>
								<div className="me-0.5 flex size-6 shrink-0 items-center justify-center">
									<Menu>
										<MenuTrigger
											aria-label={`${env.name} 更多操作`}
											className={cn(
												"inline-flex size-6 items-center justify-center rounded-md outline-none transition-colors",
												"hover:bg-foreground/8 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring",
												active
													? "text-muted-foreground"
													: "text-muted-foreground/40 group-hover:text-muted-foreground",
											)}
											data-testid={`env-${env.name}-more`}
											onClick={(event) => event.stopPropagation()}
										>
											<MoreVertical aria-hidden="true" className="size-3.5" />
										</MenuTrigger>
										<MenuPopup
											align="end"
											className="min-w-0"
											side="bottom"
											onClick={(event) => event.stopPropagation()}
										>
											<MenuItem
												closeOnClick
												variant="destructive"
												onClick={() => setDeleteTarget(env)}
											>
												删除环境
											</MenuItem>
										</MenuPopup>
									</Menu>
								</div>
							</div>
						);
					})}
				</div>
			)}
			<ConfirmDelete
				open={deleteTarget !== null}
				onOpenChange={(next) => {
					if (!next) setDeleteTarget(null);
				}}
				title={deleteTarget ? `删除环境 ${deleteTarget.name}？` : "删除环境？"}
				description="会删除该环境下的密钥。若仍有节点组绑定，需要先解除分配。"
				pending={deleting}
				error={deleteError}
				onConfirm={() => {
					if (!deleteTarget) return;
					onDelete(deleteTarget.id, () => setDeleteTarget(null));
				}}
			/>
		</div>
	);
}

export default AppPage;
