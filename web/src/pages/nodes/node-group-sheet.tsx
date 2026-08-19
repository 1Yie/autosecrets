import { Controller, useForm } from "react-hook-form";
import { useQueries } from "@tanstack/react-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { apiGet } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";
import { useCreateAssignment } from "../../hooks/fleet/use-create-assignment";
import { useUnassign } from "../../hooks/fleet/use-unassign";
import { useApplications } from "../../hooks/applications/use-applications";
import { useApplication } from "../../hooks/applications/use-application";
import { useAssignments } from "../../hooks/fleet/use-assignments";
import { useAddMember } from "../../hooks/fleet/use-add-member";
import { useRemoveMember } from "../../hooks/fleet/use-remove-member";
import { useNodes } from "../../hooks/fleet/use-nodes";
import type { NodeGroup } from "../../hooks/fleet/use-node-groups";
import { ConfirmDelete } from "../../components/confirm-delete";
import { Button } from "../../components/ui/button";
import { Field, FieldError, FieldLabel } from "../../components/ui/field";
import {
	Combobox,
	ComboboxChip,
	ComboboxChips,
	ComboboxChipsInput,
	ComboboxEmpty,
	ComboboxItem,
	ComboboxList,
	ComboboxPopup,
	ComboboxValue,
} from "../../components/ui/combobox";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "../../components/ui/select";
import {
	Sheet,
	SheetClose,
	SheetDescription,
	SheetFooter,
	SheetHeader,
	SheetPanel,
	SheetPopup,
	SheetTitle,
	SheetTrigger,
} from "../../components/ui/sheet";
import { Tabs, TabsContent, TabsList, TabsTab } from "../../components/ui/tabs";
import { toastSuccess } from "../../lib/toast";

interface NodeGroupSheetProps {
	group: NodeGroup;
}

interface NodeOption {
	label: string;
	value: string;
}

const assignmentSchema = z.object({
	application_id: z.string().min(1, "请选择应用"),
	environment_id: z.string().min(1, "请选择环境"),
});

function optionLabel(item: NodeOption) {
	return item.label;
}

function optionValue(item: NodeOption) {
	return item.value;
}

function optionsEqual(item: NodeOption, value: NodeOption) {
	return item.value === value.value;
}

/** Node Group management Sheet: 成员 tab (searchable multi-select) and
 * 分配 tab (Secret Bundle binding for this group). */
