import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  useConfirmMFAEnrollment,
  useVerifyMFAEnrollment,
} from "../hooks/auth/use-mfa-enrollment";
import { mfaVerifySchema, type MFAVerifyForm } from "../lib/constants/schemas";
import { Button } from "./ui/button";
import { InputOTP, InputOTPGroup, InputOTPSlot } from "./ui/input-otp";

export interface EnrollmentContext {
  username: string;
  enrollment_token: string;
  totp_uri: string;
}

/** Shared TOTP verify -> one-time Recovery Code confirmation flow, used by
 * Bootstrap and by the legacy-member enrollment resume on the login page. */
export function MFAEnrollmentSteps({
  enrollment,
  onDone,
}: {
  enrollment: EnrollmentContext;
  onDone: () => void;
}) {
  const [verified, setVerified] = useState<{ confirmation_token: string; recovery_codes: string[] } | null>(null);
  const [recoveryAcknowledged, setRecoveryAcknowledged] = useState(false);
  const verify = useVerifyMFAEnrollment();
  const confirm = useConfirmMFAEnrollment();
  const verifyForm = useForm<MFAVerifyForm>({
    resolver: zodResolver(mfaVerifySchema),
    mode: "onChange",
  });

  if (verified) {
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
              { onSuccess: onDone },
            )
          }
        >
          完成注册
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold">验证动态验证码</h1>
      <p className="text-sm opacity-70">
        在身份验证器中添加以下 TOTP 条目（用户名：{enrollment.username}），然后输入当前 6 位动态码。
      </p>
      <code className="block break-all rounded border p-3 text-xs opacity-80">
        {enrollment.totp_uri}
      </code>
      <form onSubmit={verifyForm.handleSubmit((values) =>
        verify.mutate(
          { enrollment_token: enrollment.enrollment_token, totp_code: values.totp_code },
          { onSuccess: setVerified },
        ),
      )} className="space-y-3">
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
