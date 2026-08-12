import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useBootstrap } from "../../hooks/auth/use-bootstrap";
import {
  useConfirmMFAEnrollment,
  useVerifyMFAEnrollment,
  type BootstrapEnrollment,
  type MFAVerified,
} from "../../hooks/auth/use-mfa-enrollment";
import { bootstrapSchema, mfaVerifySchema, type BootstrapForm, type MFAVerifyForm } from "../../lib/constants/schemas";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { InputOTP, InputOTPGroup, InputOTPSlot } from "../../components/ui/input-otp";

type Phase = "enroll" | "verify" | "recovery" | "done";

export function BootstrapPage() {
  const [phase, setPhase] = useState<Phase>("enroll");
  const [enrollment, setEnrollment] = useState<BootstrapEnrollment | null>(null);
  const [verified, setVerified] = useState<MFAVerified | null>(null);
  const [recoveryAcknowledged, setRecoveryAcknowledged] = useState(false);
  const [enrollError, setEnrollError] = useState("");

  const bootstrap = useBootstrap();
  const verify = useVerifyMFAEnrollment();
  const confirm = useConfirmMFAEnrollment();

  const enroll = useForm<BootstrapForm>({
    resolver: zodResolver(bootstrapSchema),
    mode: "onChange",
  });
  const verifyForm = useForm<MFAVerifyForm>({
    resolver: zodResolver(mfaVerifySchema),
    mode: "onChange",
  });

  const onSubmitEnroll = (values: BootstrapForm) => {
    setEnrollError("");
    bootstrap.mutate(values, {
      onSuccess: (data) => {
        setEnrollment(data as unknown as BootstrapEnrollment);
        setPhase("verify");
      },
      onError: (error) => setEnrollError((error as Error).message),
    });
  };

  const onSubmitVerify = (values: MFAVerifyForm) => {
    if (!enrollment) return;
    verify.mutate(
      { enrollment_token: enrollment.enrollment_token, totp_code: values.totp_code },
      {
        onSuccess: (data) => {
          setVerified(data);
          setPhase("recovery");
        },
      },
    );
  };

  if (phase === "verify" && enrollment) {
    return (
      <div className="space-y-4">
        <h1 className="text-xl font-bold">验证动态验证码</h1>
        <p className="text-sm opacity-70">
          在身份验证器中添加以下 TOTP 条目（用户名：{enrollment.username}），然后输入当前 6 位动态码。
        </p>
        <code className="block break-all rounded border p-3 text-xs opacity-80">
          {enrollment.totp_uri}
        </code>
        <form onSubmit={verifyForm.handleSubmit(onSubmitVerify)} className="space-y-3">
          <InputOTP
            maxLength={6}
            data-testid="totp-code"
            value={verifyForm.watch("totp_code") ?? ""}
            onChange={(value) => verifyForm.setValue("totp_code", value, { shouldValidate: true })}
          >
            <InputOTPGroup>
              {Array.from({ length: 6 }).map((_, i) => (
                <InputOTPSlot key={i} index={i} />
              ))}
            </InputOTPGroup>
          </InputOTP>
          {verifyForm.formState.errors.totp_code && (
            <p className="text-sm text-red-500">{verifyForm.formState.errors.totp_code.message}</p>
          )}
          {verify.isError && (
            <p className="text-sm text-red-500">{String((verify.error as Error).message)}</p>
          )}
          <Button variant="default" className="w-full" disabled={verify.isPending} type="submit">
            验证
          </Button>
        </form>
      </div>
    );
  }

  if (phase === "recovery" && verified) {
    return (
      <div className="space-y-4">
        <h1 className="text-xl font-bold">保存恢复码</h1>
        <p className="text-sm opacity-70">
          以下恢复码仅显示一次。请立即保存到离线位置；每张恢复码只能使用一次。
        </p>
        <ul className="rounded border p-3 font-mono text-sm" data-testid="recovery-codes">
          {verified.recovery_codes.map((code) => (
            <li key={code}>{code}</li>
          ))}
        </ul>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            data-testid="recovery-ack"
            checked={recoveryAcknowledged}
            onChange={(event) => setRecoveryAcknowledged(event.target.checked)}
          />
          我已保存恢复码
        </label>
        {confirm.isError && (
          <p className="text-sm text-red-500">{String((confirm.error as Error).message)}</p>
        )}
        <Button
          variant="default"
          className="w-full"
          disabled={!recoveryAcknowledged || confirm.isPending}
          onClick={() =>
            confirm.mutate(
              { confirmation_token: verified.confirmation_token },
              { onSuccess: () => setPhase("done") },
            )
          }
        >
          完成注册
        </Button>
      </div>
    );
  }

  if (phase === "done") {
    return (
      <div className="space-y-4">
        <h1 className="text-xl font-bold">注册完成</h1>
        <p className="text-sm opacity-70">现在可以使用用户名、密码和动态验证码登录。</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold">初始化 AutoSecrets</h1>
      <p className="text-sm opacity-70">
        从 Core 日志粘贴一次性初始化码，创建组织与首位管理员。注册分两步完成：先创建待激活身份，再绑定
        TOTP 并确认一次性恢复码。
      </p>
      <form onSubmit={enroll.handleSubmit(onSubmitEnroll)} className="space-y-3">
        <Input className="w-full" placeholder="初始化码" data-testid="code" {...enroll.register("code")} />
        {enroll.formState.errors.code && (
          <p className="text-sm text-red-500">{enroll.formState.errors.code.message}</p>
        )}
        <Input
          className="w-full"
          placeholder="组织名称"
          data-testid="organization-name"
          {...enroll.register("organization_name")}
        />
        {enroll.formState.errors.organization_name && (
          <p className="text-sm text-red-500">{enroll.formState.errors.organization_name.message}</p>
        )}
        <Input className="w-full" placeholder="用户名" data-testid="username" {...enroll.register("username")} />
        {enroll.formState.errors.username && (
          <p className="text-sm text-red-500">{enroll.formState.errors.username.message}</p>
        )}
        <Input
          className="w-full"
          type="password"
          placeholder="密码（至少 12 位）"
          data-testid="password"
          {...enroll.register("password")}
        />
        {enroll.formState.errors.password && (
          <p className="text-sm text-red-500">{enroll.formState.errors.password.message}</p>
        )}
        {enrollError && <p className="text-sm text-red-500">{enrollError}</p>}
        <Button
          variant="default"
          className="w-full"
          disabled={!enroll.formState.isValid || enroll.formState.isSubmitting || bootstrap.isPending}
          type="submit"
        >
          创建管理员
        </Button>
      </form>
    </div>
  );
}

export default BootstrapPage;