export function NodeGroupSheet({ group }: NodeGroupSheetProps) {
	const nodes = useNodes(200);
	const addMember = useAddMember(group.id);
	const removeMember = useRemoveMember(group.id);
	const unassign = useUnassign();
	const memberIds = group.member_ids;
	const options: NodeOption[] = nodes.items.map((node) => ({
		label: node.name,
		value: node.id,
	}));
	const memberValue = options.filter((option) =>
		memberIds.includes(option.value),
	);

	const onMembersChange = (next: NodeOption[] | null) => {
		const nextIds = new Set((next ?? []).map((option) => option.value));
		for (const id of memberIds) {
			if (!nextIds.has(id)) removeMember.mutate(id);
		}
		for (const option of next ?? []) {
			if (!memberIds.includes(option.value)) addMember.mutate(option.value);
		}
	};

	const createAssignment = useCreateAssignment();
	const applications = useApplications();
	const {
		control,
		handleSubmit,
		watch,
		setValue,
		reset,
		formState: { errors, isValid },
	} = useForm<{ application_id: string; environment_id: string }>({
		resolver: zodResolver(assignmentSchema),
		defaultValues: { application_id: "", environment_id: "" },
	});
	const appId = watch("application_id");
	const app = useApplication(appId);
	const assignments = useAssignments(100);
	const groupAssignments = assignments.items.filter(
		(a) => a.group_id === group.id,
	);
	const applicationNameById = Object.fromEntries(
		applications.items.map((item) => [item.id, item.name]),
	);
	const assignedAppIds = [
		...new Set(groupAssignments.map((assignment) => assignment.application_id)),
	];
	const assignedApps = useQueries({
		queries: assignedAppIds.map((id) => ({
			queryKey: ["application", id],
			queryFn: () =>
				apiGet<{
					id: string;
					name: string;
					environments: { id: string; name: string }[];
				}>(API_PATHS.application(id)),
			enabled: id.length > 0,
		})),
	});
	const environmentLabelById = Object.fromEntries(
		[
			...(app.data?.environments ?? []),
			...assignedApps.flatMap((query) => query.data?.environments ?? []),
		].map((env) => [env.id, env.name]),
	);
	const bind = (values: { application_id: string; environment_id: string }) => {
		createAssignment.mutate(
			{ group_id: group.id, ...values },
			{
				onSuccess: () => {
					reset();
					setValue("application_id", "");
				},
			},
		);
	};
	const memberError =
		(addMember.error as Error | null)?.message ??
		(removeMember.error as Error | null)?.message;

	return (
		<Sheet>
			<SheetTrigger
				render={<Button variant="outline" size="sm" />}
				aria-label={`管理 ${group.name}`}
			>
				管理
			</SheetTrigger>
			<SheetPopup>
				<SheetHeader>
					<SheetTitle>{group.name}</SheetTitle>
					<SheetDescription>
						管理组成员，以及这个组要下发的应用环境。
					</SheetDescription>
				</SheetHeader>
				<SheetPanel>
					<Tabs defaultValue="members">
						<TabsList>
							<TabsTab value="members">成员</TabsTab>
							<TabsTab value="assignments">分配</TabsTab>
						</TabsList>

						<TabsContent value="members" className="space-y-2 pt-2">
							{nodes.items.length === 0 ? (
								<p className="text-muted-foreground text-sm">
									还没有注册节点，先到节点列表添加服务器。
								</p>
							) : (
								<Combobox<NodeOption, true>
									isItemEqualToValue={optionsEqual}
									itemToStringLabel={optionLabel}
									itemToStringValue={optionValue}
									items={options}
									multiple
									onValueChange={onMembersChange}
									value={memberValue}
								>
									<ComboboxChips>
										<ComboboxValue>
											{(selected: NodeOption[]) => (
												<>
													{selected?.map((item) => (
														<ComboboxChip aria-label={item.label} key={item.value}>
															{item.label}
														</ComboboxChip>
													))}
													<ComboboxChipsInput
														aria-label="添加节点"
														placeholder={selected.length > 0 ? undefined : "搜索节点添加…"}
													/>
												</>
											)}
										</ComboboxValue>
									</ComboboxChips>
									<ComboboxPopup>
										<ComboboxEmpty>没有匹配的节点</ComboboxEmpty>
										<ComboboxList>
											{(item) => (
												<ComboboxItem key={item.value} value={item}>
													{item.label}
												</ComboboxItem>
											)}
										</ComboboxList>
									</ComboboxPopup>
								</Combobox>
							)}
							{memberError && (
								<p className="text-sm text-destructive">{memberError}</p>
							)}
						</TabsContent>

						<TabsContent value="assignments" className="space-y-3 pt-2">
							<form className="flex flex-col gap-3" onSubmit={handleSubmit(bind)}>
								<Controller
									name="application_id"
									control={control}
									render={({ field }) => (
										<Field className="w-full" invalid={Boolean(errors.application_id)}>
											<FieldLabel>应用</FieldLabel>
											<Select
												value={field.value}
												onValueChange={(value) => {
													field.onChange(value);
													setValue("environment_id", "");
												}}
											>
												<SelectTrigger
													className="w-full"
													data-testid="assignment-application"
												>
													<SelectValue placeholder="选择应用…">
														{(value) =>
															applicationNameById[String(value ?? "")] ?? "选择应用…"
														}
													</SelectValue>
												</SelectTrigger>
												<SelectContent>
													{applications.items.map((item) => (
														<SelectItem key={item.id} value={item.id}>
															{item.name}
														</SelectItem>
													))}
												</SelectContent>
											</Select>
											{errors.application_id && (
												<FieldError>{errors.application_id.message}</FieldError>
											)}
										</Field>
									)}
								/>
								<Controller
									name="environment_id"
									control={control}
									render={({ field }) => (
										<Field className="w-full" invalid={Boolean(errors.environment_id)}>
											<FieldLabel>环境</FieldLabel>
											<Select
												value={field.value}
												onValueChange={field.onChange}
												disabled={!appId}
											>
												<SelectTrigger
													className="w-full"
													data-testid="assignment-environment"
												>
													<SelectValue placeholder="选择环境…">
														{(value) =>
															environmentLabelById[String(value ?? "")] ?? "选择环境…"
														}
													</SelectValue>
												</SelectTrigger>
												<SelectContent>
													{app.data?.environments.map((env) => (
														<SelectItem key={env.id} value={env.id}>
															{env.name}
														</SelectItem>
													))}
												</SelectContent>
											</Select>
											{errors.environment_id && (
												<FieldError>{errors.environment_id.message}</FieldError>
											)}
										</Field>
									)}
								/>
								{createAssignment.isError && (
									<p className="text-sm text-destructive">
										{String((createAssignment.error as Error).message)}
									</p>
								)}
								<Button
									variant="default"
									type="submit"
									disabled={!isValid || createAssignment.isPending}
								>
									添加分配
								</Button>
							</form>

							<div className="space-y-1.5">
								<p className="text-muted-foreground text-sm">当前分配</p>
								{memberIds.length === 0 &&
									groupAssignments.some(
										(assignment) => assignment.status === "active",
									) && (
										<p className="text-sm text-destructive">
											成员已经清空，但这个组还有应用分配。删组前请先解除，清成员不会自动拆掉分配。
										</p>
									)}
								{groupAssignments.length === 0 ? (
									<p className="text-muted-foreground/72 text-sm">
										这个组还没有要下发的应用环境。
									</p>
								) : (
									groupAssignments.map((assignment) => (
										<div
											key={assignment.id}
											className="flex items-center justify-between gap-2 rounded-md border px-3 py-2 text-sm"
										>
											<span>
												{applicationNameById[assignment.application_id] ??
													assignment.application_id.slice(0, 8)}
												{" / "}
												{environmentLabelById[assignment.environment_id] ??
													assignment.environment_id.slice(0, 8)}
											</span>
											{assignment.status === "active" ? (
												<ConfirmDelete
													confirmLabel="解除"
													description="节点会停止接收这个应用环境的密钥。清成员不会拆掉分配；解除后 Agent 完成清理，才能删除节点组。"
													error={
														unassign.isError && unassign.variables === assignment.id
															? String((unassign.error as Error).message)
															: undefined
													}
													label="解除"
													pending={
														unassign.isPending && unassign.variables === assignment.id
													}
													title="解除这个分配？"
													onConfirm={() =>
														unassign.mutate(assignment.id, {
															onSuccess: () => toastSuccess("已开始解除分配"),
														})
													}
												/>
											) : (
												<span className="text-muted-foreground/72">卸载中</span>
											)}
										</div>
									))
								)}
							</div>
						</TabsContent>
					</Tabs>
				</SheetPanel>
				<SheetFooter>
					<SheetClose render={<Button variant="default" />}>完成</SheetClose>
				</SheetFooter>
			</SheetPopup>
		</Sheet>
	);
}
