import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useCreateNodeGroup } from "../../hooks/fleet/use-create-node-group";
import { nameSchema } from "../../lib/constants/schemas";
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

const groupSchema = z.object({ name: nameSchema });

export function CreateNodeGroupForm() {
	const createGroup = useCreateNodeGroup();
	const [open, setOpen] = useState(false);
	const {
		register,
		handleSubmit,
		reset,
		formState: { errors },
	} = useForm<{ name: string }>({
		resolver: zodResolver(groupSchema),
	});

	return (
		<Dialog
			open={open}
			onOpenChange={(next) => {
				setOpen(next);
				if (!next) {
					reset();
					createGroup.reset();
				}
			}}
		>
			<DialogTrigger render={<Button variant="default" />}>
				新建节点组
			</DialogTrigger>
			<DialogPopup>
				<DialogHeader>
					<DialogTitle>新建节点组</DialogTitle>
					<DialogDescription>
						将托管节点分组，以便按组下发密钥版本。
					</DialogDescription>
				</DialogHeader>
				<form
					className="contents"
					onSubmit={handleSubmit((values) => {
						createGroup.mutate(values.name, {
							onSuccess: () => {
								setOpen(false);
								reset();
							},
						});
					})}
				>
					<DialogPanel>
						<div className="flex flex-col gap-3">
							<Field className="w-full" invalid={Boolean(errors.name)}>
								<FieldLabel htmlFor="node-group-name">名称</FieldLabel>
								<Input
									id="node-group-name"
									className="w-full"
									placeholder="例如 web"
									data-testid="node-group-name"
									{...register("name")}
								/>
								{errors.name && <FieldError>{errors.name.message}</FieldError>}
							</Field>
							{createGroup.isError && (
								<p className="text-sm text-red-500">
									{String((createGroup.error as Error).message)}
								</p>
							)}
						</div>
					</DialogPanel>
					<DialogFooter>
						<DialogClose render={<Button variant="ghost" />}>取消</DialogClose>
						<Button
							variant="default"
							type="submit"
							disabled={createGroup.isPending}
						>
							创建
						</Button>
					</DialogFooter>
				</form>
			</DialogPopup>
		</Dialog>
	);
}
