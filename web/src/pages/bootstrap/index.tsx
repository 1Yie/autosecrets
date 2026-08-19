import { useNavigate } from "react-router-dom";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useDocumentTitle } from "../../hooks/use-document-title";
import { useBootstrap } from "../../hooks/auth/use-bootstrap";
import {
	bootstrapSchema,
	type BootstrapForm,
} from "../../lib/constants/schemas";
import { Button } from "../../components/ui/button";
import { Field, FieldError, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";

export function BootstrapPage() {
	const navigate = useNavigate();
	const bootstrap = useBootstrap();
	useDocumentTitle("初始化");
	const form = useForm<BootstrapForm>({
		resolver: zodResolver(bootstrapSchema),
		mode: "onChange",
	});

	return (
		<div className="space-y-4">
			<h1 className="text-xl font-bold">初始化 AutoSecrets</h1>
			<p className="text-sm text-muted-foreground">
				从 Core 日志粘贴一次性初始化码，创建 Administrator。本地登录默认使用
				用户名与密码，之后可在设置中启用 TOTP。
			</p>
			<form
				onSubmit={form.handleSubmit((values) =>
					bootstrap.mutate(values, {
						onSuccess: () => navigate("/dashboard/overview", { replace: true }),
					}),
				)}
				className="flex flex-col gap-3"
			>
				<Field className="w-full" invalid={Boolean(form.formState.errors.code)}>
					<FieldLabel>初始化码</FieldLabel>
					<Input className="w-full" data-testid="code" {...form.register("code")} />
					{form.formState.errors.code && (
						<FieldError>{form.formState.errors.code.message}</FieldError>
					)}
				</Field>
				<Field className="w-full" invalid={Boolean(form.formState.errors.username)}>
					<FieldLabel>用户名</FieldLabel>
					<Input
						className="w-full"
						data-testid="username"
						{...form.register("username")}
					/>
					{form.formState.errors.username && (
						<FieldError>{form.formState.errors.username.message}</FieldError>
					)}
				</Field>
				<Field className="w-full" invalid={Boolean(form.formState.errors.password)}>
					<FieldLabel>密码</FieldLabel>
					<Input
						className="w-full"
						type="password"
						placeholder="至少 12 位"
						data-testid="password"
						{...form.register("password")}
					/>
					{form.formState.errors.password && (
						<FieldError>{form.formState.errors.password.message}</FieldError>
					)}
				</Field>
				{bootstrap.isError && (
					<p role="alert" className="text-sm text-destructive">
						{String((bootstrap.error as Error).message)}
					</p>
				)}
				<Button
					className="w-full"
					disabled={!form.formState.isValid}
					loading={bootstrap.isPending}
					type="submit"
				>
					创建管理员
				</Button>
			</form>
		</div>
	);
}

export default BootstrapPage;
