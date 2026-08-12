import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useLogin } from "../../hooks/auth/use-login";
import { useResumeMFAEnrollment } from "../../hooks/auth/use-resume-mfa";
import { loginSchema, type LoginForm } from "../../lib/constants/schemas";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { InputOTP, InputOTPGroup, InputOTPSlot } from "../../components/ui/input-otp";
import { MFAEnrollmentSteps, type EnrollmentContext } from "../../components/mfa-enrollment-steps";
import { ApiError } from "../../lib/api";

type SecondFactor = "totp" | "recovery";

export function LoginPage() {
  const login = useLogin();
  const resume = useResumeMFAEnrollment();
  const [secondFactor, setSecondFactor] = useState<SecondFactor>("totp");
  const [enrollment, setEnrollment] = useState<EnrollmentContext | null>(null);
  const needsEnrollment = login.isError && login.error instanceof ApiError && login.error.code === "mfa_enrollment_required";
  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<LoginForm>({ resolver: zodResolver(loginSchema) });

  const onSubmit = (values: LoginForm) => {
    login.mutate({
      username: values.username,
      password: values.password,
      ...(secondFactor === "totp" ? { totp_code: values.totp_code } : { recovery_code: values.recovery_code }),
    });
  };

  if (enrollment) {
    return (
      <MFAEnrollmentSteps
        enrollment={enrollment}
        onDone={() => {
          setEnrollment(null);
          login.reset();
        }}
      />
    );
  }

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold">登录</h1>
      {needsEnrollment && (
        <div className="space-y-2 rounded border border-amber-300/40 bg-amber-50 p-3 text-sm dark:bg-amber-950/20">
          <p className="font-medium">此账号需要先完成 MFA 注册</p>
          <p className="opacity-70">验证密码后，请绑定身份验证器并保存一次性恢复码。</p>
          <Button
            variant="default"
            size="sm"
            disabled={resume.isPending}
            onClick={() => {
              const username = watch("username");
              const password = watch("password");
              resume.mutate(
                { username, password },
                { onSuccess: setEnrollment },
              );
            }}
            data-testid="resume-mfa"
          >
            开始注册
          </Button>
          {resume.isError && (
            <p className="text-sm text-red-500">{String((resume.error as Error).message)}</p>
          )}
        </div>
      )}
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-3">
        <Input className="w-full" placeholder="用户名" data-testid="username" {...register("username")} />
        {errors.username && <p className="text-sm text-red-500">{errors.username.message}</p>}
        <Input
          className="w-full"
          type="password"
          placeholder="密码"
          data-testid="password"
          {...register("password")}
        />
        {errors.password && <p className="text-sm text-red-500">{errors.password.message}</p>}
        <div className="flex gap-2 text-sm">
          <button
            type="button"
            className={secondFactor === "totp" ? "font-semibold" : "opacity-60"}
            onClick={() => setSecondFactor("totp")}
            data-testid="factor-totp"
          >
            动态验证码
          </button>
          <span className="opacity-30">|</span>
          <button
            type="button"
            className={secondFactor === "recovery" ? "font-semibold" : "opacity-60"}
            onClick={() => setSecondFactor("recovery")}
            data-testid="factor-recovery"
          >
            恢复码
          </button>
        </div>
        {secondFactor === "totp" ? (
          <>
            <InputOTP
              maxLength={6}
              data-testid="totp-code"
              value={watch("totp_code") ?? ""}
              onChange={(value) => setValue("totp_code", value, { shouldValidate: true })}
            >
              <InputOTPGroup>
                {Array.from({ length: 6 }).map((_, i) => (
                  <InputOTPSlot key={i} index={i} />
                ))}
              </InputOTPGroup>
            </InputOTP>
            {errors.totp_code && <p className="text-sm text-red-500">{errors.totp_code.message}</p>}
          </>
        ) : (
          <Input
            className="w-full"
            placeholder="恢复码（例如 ABCD-EFGH-JKLM）"
            data-testid="recovery-code"
            {...register("recovery_code")}
          />
        )}
        {login.isError && <p className="text-sm text-red-500">{String((login.error as Error).message)}</p>}
        <Button variant="default" className="w-full" disabled={isSubmitting || login.isPending} type="submit">
          登录
        </Button>
      </form>
    </div>
  );
}

export default LoginPage;
