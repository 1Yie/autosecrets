import { useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { MoreVertical } from "lucide-react";
import { useUpdateBinding } from "../../hooks/applications/use-update-binding";
import { useDeleteSecret } from "../../hooks/applications/use-delete-secret";
import { ConfirmDelete } from "../../components/confirm-delete";
import { bindingSchema, type BindingForm } from "../../lib/constants/schemas";
import {
	SAFE_BINDING_MODES,
	DEFAULT_BINDING_MODE,
	BINDING_MODE_LABELS,
	bindingModeLabel,
} from "../../lib/constants/modes";
import { toastSuccess } from "../../lib/toast";
import type { SecretRow } from "../../hooks/applications/use-secrets";
import { Button } from "../../components/ui/button";
import { Field, FieldError, FieldLabel } from "../../components/ui/field";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "../../components/ui/select";
import { TableCell, TableRow } from "../../components/ui/table";
import {
	Dialog,
	DialogClose,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogPanel,
	DialogPopup,
	DialogTitle,
} from "../../components/ui/dialog";
import {
	Menu,
	MenuItem,
	MenuPopup,
	MenuTrigger,
} from "../../components/ui/menu";
import { SegmentedPathInput } from "../../components/segmented-path-input";
import { UpdateValueButton } from "./update-value-button";

interface SecretTableRowProps {
	secret: SecretRow;
	appId: string;
	envId: string;
}

export function SecretTableRow({ secret, appId, envId }: SecretTableRowProps) {
	const remove = useDeleteSecret(appId, envId);
	const [bindingOpen, setBindingOpen] = useState(false);
	const [deleteOpen, setDeleteOpen] = useState(false);
	const path = secret.binding?.path ?? "";
	const mode = secret.binding?.mode;

	return (
		<TableRow className="border-b">
			<TableCell className="p-2 font-mono">{secret.name}</TableCell>
			<TableCell className="p-2">
				<div className="min-w-0">
					<p
						className={
							path ? "truncate font-mono text-sm" : "text-sm text-muted-foreground"
						}
						data-testid={`binding-path-${secret.name}`}
						title={path || undefined}
					>
						{path || "未设置"}
					</p>
					{mode ? (
						<p className="text-xs text-muted-foreground">{bindingModeLabel(mode)}</p>
					) : null}
				</div>
			</TableCell>
			<TableCell className="p-2">
				<SecretVersion
					name={secret.name}
					selected={secret.selected_version}
					latest={secret.latest_version}
				/>
			</TableCell>
			<TableCell className="p-2">
				<div className="flex items-center justify-end gap-1">
					<UpdateValueButton secret={secret} appId={appId} envId={envId} />
					<Menu>
						<MenuTrigger
							aria-label={`${secret.name} 更多操作`}
							render={<Button variant="outline" size="icon-sm" />}
						>
							<MoreVertical />
						</MenuTrigger>
						<MenuPopup align="end" side="bottom">
							<MenuItem closeOnClick onClick={() => setBindingOpen(true)}>
								编辑绑定
							</MenuItem>
							<MenuItem
								closeOnClick
								variant="destructive"
								onClick={() => setDeleteOpen(true)}
							>
								删除
							</MenuItem>
						</MenuPopup>
					</Menu>
				</div>
				<BindingEditorDialog
					secret={secret}
					appId={appId}
					envId={envId}
					open={bindingOpen}
					onOpenChange={setBindingOpen}
				/>
				<ConfirmDelete
					open={deleteOpen}
					onOpenChange={setDeleteOpen}
					title={`删除密钥 ${secret.name}？`}
					description="会删除该密钥及其版本。若环境仍绑定到节点组，需要先解除分配。"
					pending={remove.isPending}
					error={
						remove.isError ? String((remove.error as Error).message) : undefined
					}
					onConfirm={() =>
						remove.mutate(secret.id, {
							onSuccess: () => setDeleteOpen(false),
						})
					}
				/>
			</TableCell>
		</TableRow>
	);
}

function BindingEditorDialog({
	secret,
	appId,
	envId,
	open,
	onOpenChange,
}: {
	secret: SecretRow;
	appId: string;
	envId: string;
	open: boolean;
	onOpenChange: (open: boolean) => void;
}) {
	const update = useUpdateBinding(secret.id, appId, envId);
	const methods = useForm<BindingForm>({
		resolver: zodResolver(bindingSchema),
		defaultValues: bindingDefaults(secret),
	});

	return (
		<Dialog
			open={open}
			onOpenChange={(next) => {
				if (next) {
					methods.reset(bindingDefaults(secret));
				}
				onOpenChange(next);
			}}
		>
			<DialogPopup>
				<DialogHeader>
					<DialogTitle>编辑绑定</DialogTitle>
					<DialogDescription>
						把 {secret.name} 映射到 Materialized Bundle 中的相对路径和权限。
					</DialogDescription>
				</DialogHeader>
				<form
					className="contents"
					onSubmit={methods.handleSubmit((values) => {
						update.mutate(
							{ ...values, uid: 0, gid: 0 },
							{
								onSuccess: () => {
									toastSuccess("绑定已更新");
									onOpenChange(false);
								},
							},
						);
					})}
				>
					<DialogPanel>
						<div className="flex flex-col gap-4">
							<Field
								className="w-full"
								invalid={Boolean(methods.formState.errors.path)}
							>
								<FieldLabel>绑定路径</FieldLabel>
								<Controller
									name="path"
									control={methods.control}
									render={({ field }) => (
										<SegmentedPathInput
											value={field.value}
											onChange={field.onChange}
											onBlur={field.onBlur}
											name={field.name}
											testId={`binding-${secret.name}`}
										/>
									)}
								/>
								{methods.formState.errors.path && (
									<FieldError>{methods.formState.errors.path.message}</FieldError>
								)}
							</Field>
							<Field className="w-full">
								<FieldLabel>权限</FieldLabel>
								<Controller
									name="mode"
									control={methods.control}
									render={({ field }) => (
										<Select value={field.value} onValueChange={field.onChange}>
											<SelectTrigger
												className="w-full"
												data-testid={`mode-${secret.name}`}
											>
												<SelectValue>
													{(value) => bindingModeLabel(String(value ?? ""))}
												</SelectValue>
											</SelectTrigger>
											<SelectContent alignItemWithTrigger={false}>
												{SAFE_BINDING_MODES.map((item) => (
													<SelectItem key={item} value={item}>
														{BINDING_MODE_LABELS[item]}
													</SelectItem>
												))}
											</SelectContent>
										</Select>
									)}
								/>
							</Field>
							{update.isError && (
								<p className="text-sm text-red-500">
									{String((update.error as Error).message)}
								</p>
							)}
						</div>
					</DialogPanel>
					<DialogFooter>
						<DialogClose render={<Button variant="ghost" />}>取消</DialogClose>
						<Button
							type="submit"
							loading={update.isPending}
							disabled={!methods.formState.isDirty}
						>
							保存
						</Button>
					</DialogFooter>
				</form>
			</DialogPopup>
		</Dialog>
	);
}

function SecretVersion({
	name,
	selected,
	latest,
}: {
	name: string;
	selected: number;
	latest: number;
}) {
	const behind = selected !== latest;
	return (
		<div className="min-w-0" data-testid={`version-${name}`}>
			<p className="text-sm">v{selected}</p>
			{behind ? (
				<p className="text-xs text-muted-foreground">最新 v{latest}</p>
			) : null}
		</div>
	);
}

function bindingDefaults(secret: SecretRow): BindingForm {
	return {
		path: secret.binding?.path ?? "",
		mode: (secret.binding?.mode ?? DEFAULT_BINDING_MODE) as BindingForm["mode"],
	};
}
