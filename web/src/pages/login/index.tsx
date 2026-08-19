import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { LogIn } from "lucide-react";
import { useDocumentTitle } from "../../hooks/use-document-title";
import { useCompleteLogin, useLogin } from "../../hooks/auth/use-login";
import { useOIDCStatus } from "../../hooks/auth/use-oidc-status";
import {
	loginSchema,
	secondFactorSchema,
	type LoginForm,
	type SecondFactorForm,
} from "../../lib/constants/schemas";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiURL } from "../../lib/env";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Field, FieldError, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import {
	InputOTP,
	InputOTPGroup,
	InputOTPSlot,
} from "../../components/ui/input-otp";
import { ApiError } from "../../lib/api";

type SecondFactor = "totp" | "recovery";

function loginUnavailableMessage(title: string, available: boolean): string {
	return available ? `${title} 当前不可用` : `尚未配置 ${title}`;
}

function ExternalLoginOption({
	title,
	loginLabel,
	available,
	loginAvailable,
	href,
}: {
	title: string;
	loginLabel: string;
	available: boolean;
	loginAvailable: boolean;
	href: string;
}) {
	if (loginAvailable) {
		return (
			<Button
				variant="outline"
				className="w-full"
				onClick={() => window.location.assign(href)}
			>
				<LogIn />
				{loginLabel}
			</Button>
		);
	}
	return (
		<div className="space-y-2">
			<div className="flex items-center justify-between gap-3">
				<p className="text-sm font-medium">{title}</p>
				<Badge variant="secondary">不可用</Badge>
			</div>
			<p className="text-sm text-muted-foreground">
				{loginUnavailableMessage(title, available)}
			</p>
		</div>
	);
}

