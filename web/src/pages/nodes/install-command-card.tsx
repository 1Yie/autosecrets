import { useState } from "react";
import { Check, Copy, Terminal } from "lucide-react";
import { useInstallCommand } from "../../hooks/fleet/use-install-command";
import type { ManagedNode } from "../../hooks/fleet/use-nodes";
import { Button } from "../../components/ui/button";
import { ScrollArea } from "../../components/ui/scroll-area";
import { Skeleton } from "../../components/ui/skeleton";
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

export function InstallCommandCard({ node }: { node: ManagedNode }) {
	const install = useInstallCommand(node.id);
	const [open, setOpen] = useState(false);
	const [copied, setCopied] = useState(false);

	const generate = () => {
		install.mutate(node.bundle_dir ? { bundle_dir: node.bundle_dir } : {});
	};

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
				if (next && !open) generate();
				setOpen(next);
				if (next) return;
				setCopied(false);
				install.reset();
			}}
		>
			<DialogTrigger
				render={
					<Button variant="outline" size="sm" data-testid={`connect-${node.id}`} />
				}
			>
				生成连接
			</DialogTrigger>
			<DialogPopup>
				<DialogHeader>
					<DialogTitle>生成连接</DialogTitle>
					<DialogDescription>
						为「{node.name}」生成一次性安装命令。令牌仅显示一次，10 分钟后过期。
					</DialogDescription>
				</DialogHeader>
				<DialogPanel>
					<div className="flex flex-col gap-3">
						{install.isError && (
							<p className="text-sm text-red-500">
								{String((install.error as Error).message)}
							</p>
						)}
						{install.isPending && <Skeleton className="h-28 w-full" />}
						{install.data && (
							<div className="overflow-hidden rounded-lg border bg-zinc-950 dark:bg-black/70">
								<div className="flex flex-wrap items-center justify-between gap-2 border-b border-white/10 px-3 py-2">
									<p className="flex min-w-0 items-center gap-1.5 font-mono text-xs text-zinc-400">
										<Terminal aria-hidden="true" className="size-3.5 shrink-0" />
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
					<DialogClose render={<Button variant="ghost" />}>关闭</DialogClose>
				</DialogFooter>
			</DialogPopup>
		</Dialog>
	);
}
