import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import QRCode from "qrcode";
import {
  useConfirmMFAEnrollment,
  useVerifyMFAEnrollment,
} from "../hooks/auth/use-mfa-enrollment";
import { mfaVerifySchema, type MFAVerifyForm } from "../lib/constants/schemas";
import { Button } from "./ui/button";
import { Field, FieldLabel } from "./ui/field";
import { InputOTP, InputOTPGroup, InputOTPSlot } from "./ui/input-otp";
import {
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogPanel,
  DialogTitle,
} from "./ui/dialog";

export interface EnrollmentContext {
  username: string;
  enrollment_token: string;
  totp_uri: string;
}

/** TOTP verify -> one-time Recovery Code confirmation flow. */
export function MFAEnrollmentSteps({
  enrollment,
  onDone,
}: {
  enrollment: EnrollmentContext;
  onDone: () => void;
}) {
  const [verified, setVerified] = useState<{ confirmation_token: string; recovery_codes: string[] } | null>(null);
  const [recoveryAcknowledged, setRecoveryAcknowledged] = useState(false);
  const [qrUrl, setQrUrl] = useState("");
  const verify = useVerifyMFAEnrollment();
  const confirm = useConfirmMFAEnrollment();
  const verifyForm = useForm<MFAVerifyForm>({
    resolver: zodResolver(mfaVerifySchema),
    mode: "onChange",
  });

  useEffect(() => {
    let active = true;
    QRCode.toDataURL(enrollment.totp_uri, { width: 200, margin: 1 })
      .then((url) => {
        if (active) setQrUrl(url);
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, [enrollment.totp_uri]);

  if (verified) {
    return (
      <>
        <DialogHeader>
          <DialogTitle>保存恢复码</DialogTitle>
          <DialogDescription>
            以下恢复码仅显示一次。请立即保存到离线位置；每张恢复码只能使用一次。
          </DialogDescription>
        </DialogHeader>
        <DialogPanel>
          <div className="space-y-4">
            <ul
              className="grid gap-2 sm:grid-cols-2"
              data-testid="recovery-codes"
            >
              {verified.recovery_codes.map((code) => (
                <li key={code} className="rounded-md border bg-muted/40 px-3 py-2 font-mono text-sm">
                  {code}
                </li>
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
          </div>
        </DialogPanel>
        <DialogFooter>
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
        </DialogFooter>
      </>
    );
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>验证动态验证码</DialogTitle>
      </DialogHeader>
      <form
        onSubmit={verifyForm.handleSubmit((values) =>
          verify.mutate(
            { enrollment_token: enrollment.enrollment_token, totp_code: values.totp_code },
            { onSuccess: setVerified },
          ),
        )}
        className="contents"
      >
        <DialogPanel>
          <div className="space-y-4">
            <div className="flex justify-center">
              {qrUrl ? (
                <img
                  src={qrUrl}
                  alt="TOTP 二维码"
                  className="h-48 w-48 rounded-lg border bg-white p-2"
                  data-testid="totp-qr"
                />
              ) : (
                <div className="h-48 w-48 animate-pulse rounded-lg border bg-muted" />
              )}
            </div>
            <details className="text-xs text-muted-foreground">
              <summary className="cursor-pointer">无法扫码？手动输入</summary>
              <code className="mt-2 block break-all rounded-md border bg-muted/40 p-2 font-mono">
                {enrollment.totp_uri}
              </code>
            </details>
            <Field>
              <FieldLabel htmlFor="mfa-enrollment-totp">动态验证码</FieldLabel>
              <InputOTP
                id="mfa-enrollment-totp"
                maxLength={6}
                pushPasswordManagerStrategy="none"
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
            </Field>
            {verify.isError && (
              <p className="text-sm text-red-500">{String((verify.error as Error).message)}</p>
            )}
          </div>
        </DialogPanel>
        <DialogFooter>
          <Button variant="default" className="w-full" disabled={verify.isPending} type="submit">
            验证
          </Button>
        </DialogFooter>
      </form>
    </>
  );
}