export function LoginPage() {
	const login = useLogin();
	const completeLogin = useCompleteLogin();
	const oidc = useOIDCStatus();
	const navigate = useNavigate();
	useDocumentTitle("登录");
	const [challengeActive, setChallengeActive] = useState(false);
	const [secondFactor, setSecondFactor] = useState<SecondFactor>("totp");
	const [challengeMessage, setChallengeMessage] = useState("");
	const loginForm = useForm<LoginForm>({ resolver: zodResolver(loginSchema) });
	const factorForm = useForm<SecondFactorForm>({
		resolver: zodResolver(secondFactorSchema),
	});

	useEffect(() => {
		if (challengeMessage && !challengeActive) loginForm.setFocus("username");
	}, [challengeActive, challengeMessage, loginForm]);

	const onLogin = (values: LoginForm) => {
		setChallengeMessage("");
		login.mutate(values, {
			onSuccess: (result) => {
				if (result.code === "second_factor_required") {
					setChallengeActive(true);
					return;
				}
				navigate("/dashboard/overview", { replace: true });
			},
		});
	};

	const onSecondFactor = (values: SecondFactorForm) => {
		completeLogin.mutate(
			secondFactor === "totp"
				? { totp_code: values.totp_code }
				: { recovery_code: values.recovery_code },
			{
				onSuccess: () => navigate("/dashboard/overview", { replace: true }),
				onError: (error) => {
					if (error instanceof ApiError && error.code === "challenge_expired") {
						setChallengeActive(false);
						setChallengeMessage("登录验证已过期，请重新输入用户名和密码。");
						factorForm.reset();
					}
				},
			},
		);
	};

	if (challengeActive) {
		return (
			<div className="space-y-4" aria-live="polite">
				<div>
					<h1 className="text-xl font-bold">验证第二因子</h1>
					<p className="mt-1 text-sm text-muted-foreground">
						密码已验证。请输入动态验证码或一个未使用的恢复码。
					</p>
				</div>
				<div
					className="flex gap-1 border-b"
					role="tablist"
					aria-label="第二因子类型"
				>
					{(["totp", "recovery"] as const).map((factor) => (
						<button
							key={factor}
							type="button"
							role="tab"
							aria-selected={secondFactor === factor}
							className={`border-b-2 px-3 py-2 text-sm ${
								secondFactor === factor
									? "border-foreground font-medium"
									: "border-transparent text-muted-foreground"
							}`}
							onClick={() => setSecondFactor(factor)}
							data-testid={`factor-${factor}`}
						>
							{factor === "totp" ? "动态验证码" : "恢复码"}
						</button>
					))}
				</div>
				<form
					onSubmit={factorForm.handleSubmit(onSecondFactor)}
					className="flex flex-col gap-3"
				>
					{secondFactor === "totp" ? (
						<Field invalid={Boolean(factorForm.formState.errors.totp_code)}>
							<FieldLabel>动态验证码</FieldLabel>
							<InputOTP
								autoFocus
								maxLength={6}
								data-testid="totp-code"
								value={factorForm.watch("totp_code") ?? ""}
								onChange={(value) =>
									factorForm.setValue("totp_code", value, {
										shouldValidate: true,
									})
								}
							>
								<InputOTPGroup>
									{Array.from({ length: 6 }).map((_, index) => (
										<InputOTPSlot key={index} index={index} />
									))}
								</InputOTPGroup>
							</InputOTP>
							{factorForm.formState.errors.totp_code && (
								<FieldError>{factorForm.formState.errors.totp_code.message}</FieldError>
							)}
						</Field>
					) : (
						<Field>
							<FieldLabel>恢复码</FieldLabel>
							<Input
								autoFocus
								placeholder="一次性恢复码"
								data-testid="recovery-code"
								{...factorForm.register("recovery_code")}
							/>
						</Field>
					)}
					{completeLogin.isError && (
						<p role="alert" className="text-sm text-destructive">
							{String((completeLogin.error as Error).message)}
						</p>
					)}
					<div className="flex gap-2">
						<Button
							variant="outline"
							onClick={() => {
								setChallengeActive(false);
								completeLogin.reset();
							}}
						>
							返回
						</Button>
						<Button
							className="flex-1"
							loading={completeLogin.isPending}
							type="submit"
						>
							继续
						</Button>
					</div>
				</form>
			</div>
		);
	}

	const oauthAvailable = oidc.data?.oauth?.available ?? false;
	const oauthLoginAvailable = oidc.data?.oauth?.login_available ?? false;
	const oidcAvailable =
		oidc.data?.oidc?.available ?? oidc.data?.available ?? false;
	const oidcLoginAvailable =
		oidc.data?.oidc?.login_available ?? oidc.data?.login_available ?? false;
	const passwordLoginAvailable = oidc.data?.password_login_available ?? true;
	const showExternalLogin = oidc.isSuccess && (oauthAvailable || oidcAvailable);

	return (
		<div
			className="space-y-4"
			data-testid="login-page"
			data-providers={oidc.isFetched ? "ready" : "loading"}
		>
			<h1 className="text-xl font-bold">登录</h1>
			{challengeMessage && (
				<p role="alert" className="text-sm text-destructive">
					{challengeMessage}
				</p>
			)}
			{!passwordLoginAvailable && (
				<p className="text-sm text-muted-foreground">
					已关闭密码登录，请使用 OAuth 或 OpenID Connect。
				</p>
			)}
			{passwordLoginAvailable && (
				<form
					onSubmit={loginForm.handleSubmit(onLogin)}
					className="flex flex-col gap-3"
				>
					<Field
						className="w-full"
						invalid={Boolean(loginForm.formState.errors.username)}
					>
						<FieldLabel>用户名</FieldLabel>
						<Input
							className="w-full"
							data-testid="username"
							{...loginForm.register("username")}
						/>
						{loginForm.formState.errors.username && (
							<FieldError>{loginForm.formState.errors.username.message}</FieldError>
						)}
					</Field>
					<Field
						className="w-full"
						invalid={Boolean(loginForm.formState.errors.password)}
					>
						<FieldLabel>密码</FieldLabel>
						<Input
							className="w-full"
							type="password"
							data-testid="password"
							{...loginForm.register("password")}
						/>
						{loginForm.formState.errors.password && (
							<FieldError>{loginForm.formState.errors.password.message}</FieldError>
						)}
					</Field>
					{login.isError && (
						<p role="alert" className="text-sm text-destructive">
							{String((login.error as Error).message)}
						</p>
					)}
					<Button className="w-full" loading={login.isPending} type="submit">
						登录
					</Button>
				</form>
			)}
			{showExternalLogin && (
				<>
					{passwordLoginAvailable && (
						<div className="flex items-center gap-3 text-xs text-muted-foreground">
							<span className="h-px flex-1 bg-border" />或
							<span className="h-px flex-1 bg-border" />
						</div>
					)}
					{oauthAvailable && (
						<ExternalLoginOption
							title="OAuth"
							loginLabel="使用 OAuth 登录"
							available={oauthAvailable}
							loginAvailable={oauthLoginAvailable}
							href={apiURL(`${API_PATHS.oauthLogin}?return_to=/dashboard/overview`)}
						/>
					)}
					{oidcAvailable && (
						<ExternalLoginOption
							title="OpenID Connect"
							loginLabel="使用 OpenID Connect 登录"
							available={oidcAvailable}
							loginAvailable={oidcLoginAvailable}
							href={apiURL(`${API_PATHS.oidcLogin}?return_to=/dashboard/overview`)}
						/>
					)}
				</>
			)}
		</div>
	);
}

export default LoginPage;
