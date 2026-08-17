import { useState } from "react";
import { Check, Copy, Terminal } from "lucide-react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useInstallCommand } from "../../hooks/fleet/use-install-command";
import { nameSchema } from "../../lib/constants/schemas";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Field, FieldError, FieldLabel } from "../../components/ui/field";
import { ScrollArea } from "../../components/ui/scroll-area";
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

const commandFormSchema = z.object({
	name: nameSchema,
	bundle_dir: z
		.string()
		.trim()
		.refine(
			(path) => path === "" || path.startsWith("/") || path.startsWith("~/"),
			"部署路径必须是绝对路径或以 ~/ 开头",
		),
});

type CommandForm = z.infer<typeof commandFormSchema>;

export function InstallCommandCard() {
	const install = useInstallCommand();
	const [open, setOpen] = useState(false);
	const [copied, setCopied] = useState(false);
	const {
		register,
		handleSubmit,
		reset,
		formState: { errors },
	} = useForm<CommandForm>({
		resolver: zodResolver(commandFormSchema),
		defaultValues: { name: "node", bundle_dir: "~/.autosecrets" },
	});

	const copy = async () => {
		if (!install.data) return;
		await navigator.clipboard.writeText(install.data.command);
		setCopied(true);
		window.setTimeout(() => setCopied(false), 2000);
	};

	return (
		<Dialog
			open={open}
			onOpenChange={(next) => {
				setOpen(next);
				if (!next) {
					setCopied(false);
					reset();
					install.reset();
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
						生成一次性安装命令。在目标服务器上执行后，密钥会同步到部署路径。令牌仅显示一次，10
						分钟后过期。
					</DialogDescription>
				</DialogHeader>
				<form
					className="contents"
					onSubmit={handleSubmit((values) =>
						install.mutate({
							name: values.name,
							bundle_dir: values.bundle_dir || undefined,
						}),
					)}
				>
					<DialogPanel>
						<div className="flex flex-col gap-3">
							<Field className="w-full" invalid={Boolean(errors.name)}>
								<FieldLabel htmlFor="node-name">服务器名称</FieldLabel>
								<Input
									id="node-name"
									className="w-full"
									type="text"
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
									type="text"
									data-testid="node-bundle-dir"
									{...register("bundle_dir")}
								/>
								{errors.bundle_dir && (
									<FieldError>{errors.bundle_dir.message}</FieldError>
								)}
							</Field>
							{install.isError && (
								<p className="text-sm text-red-500">
									{String((install.error as Error).message)}
								</p>
							)}
							{install.data && (
								<div className="overflow-hidden rounded-lg border bg-zinc-950 dark:bg-black/70">
									<div className="flex flex-wrap items-center justify-between gap-2 border-b border-white/10 px-3 py-2">
										<p className="flex min-w-0 items-center gap-1.5 font-mono text-xs text-zinc-400">
											<Terminal
												aria-hidden="true"
												className="size-3.5 shrink-0"
											/>
											<span className="truncate">
												安装命令 · 过期时间{" "}
												{new Date(install.data.expires_at).toLocaleString()}
											</span>
										</p>
										<Button
											type="button"
											variant="outline"
											size="xs"
											className="border-white/20 bg-transparent text-zinc-200 hover:bg-white/10"
											onClick={copy}
										>
											{copied ? (
												<>
													<Check aria-hidden="true" className="size-3.5" />
													已复制
												</>
											) : (
												<>
													<Copy aria-hidden="true" className="size-3.5" />
													复制命令
												</>
											)}
										</Button>
									</div>
									<ScrollArea
										className="h-auto max-h-40 w-full"
										overscrollContain
										scrollbarGutter
									>
										<pre
											className="w-max min-w-full p-3 font-mono text-sm text-green-400"
											data-testid="install-command"
										>
											{install.data.command}
										</pre>
									</ScrollArea>
								</div>
							)}
						</div>
					</DialogPanel>
					<DialogFooter>
						<DialogClose render={<Button variant="ghost" />}>取消</DialogClose>
						<Button
							variant="default"
							type="submit"
							disabled={install.isPending}
						>
							生成
						</Button>
					</DialogFooter>
				</form>
			</DialogPopup>
		</Dialog>
	);
}
