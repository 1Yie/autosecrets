import { useState } from "react";
import { KeyRound, Link2, ShieldCheck, ShieldOff } from "lucide-react";
import { MFAEnrollmentSteps, type EnrollmentContext } from "../../components/mfa-enrollment-steps";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Field, FieldLabel } from "../../components/ui/field";
import {
  useAuthenticationSecurity,
  useDeleteOIDCBinding,
  useDisableTOTP,
  useStartOIDCBinding,
  useStartTOTPEnrollment,
  type CredentialProof,
} from "../../hooks/auth/use-security";

export function SecurityPage() {
  const security = useAuthenticationSecurity();
  const startTOTP = useStartTOTPEnrollment();
  const disableTOTP = useDisableTOTP();
  const startBinding = useStartOIDCBinding();
  const deleteBinding = useDeleteOIDCBinding();
  const [enrollment, setEnrollment] = useState<EnrollmentContext | null>(null);
  const [localProof, setLocalProof] = useState<CredentialProof>({ password: "" });
  const [oidcProof, setOIDCProof] = useState<CredentialProof>({ password: "" });

  if (security.isLoading) return <p className="text-sm text-muted-foreground">正在加载安全设置...</p>;
  if (security.isError || !security.data) {
    return <p role="alert" className="text-sm text-destructive">无法加载安全设置</p>;
  }
  if (enrollment) {
    return (
      <div className="max-w-lg">
        <MFAEnrollmentSteps
          enrollment={enrollment}
          onDone={() => {
            setEnrollment(null);
            void security.refetch();
          }}
        />
      </div>
    );
  }

  const totpRequired = security.data.totp_login_required;
  const oidc = security.data.oidc;

  return (
    <div className="max-w-3xl space-y-8">
      <header>
        <h1 className="text-xl font-semibold">登录安全</h1>
        <p className="mt-1 text-sm text-muted-foreground">管理本地登录验证和 External Identity Binding。</p>
      </header>

      <section className="space-y-4 border-t pt-6" aria-labelledby="local-login-heading">
        <div className="flex items-start justify-between gap-4">
          <div className="flex gap-3">
            {totpRequired ? <ShieldCheck className="mt-0.5 size-5" /> : <ShieldOff className="mt-0.5 size-5" />}
            <div>
              <h2 id="local-login-heading" className="font-medium">本地登录需要 TOTP</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                {totpRequired ? "用户名和密码验证后还需要动态验证码或恢复码。" : "当前本地登录只需要用户名和密码。"}
              </p>
              {totpRequired && <p className="mt-1 text-xs text-muted-foreground">停用会删除 TOTP 和恢复码，并撤销其他 Session。</p>}
            </div>
          </div>
          <span className="text-sm font-medium">{totpRequired ? "已启用" : "已停用"}</span>
        </div>
        <CredentialProofFields
          idPrefix="local"
          value={localProof}
          onChange={setLocalProof}
          requireTOTP={totpRequired}
          passwordLabel={totpRequired ? "当前密码" : "输入当前密码以启用"}
        />
        {(startTOTP.isError || disableTOTP.isError) && (
          <p role="alert" className="text-sm text-destructive">{String((startTOTP.error ?? disableTOTP.error) as Error)}</p>
        )}
        {totpRequired ? (
          <Button
            variant="destructive-outline"
            loading={disableTOTP.isPending}
            disabled={!localProof.password || !localProof.totp_code}
            onClick={() => disableTOTP.mutate(localProof)}
          >
            <ShieldOff />停用本地 TOTP
          </Button>
        ) : (
          <Button
            loading={startTOTP.isPending}
            disabled={!localProof.password}
            onClick={() => startTOTP.mutate(localProof.password, { onSuccess: setEnrollment })}
          >
            <KeyRound />启用本地 TOTP
          </Button>
        )}
      </section>

      <section className="space-y-4 border-t pt-6" aria-labelledby="oidc-heading">
        <div className="flex items-start justify-between gap-4">
          <div className="flex gap-3">
            <Link2 className="mt-0.5 size-5" />
            <div>
              <h2 id="oidc-heading" className="font-medium">OpenID Connect</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                {oidc.bound
                  ? `已绑定 ${oidc.display_name || oidc.issuer || "External Identity"}`
                  : oidc.available ? "Provider 配置可用，尚未绑定身份。" : "Provider 配置不可用。"}
              </p>
              {oidc.bound && <p className="mt-1 text-xs text-muted-foreground">解除绑定会撤销 OIDC 创建的 Session 和其他浏览器 Session。</p>}
            </div>
          </div>
          <span className="text-sm font-medium">{oidc.bound ? "已绑定" : oidc.available ? "未绑定" : "不可用"}</span>
        </div>
        {oidc.configuration_error && (
          <p className="border-l-2 border-destructive pl-3 text-sm text-muted-foreground">{oidc.configuration_error}</p>
        )}
        {oidc.available && (
          <>
            <CredentialProofFields
              idPrefix="oidc"
              value={oidcProof}
              onChange={setOIDCProof}
              requireTOTP={totpRequired}
              passwordLabel="当前密码"
            />
            {(startBinding.isError || deleteBinding.isError) && (
              <p role="alert" className="text-sm text-destructive">{String((startBinding.error ?? deleteBinding.error) as Error)}</p>
            )}
            {oidc.bound ? (
              <Button
                variant="destructive-outline"
                loading={deleteBinding.isPending}
                disabled={!oidcProof.password || totpRequired && !oidcProof.totp_code}
                onClick={() => deleteBinding.mutate(oidcProof)}
              >
                解除绑定
              </Button>
            ) : (
              <Button
                loading={startBinding.isPending}
                disabled={!oidcProof.password || totpRequired && !oidcProof.totp_code}
                onClick={() => startBinding.mutate(oidcProof, { onSuccess: ({ authorization_url }) => window.location.assign(authorization_url) })}
              >
                <Link2 />绑定 External Identity
              </Button>
            )}
          </>
        )}
      </section>
    </div>
  );
}

function CredentialProofFields({
  idPrefix,
  value,
  onChange,
  requireTOTP,
  passwordLabel,
}: {
  idPrefix: "local" | "oidc";
  value: CredentialProof;
  onChange: (value: CredentialProof) => void;
  requireTOTP: boolean;
  passwordLabel: string;
}) {
  return (
    <div className="grid max-w-md gap-3 sm:grid-cols-2">
      <Field>
        <FieldLabel htmlFor={`${idPrefix}-password`}>{passwordLabel}</FieldLabel>
        <Input
          id={`${idPrefix}-password`}
          data-testid={`${idPrefix}-password`}
          type="password"
          autoComplete="current-password"
          value={value.password}
          onChange={(event) => onChange({ ...value, password: event.target.value })}
        />
      </Field>
      {requireTOTP && (
        <Field>
          <FieldLabel htmlFor={`${idPrefix}-totp`}>当前动态验证码</FieldLabel>
          <Input
            id={`${idPrefix}-totp`}
            data-testid={`${idPrefix}-totp`}
            type="text"
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={6}
            value={value.totp_code ?? ""}
            onChange={(event) => onChange({ ...value, totp_code: event.target.value })}
          />
        </Field>
      )}
    </div>
  );
}

export default SecurityPage;
