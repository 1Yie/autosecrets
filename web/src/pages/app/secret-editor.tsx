import { useState } from "react";
import { KeyRound } from "lucide-react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useSecrets } from "../../hooks/applications/use-secrets";
import { useCreateSecret } from "../../hooks/applications/use-create-secret";
import { secretSchema, type SecretForm } from "../../lib/constants/schemas";
import { SecretTableRow } from "./binding-row";
import { Skeleton } from "../../components/ui/skeleton";
import { Button } from "../../components/ui/button";
import { Field, FieldError, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import {
	Table,
	TableBody,
	TableHead,
	TableHeader,
	TableRow,
} from "../../components/ui/table";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "../../components/ui/empty";
import { Frame } from "../../components/ui/frame";
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

interface SecretEditorProps {
	appId: string;
	envId: string;
}

export function SecretEditor({ appId, envId }: SecretEditorProps) {
	const secrets = useSecrets(appId, envId);
	const create = useCreateSecret(appId, envId);
	const [createOpen, setCreateOpen] = useState(false);
	const {
		register,
		handleSubmit,
		reset,
		formState: { errors },
	} = useForm<SecretForm>({ resolver: zodResolver(secretSchema) });

	return (
		<div className="space-y-6">
			<div className="flex items-center justify-between">
				<h2 className="font-semibold">密钥</h2>
				<Dialog open={createOpen} onOpenChange={setCreateOpen}>
					<DialogTrigger render={<Button variant="default" />}>
						添加密钥
					</DialogTrigger>
					<DialogPopup>
						<DialogHeader>
							<DialogTitle>添加密钥</DialogTitle>
							<DialogDescription>
								名称与值；发布后已分配的节点会同步该值。
							</DialogDescription>
						</DialogHeader>
						<form
							className="contents"
							onSubmit={handleSubmit((v) => {
								create.mutate(v, {
									onSuccess: () => {
										setCreateOpen(false);
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
											placeholder="例如 db_pass"
											data-testid="secret-name"
											{...register("name")}
										/>
										{errors.name && (
											<FieldError>{errors.name.message}</FieldError>
										)}
									</Field>
									<Field className="w-full" invalid={Boolean(errors.value)}>
										<FieldLabel>值</FieldLabel>
										<Input
											className="w-full"
											placeholder="密钥内容"
											data-testid="secret-value"
											{...register("value")}
										/>
										{errors.value && (
											<FieldError>{errors.value.message}</FieldError>
										)}
									</Field>
									{create.isError && (
										<p className="text-sm text-red-500">
											{String((create.error as Error).message)}
										</p>
									)}
								</div>
							</DialogPanel>
							<DialogFooter>
								<DialogClose render={<Button variant="ghost" />}>
									取消
								</DialogClose>
								<Button
									variant="default"
									type="submit"
									disabled={create.isPending}
								>
									添加
								</Button>
							</DialogFooter>
						</form>
					</DialogPopup>
				</Dialog>
			</div>

			{secrets.isLoading && <Skeleton className="h-24 w-full" />}
			{secrets.isError && (
				<p className="text-sm text-red-500">密钥列表加载失败</p>
			)}
			{secrets.isSuccess && (secrets.data?.length ?? 0) === 0 && (
				<Empty className="border-dashed" data-testid="secrets-empty">
					<EmptyHeader>
						<EmptyMedia variant="icon">
							<KeyRound aria-hidden="true" />
						</EmptyMedia>
						<EmptyTitle>还没有密钥</EmptyTitle>
						<EmptyDescription>
							添加一个密钥，再绑定路径并发布。
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			)}
			{(secrets.data?.length ?? 0) > 0 && (
				<Frame className="w-full">
					<Table variant="card" className="w-full text-left text-sm">
						<TableHeader>
							<TableRow className="border-b opacity-60">
								<TableHead className="p-2">名称</TableHead>
								<TableHead className="p-2">绑定路径</TableHead>
								<TableHead className="p-2">版本</TableHead>
								<TableHead className="p-2 text-right">操作</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{secrets.data?.map((s) => (
								<SecretTableRow
									key={s.id}
									secret={s}
									appId={appId}
									envId={envId}
								/>
							))}
						</TableBody>
					</Table>
				</Frame>
			)}
		</div>
	);
}
