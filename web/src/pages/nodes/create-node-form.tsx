import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useCreateNode } from "../../hooks/fleet/use-create-node";
import { bundleDirSchema, nameSchema } from "../../lib/constants/schemas";
import { Button } from "../../components/ui/button";
import { Field, FieldError, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
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

const createNodeSchema = z.object({
	name: nameSchema,
	bundle_dir: bundleDirSchema,
});

type CreateNodeFormValues = z.infer<typeof createNodeSchema>;

export function CreateNodeForm() {
	const createNode = useCreateNode();
	const [open, setOpen] = useState(false);
	const {
		register,
		handleSubmit,
		reset,
		formState: { errors },
	} = useForm<CreateNodeFormValues>({
		resolver: zodResolver(createNodeSchema),
		defaultValues: { name: "", bundle_dir: "~/.autosecrets" },
	});

	return (
		<Dialog
			open={open}
			onOpenChange={(next) => {
				setOpen(next);
				if (!next) {
					reset();
					createNode.reset();
				}
			}}
		>
			<DialogTrigger render={<Button variant="default" />}>
				添加服务器
			</DialogTrigger>
			<DialogPopup>
				<DialogHeader>
					<DialogTitle>添加服务器</DialogTitle>
					<DialogDescription>
						先登记一台服务器。登记后可在列表里生成安装命令。
					</DialogDescription>
				</DialogHeader>
				<form
					className="contents"
					onSubmit={handleSubmit((values) => {
						createNode.mutate(
							{
								name: values.name,
								bundle_dir: values.bundle_dir || undefined,
							},
							{
								onSuccess: () => {
									setOpen(false);
									reset();
									toastSuccess("服务器已添加");
								},
							},
						);
					})}
				>
					<DialogPanel>
						<div className="flex flex-col gap-3">
							<Field className="w-full" invalid={Boolean(errors.name)}>
								<FieldLabel htmlFor="node-name">服务器名称</FieldLabel>
								<Input
									id="node-name"
									className="w-full"
									placeholder="如 web-1"
									data-testid="node-name"
									{...register("name")}
								/>
								{errors.name && <FieldError>{errors.name.message}</FieldError>}
							</Field>
							<Field className="w-full" invalid={Boolean(errors.bundle_dir)}>
								<FieldLabel htmlFor="node-bundle-dir">部署路径</FieldLabel>
								<Input
									id="node-bundle-dir"
									className="w-full"
									data-testid="node-bundle-dir"
									{...register("bundle_dir")}
								/>
								{errors.bundle_dir && (
									<FieldError>{errors.bundle_dir.message}</FieldError>
								)}
							</Field>
							{createNode.isError && (
								<p className="text-sm text-red-500">
									{String((createNode.error as Error).message)}
								</p>
							)}
						</div>
					</DialogPanel>
					<DialogFooter>
						<DialogClose render={<Button variant="ghost" />}>取消</DialogClose>
						<Button variant="default" type="submit" disabled={createNode.isPending}>
							添加
						</Button>
					</DialogFooter>
				</form>
			</DialogPopup>
		</Dialog>
	);
}
