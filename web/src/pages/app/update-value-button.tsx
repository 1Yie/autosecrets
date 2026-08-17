import { useState } from "react";
import { useCreateVersion } from "../../hooks/applications/use-create-version";
import { toastSuccess } from "../../lib/toast";
import type { SecretRow } from "../../hooks/applications/use-secrets";
import { Button } from "../../components/ui/button";
import { Field, FieldLabel } from "../../components/ui/field";
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

interface UpdateValueButtonProps {
	secret: SecretRow;
	appId: string;
	envId: string;
}

/** Single-field transient input; the guide allows useState for this. */
export function UpdateValueButton({
	secret,
	appId,
	envId,
}: UpdateValueButtonProps) {
	const updateValue = useCreateVersion(secret.id, appId, envId);
	const [value, setValue] = useState("");
	const [open, setOpen] = useState(false);

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={<Button variant="outline" size="sm" />}>
				轮换
			</DialogTrigger>
			<DialogPopup>
				<DialogHeader>
					<DialogTitle>轮换 {secret.name}</DialogTitle>
					<DialogDescription>
						将创建新版本；点右上角「发布」后节点同步新值。
					</DialogDescription>
				</DialogHeader>
				<div className="contents">
					<DialogPanel>
						<div className="flex flex-col gap-3">
							<Field className="w-full">
								<FieldLabel>新值</FieldLabel>
								<Input
									className="w-full"
									placeholder="密钥内容"
									value={value}
									onChange={(e) => setValue(e.target.value)}
									data-testid={`update-${secret.name}`}
								/>
							</Field>
							{updateValue.isError && (
								<p className="text-sm text-red-500">
									{String((updateValue.error as Error).message)}
								</p>
							)}
						</div>
					</DialogPanel>
					<DialogFooter>
						<DialogClose render={<Button variant="ghost" />}>取消</DialogClose>
						<Button
							variant="default"
							disabled={!value || updateValue.isPending}
							onClick={() => {
								updateValue.mutate(value, {
									onSuccess: () => {
										toastSuccess("已轮换");
										setOpen(false);
										setValue("");
									},
								});
							}}
						>
							轮换
						</Button>
					</DialogFooter>
				</div>
			</DialogPopup>
		</Dialog>
	);
}
