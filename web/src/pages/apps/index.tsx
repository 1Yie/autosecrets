import { useState } from "react";
import { Link } from "react-router-dom";
import { ChevronRight } from "lucide-react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useDocumentTitle } from "../../hooks/use-document-title";
import { useApplications } from "../../hooks/applications/use-applications";
import { useCreateApplication } from "../../hooks/applications/use-create-application";
import { useDeleteApplication } from "../../hooks/applications/use-delete-application";
import { ConfirmDelete } from "../../components/confirm-delete";
import { nameSchema } from "../../lib/constants/schemas";
import { z } from "zod";
import { Button } from "../../components/ui/button";
import { Field, FieldError, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import { Skeleton } from "../../components/ui/skeleton";
import { Frame } from "../../components/ui/frame";
import { TablePagination } from "../../components/table-pagination";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "../../components/ui/table";
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

const createAppSchema = z.object({ name: nameSchema });

export function AppsPage() {
	const apps = useApplications();
	useDocumentTitle("应用");
	const create = useCreateApplication();
	const remove = useDeleteApplication();
	const [open, setOpen] = useState(false);
	const {
		register,
		handleSubmit,
		reset,
		formState: { errors },
	} = useForm<{ name: string }>({ resolver: zodResolver(createAppSchema) });

	return (
		<div className="space-y-4">
			<div className="flex items-center justify-between">
				<h1 className="text-xl font-bold">应用</h1>
				<Dialog open={open} onOpenChange={setOpen}>
					<DialogTrigger render={<Button variant="default" />}>
						新建应用
					</DialogTrigger>
					<DialogPopup>
						<DialogHeader>
							<DialogTitle>新建应用</DialogTitle>
							<DialogDescription>应用是密钥的顶层分组。</DialogDescription>
						</DialogHeader>
						<form
							className="contents"
							onSubmit={handleSubmit((values) => {
								create.mutate(values.name, {
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
										<FieldLabel>名称</FieldLabel>
										<Input
											className="w-full"
											placeholder="例如 payments"
											data-testid="app-name"
											{...register("name")}
										/>
										{errors.name && <FieldError>{errors.name.message}</FieldError>}
									</Field>
									{create.isError && (
										<p className="text-sm text-red-500">
											{String((create.error as Error).message)}
										</p>
									)}
								</div>
							</DialogPanel>
							<DialogFooter>
								<DialogClose render={<Button variant="ghost" />}>取消</DialogClose>
								<Button variant="default" type="submit" disabled={create.isPending}>
									创建
								</Button>
							</DialogFooter>
						</form>
					</DialogPopup>
				</Dialog>
			</div>

			{apps.isLoading && (
				<div className="space-y-2">
					<Skeleton className="h-10 w-full" />
					<Skeleton className="h-10 w-full" />
				</div>
			)}
			{apps.isError && <p className="text-sm text-red-500">应用加载失败</p>}
			{apps.items.length === 0 && !apps.isLoading && (
				<p className="opacity-60">还没有应用，点右上角「新建应用」开始。</p>
			)}
			{apps.items.length > 0 && (
				<Frame className="w-full">
					<Table variant="card">
						<TableHeader>
							<TableRow className="hover:bg-transparent">
								<TableHead>名称</TableHead>
								<TableHead>创建时间</TableHead>
								<TableHead className="text-right">操作</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{apps.items.map((app) => (
								<TableRow key={app.id} className="hover:bg-muted/50">
									<TableCell>
										<Link
											to={`/dashboard/apps/${app.id}`}
											className="flex items-center gap-1.5 font-medium text-primary hover:underline"
										>
											{app.name}
											<ChevronRight className="size-4 opacity-50" />
										</Link>
									</TableCell>
									<TableCell className="text-muted-foreground tabular-nums">
										{new Date(app.created_at).toLocaleString()}
									</TableCell>
									<TableCell className="text-right">
										<ConfirmDelete
											label="删除"
											title={`删除应用 ${app.name}？`}
											description="会删除该应用下的环境和密钥。若仍有节点组绑定，需要先解除分配。"
											pending={remove.isPending}
											error={
												remove.isError ? String((remove.error as Error).message) : undefined
											}
											onConfirm={() => remove.mutate(app.id)}
										/>
									</TableCell>
								</TableRow>
							))}
						</TableBody>
					</Table>
					<TablePagination noun="应用" page={apps} />
				</Frame>
			)}
		</div>
	);
}

export default AppsPage;
