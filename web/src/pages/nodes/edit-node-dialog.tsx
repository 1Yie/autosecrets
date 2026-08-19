import { useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useUpdateNode } from "../../hooks/fleet/use-update-node";
import type { ManagedNode } from "../../hooks/fleet/use-nodes";
import { bundleDirSchema, nameSchema } from "../../lib/constants/schemas";
import {
	POLL_INTERVAL_OPTIONS,
	pollIntervalLabel,
} from "../../lib/constants/poll-interval";
import { Button } from "../../components/ui/button";
import { Field, FieldError, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "../../components/ui/select";
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
import { toastSuccess } from "../../lib/toast";

const editNodeSchema = z.object({
	name: nameSchema,
	bundle_dir: bundleDirSchema,
	poll_interval_seconds: z.number().int().min(5).max(86400),
});

type EditNodeForm = z.infer<typeof editNodeSchema>;

function pollOptions(current: number): number[] {
	if (
		POLL_INTERVAL_OPTIONS.includes(
			current as (typeof POLL_INTERVAL_OPTIONS)[number],
		)
	) {
		return [...POLL_INTERVAL_OPTIONS];
	}
	return [...POLL_INTERVAL_OPTIONS, current].sort((a, b) => a - b);
}

export function EditNodeDialog({ node }: { node: ManagedNode }) {
	const update = useUpdateNode(node.id);
	const [open, setOpen] = useState(false);
	const defaults = (): EditNodeForm => ({
		name: node.name,
		bundle_dir: node.bundle_dir,
		poll_interval_seconds: node.poll_interval_seconds,
	});
	const {
		register,
		handleSubmit,
		reset,
		control,
		formState: { errors },
	} = useForm<EditNodeForm>({
		resolver: zodResolver(editNodeSchema),
		values: defaults(),
	});

	return (
		<Dialog
			open={open}
			onOpenChange={(next) => {
				setOpen(next);
				if (!next) {
					reset(defaults());
					update.reset();
				}
			}}
		>
			<DialogTrigger
				render={
					<Button variant="outline" size="sm" data-testid={`edit-${node.id}`} />
				}
			>
				修改
			</DialogTrigger>
			<DialogPopup>
				<DialogHeader>
					<DialogTitle>修改服务器</DialogTitle>
					<DialogDescription>更新名称、部署路径或同步频率。</DialogDescription>
				</DialogHeader>
				<form
					className="contents"
					onSubmit={handleSubmit((values) => {
						update.mutate(
							{
								name: values.name,
								bundle_dir: values.bundle_dir,
								poll_interval_seconds: values.poll_interval_seconds,
							},
							{
								onSuccess: () => {
									setOpen(false);
									toastSuccess("服务器已更新");
								},
							},
						);
					})}
				>
					<DialogPanel>
						<div className="flex flex-col gap-3">
							<Field className="w-full" invalid={Boolean(errors.name)}>
								<FieldLabel htmlFor={`edit-node-name-${node.id}`}>
									服务器名称
								</FieldLabel>
								<Input
									id={`edit-node-name-${node.id}`}
									className="w-full"
									data-testid={`edit-node-name-${node.id}`}
									{...register("name")}
								/>
								{errors.name && <FieldError>{errors.name.message}</FieldError>}
							</Field>
							<Field className="w-full" invalid={Boolean(errors.bundle_dir)}>
								<FieldLabel htmlFor={`edit-node-bundle-${node.id}`}>
									部署路径
								</FieldLabel>
								<Input
									id={`edit-node-bundle-${node.id}`}
									className="w-full"
									data-testid={`edit-node-bundle-${node.id}`}
									{...register("bundle_dir")}
								/>
								{errors.bundle_dir && (
									<FieldError>{errors.bundle_dir.message}</FieldError>
								)}
							</Field>
							<Controller
								name="poll_interval_seconds"
								control={control}
								render={({ field }) => (
									<Field
										className="w-full"
										invalid={Boolean(errors.poll_interval_seconds)}
									>
										<FieldLabel>更新频率</FieldLabel>
										<Select
											value={String(field.value)}
											onValueChange={(value) =>
												field.onChange(Number(value ?? field.value))
											}
										>
											<SelectTrigger
												className="w-full"
												data-testid={`edit-poll-interval-${node.id}`}
											>
												<SelectValue>{pollIntervalLabel(field.value)}</SelectValue>
											</SelectTrigger>
											<SelectContent>
												{pollOptions(node.poll_interval_seconds).map((seconds) => (
													<SelectItem key={seconds} value={String(seconds)}>
														{pollIntervalLabel(seconds)}
													</SelectItem>
												))}
											</SelectContent>
										</Select>
										{errors.poll_interval_seconds && (
											<FieldError>{errors.poll_interval_seconds.message}</FieldError>
										)}
									</Field>
								)}
							/>
							{update.isError && (
								<p className="text-sm text-red-500">
									{String((update.error as Error).message)}
								</p>
							)}
						</div>
					</DialogPanel>
					<DialogFooter>
						<DialogClose render={<Button variant="ghost" />}>取消</DialogClose>
						<Button variant="default" type="submit" disabled={update.isPending}>
							保存
						</Button>
					</DialogFooter>
				</form>
			</DialogPopup>
		</Dialog>
	);
}
