import { useEffect, useId, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  Info,
  KeyRound,
  Link2,
  Lock,
  Settings2,
  ShieldCheck,
  ShieldOff,
  UserRound,
} from "lucide-react";
import { AboutAutosecrets } from "../../components/about-autosecrets";
import { useDocumentTitle } from "../../hooks/use-document-title";
import { useMe } from "../../hooks/auth/use-me";
import {
  MFAEnrollmentSteps,
  type EnrollmentContext,
} from "../../components/mfa-enrollment-steps";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Field, FieldError, FieldLabel } from "../../components/ui/field";
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
} from "../../components/ui/input-otp";
import { Badge } from "../../components/ui/badge";
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
} from "../../components/ui/dialog";
import { Label } from "../../components/ui/label";
import { Switch } from "../../components/ui/switch";
import { Tabs, TabsList, TabsPanel, TabsTab } from "../../components/ui/tabs";
import {
  Frame,
  FrameHeader,
  FramePanel,
  FrameTitle,
} from "../../components/ui/frame";
import {
  useAuthenticationSecurity,
  useChangePassword,
  useChangeUsername,
  useDeleteOAuthBinding,
  useDeleteOIDCBinding,
  useDisableTOTP,
  useSetPasswordLogin,
  useStartOAuthBinding,
  useStartOIDCBinding,
  useStartTOTPEnrollment,
  type CredentialProof,
  type ExternalProviderSecurity,
} from "../../hooks/auth/use-security";
import {
  changePasswordSchema,
  changeUsernameSchema,
  type ChangePasswordForm,
  type ChangeUsernameForm,
} from "../../lib/constants/schemas";
import { toastSuccess } from "../../lib/toast";

const SECURITY_SECTIONS = [
  "account",
  "totp",
  "external",
  "login",
  "about",
] as const;
type SecuritySection = (typeof SECURITY_SECTIONS)[number];

function parseSecuritySection(value: string | null): SecuritySection {
  return SECURITY_SECTIONS.includes(value as SecuritySection)
    ? (value as SecuritySection)
    : "account";
}

function providerUnavailableMessage(
  kind: "oauth" | "oidc",
  error?: string,
): string | null {
  if (kind === "oauth") {
    if (!error || error === "OAuth is not configured") {
      return "尚未配置 OAuth";
    }
    return "OAuth 当前不可用";
  }
  if (!error || error === "OIDC is not configured") {
    return "尚未配置 OpenID Connect";
  }
  return "OpenID Connect 当前不可用";
}

export function SecurityPage() {
  const security = useAuthenticationSecurity();
  const me = useMe();
  const [searchParams, setSearchParams] = useSearchParams();
  const section = parseSecuritySection(searchParams.get("section"));
  useDocumentTitle("设置");
  const startTOTP = useStartTOTPEnrollment();
  const disableTOTP = useDisableTOTP();
  const setPasswordLogin = useSetPasswordLogin();
  const startOIDCBinding = useStartOIDCBinding();
  const deleteOIDCBinding = useDeleteOIDCBinding();
  const startOAuthBinding = useStartOAuthBinding();
  const deleteOAuthBinding = useDeleteOAuthBinding();
  const [enrollment, setEnrollment] = useState<EnrollmentContext | null>(null);
  const [localProof, setLocalProof] = useState<CredentialProof>({
    password: "",
  });

  if (security.isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-40" />
        <div className="flex gap-6">
          <Skeleton className="h-48 w-44 shrink-0" />
          <Skeleton className="h-72 min-w-0 flex-1" />
        </div>
      </div>
    );
  }
  if (security.isError || !security.data) {
    return (
      <div className="space-y-3">
        <h1 className="text-xl font-bold">设置</h1>
        <p role="alert" className="text-sm text-red-500">
          无法加载安全设置
        </p>
        <Button variant="default" onClick={() => security.refetch()}>
          重试
        </Button>
      </div>
    );
  }
  const totpRequired = security.data.totp_login_required;
  const passwordLoginEnabled = security.data.password_login_enabled ?? true;
  const oidc = security.data.oidc;
  const oauth = security.data.oauth ?? {
    available: false,
    bound: false,
    configuration_error: "OAuth is not configured",
  };
  const oidcLoginAvailable = oidc.available && oidc.bound;
  const oauthLoginAvailable = oauth.available && oauth.bound;
  const canDisablePasswordLogin = oidcLoginAvailable || oauthLoginAvailable;
  const currentUsername = me.data?.member?.username ?? "";

  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="text-xl font-bold">设置</h1>
      </div>

      <Tabs
        orientation="vertical"
        value={section}
        onValueChange={(value) => {
          const next = parseSecuritySection(value);
          setSearchParams(next === "account" ? {} : { section: next }, {
            replace: true,
          });
        }}
        className="items-start gap-6"
      >
        <TabsList className="shrink-0">
          <TabsTab value="account">
            <UserRound />
            账号
          </TabsTab>
          <TabsTab value="totp">
            <ShieldCheck />
            TOTP
          </TabsTab>
          <TabsTab value="external">
            <Link2 />
            外部登录
          </TabsTab>
          <TabsTab value="login">
            <Settings2 />
            登录配置
          </TabsTab>
          <TabsTab value="about">
            <Info />
            关于
          </TabsTab>
        </TabsList>

        <TabsPanel value="account" className="min-w-0 space-y-4">
          <SectionIntro title="账号" description="修改管理员用户名和密码。" />
          <div className="grid gap-4 md:grid-cols-2">
            <ChangeUsernameCard
              currentUsername={currentUsername}
              totpRequired={totpRequired}
            />
            <ChangePasswordCard totpRequired={totpRequired} />
          </div>
        </TabsPanel>

        <TabsPanel value="totp" className="min-w-0 space-y-4">
          <SectionIntro
            title="TOTP"
            description="用身份验证器生成的动态验证码作为第二步登录。"
          />
          <Frame className="min-w-0 w-full">
            <FramePanel>
              <FrameHeader className="px-0 pt-0">
                <div className="flex items-center justify-between gap-4">
                  <div className="flex items-center gap-3">
                    {totpRequired ? (
                      <ShieldCheck className="size-5 shrink-0" />
                    ) : (
                      <ShieldOff className="size-5 shrink-0" />
                    )}
                    <FrameTitle className="text-base">本地 TOTP</FrameTitle>
                  </div>
                  <Badge variant={totpRequired ? "success" : "outline"}>
                    {totpRequired ? "已启用" : "已停用"}
                  </Badge>
                </div>
              </FrameHeader>

              <div className="mt-4 space-y-4">
                <CredentialProofFields
                  idPrefix="local"
                  value={localProof}
                  onChange={setLocalProof}
                  requireTOTP={totpRequired}
                  passwordLabel={
                    totpRequired ? "当前密码" : "输入当前密码以启用"
                  }
                />
                {(startTOTP.isError || disableTOTP.isError) && (
                  <p role="alert" className="text-sm text-destructive">
                    {String((startTOTP.error ?? disableTOTP.error) as Error)}
                  </p>
                )}
                {totpRequired ? (
                  <Button
                    variant="destructive-outline"
                    loading={disableTOTP.isPending}
                    disabled={!localProof.password || !localProof.totp_code}
                    onClick={() => disableTOTP.mutate(localProof)}
                  >
                    <ShieldOff />
                    停用本地 TOTP
                  </Button>
                ) : (
                  <Button
                    loading={startTOTP.isPending}
                    disabled={!localProof.password}
                    onClick={() =>
                      startTOTP.mutate(localProof.password, {
                        onSuccess: setEnrollment,
                      })
                    }
                  >
                    <KeyRound />
                    启用本地 TOTP
                  </Button>
                )}
              </div>
            </FramePanel>
          </Frame>
        </TabsPanel>

        <TabsPanel value="external" className="min-w-0 space-y-4">
          <SectionIntro
            title="外部登录"
            description="绑定 OAuth 或 OpenID Connect 后，可以用外部身份登录。"
          />
          <div className="grid gap-4 md:grid-cols-2">
            <ExternalProviderCard
              title="OAuth"
              kind="oauth"
              provider={oauth}
              totpRequired={totpRequired}
              lastBindingLocked={!passwordLoginEnabled && !oidcLoginAvailable}
              startBinding={startOAuthBinding}
              deleteBinding={deleteOAuthBinding}
            />
            <ExternalProviderCard
              title="OpenID Connect"
              kind="oidc"
              provider={oidc}
              totpRequired={totpRequired}
              lastBindingLocked={!passwordLoginEnabled && !oauthLoginAvailable}
              startBinding={startOIDCBinding}
              deleteBinding={deleteOIDCBinding}
            />
          </div>
        </TabsPanel>

        <TabsPanel value="login" className="min-w-0 space-y-4">
          <SectionIntro
            title="登录配置"
            description="选择登录页是否还提供用户名和密码。"
          />
          <PasswordLoginOption
            enabled={passwordLoginEnabled}
            canDisable={canDisablePasswordLogin}
            totpRequired={totpRequired}
            setPasswordLogin={setPasswordLogin}
          />
        </TabsPanel>

        <TabsPanel value="about" className="min-w-0 space-y-4">
          <SectionIntro
            title="关于 AutoSecrets"
            description="AutoSecrets 是一个轻量级密钥托管服务。"
          />
          <AboutAutosecrets />
        </TabsPanel>
      </Tabs>

      <Dialog
        open={enrollment !== null}
        onOpenChange={(open) => {
          if (!open) setEnrollment(null);
        }}
      >
        <DialogPopup>
          {enrollment && (
            <MFAEnrollmentSteps
              enrollment={enrollment}
              onDone={() => {
                setEnrollment(null);
                void security.refetch();
              }}
            />
          )}
        </DialogPopup>
      </Dialog>
    </div>
  );
}

function SectionIntro({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <div className="space-y-1">
      <h2 className="text-lg font-semibold">{title}</h2>
      <p className="text-sm text-muted-foreground">{description}</p>
    </div>
  );
}

function PasswordLoginOption({
  enabled,
  canDisable,
  totpRequired,
  setPasswordLogin,
}: {
  enabled: boolean;
  canDisable: boolean;
  totpRequired: boolean;
  setPasswordLogin: {
    isPending: boolean;
    isError: boolean;
    error: unknown;
    reset: () => void;
    mutate: (
      body: { enabled: boolean } & CredentialProof,
      options: { onSuccess: () => void },
    ) => void;
  };
}) {
  const switchId = useId();
  const [proof, setProof] = useState<CredentialProof>({ password: "" });
  const [pendingEnabled, setPendingEnabled] = useState<boolean | null>(null);
  const disableLocked = enabled && !canDisable;
  const proofReady =
    Boolean(proof.password) && (!totpRequired || Boolean(proof.totp_code));
  const confirmLabel = pendingEnabled ? "启用密码登录" : "关闭密码登录";

  return (
    <>
      <Frame className="min-w-0 w-full">
        <FramePanel>
          <div className="flex items-start justify-between gap-4">
            <div className="flex min-w-0 flex-col gap-1">
              <Label htmlFor={switchId}>密码登录</Label>
              <p className="text-muted-foreground text-xs">
                {enabled
                  ? "关闭后只能通过已绑定的 OAuth 或 OpenID Connect 登录。密码仍用于修改安全设置。"
                  : "当前不能用用户名和密码登录。外部登录不可用时会自动恢复。"}
              </p>
              {disableLocked && (
                <p className="text-muted-foreground text-xs">
                  需要先绑定可用的 OAuth 或 OpenID Connect
                </p>
              )}
            </div>
            <Switch
              id={switchId}
              data-testid="password-login-switch"
              checked={enabled}
              disabled={disableLocked || setPasswordLogin.isPending}
              onCheckedChange={(next) => {
                if (next === enabled) {
                  return;
                }
                if (!next && !canDisable) {
                  return;
                }
                setProof({ password: "" });
                setPendingEnabled(next);
              }}
            />
          </div>
        </FramePanel>
      </Frame>

      <Dialog
        open={pendingEnabled !== null}
        onOpenChange={(open) => {
          if (!open) {
            setPendingEnabled(null);
            setProof({ password: "" });
            setPasswordLogin.reset();
          }
        }}
      >
        <DialogPopup>
          <DialogHeader>
            <DialogTitle>{confirmLabel}</DialogTitle>
            <DialogDescription>
              {pendingEnabled
                ? "确认后可以再用用户名和密码登录。"
                : "确认后登录页将只保留已绑定的外部登录。"}
            </DialogDescription>
          </DialogHeader>
          <DialogPanel className="space-y-4">
            <CredentialProofFields
              idPrefix="password-login"
              value={proof}
              onChange={setProof}
              requireTOTP={totpRequired}
              passwordLabel="当前密码"
            />
            {setPasswordLogin.isError && (
              <p role="alert" className="text-sm text-destructive">
                {String(setPasswordLogin.error as Error)}
              </p>
            )}
          </DialogPanel>
          <DialogFooter>
            <DialogClose render={<Button variant="ghost" type="button" />}>
              取消
            </DialogClose>
            <Button
              type="button"
              variant={pendingEnabled ? "default" : "destructive"}
              loading={setPasswordLogin.isPending}
              disabled={!proofReady || pendingEnabled === null}
              onClick={() => {
                if (pendingEnabled === null) {
                  return;
                }
                setPasswordLogin.mutate(
                  { enabled: pendingEnabled, ...proof },
                  {
                    onSuccess: () => {
                      setPendingEnabled(null);
                      setProof({ password: "" });
                      toastSuccess(
                        pendingEnabled ? "已启用密码登录" : "已关闭密码登录",
                      );
                    },
                  },
                );
              }}
            >
              {confirmLabel}
            </Button>
          </DialogFooter>
        </DialogPopup>
      </Dialog>
    </>
  );
}

function ExternalProviderCard({
  title,
  kind,
  provider,
  totpRequired,
  lastBindingLocked,
  startBinding,
  deleteBinding,
}: {
  title: string;
  kind: "oauth" | "oidc";
  provider: ExternalProviderSecurity;
  totpRequired: boolean;
  lastBindingLocked: boolean;
  startBinding: {
    isPending: boolean;
    isError: boolean;
    error: unknown;
    mutate: (
      proof: CredentialProof,
      options: { onSuccess: (data: { authorization_url: string }) => void },
    ) => void;
  };
  deleteBinding: {
    isPending: boolean;
    isError: boolean;
    error: unknown;
    mutate: (proof: CredentialProof) => void;
  };
}) {
  const [proof, setProof] = useState<CredentialProof>({ password: "" });
  const unavailable = provider.available
    ? null
    : providerUnavailableMessage(kind, provider.configuration_error);

  return (
    <Frame className="h-full min-w-0 w-full">
      <FramePanel className="flex-1">
        <FrameHeader className="px-0 pt-0">
          <div className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-3">
              <Link2 className="size-5 shrink-0" />
              <FrameTitle className="text-base">{title}</FrameTitle>
            </div>
            <div className="flex items-center gap-2">
              {provider.bound && (provider.display_name || provider.issuer) ? (
                <Badge variant="outline">
                  {provider.display_name || provider.issuer}
                </Badge>
              ) : null}
              <Badge
                variant={
                  provider.bound
                    ? "success"
                    : provider.available
                      ? "outline"
                      : "secondary"
                }
              >
                {provider.bound
                  ? "已绑定"
                  : provider.available
                    ? "未绑定"
                    : "不可用"}
              </Badge>
            </div>
          </div>
        </FrameHeader>

        {unavailable && (
          <p className="mt-4 text-sm text-muted-foreground">{unavailable}</p>
        )}
        {provider.available && (
          <div className="mt-4 space-y-4">
            <CredentialProofFields
              idPrefix={kind}
              value={proof}
              onChange={setProof}
              requireTOTP={totpRequired}
              passwordLabel="当前密码"
            />
            {lastBindingLocked && (
              <p className="text-sm text-muted-foreground">
                请先启用密码登录，再解除最后一个外部登录
              </p>
            )}
            {(startBinding.isError || deleteBinding.isError) && (
              <p role="alert" className="text-sm text-destructive">
                {String((startBinding.error ?? deleteBinding.error) as Error)}
              </p>
            )}
            {provider.bound ? (
              <Button
                variant="destructive-outline"
                loading={deleteBinding.isPending}
                disabled={
                  lastBindingLocked ||
                  !proof.password ||
                  (totpRequired && !proof.totp_code)
                }
                onClick={() => deleteBinding.mutate(proof)}
              >
                解除绑定
              </Button>
            ) : (
              <Button
                loading={startBinding.isPending}
                disabled={!proof.password || (totpRequired && !proof.totp_code)}
                onClick={() =>
                  startBinding.mutate(proof, {
                    onSuccess: ({ authorization_url }) =>
                      window.location.assign(authorization_url),
                  })
                }
              >
                <Link2 />
                绑定 External Identity
              </Button>
            )}
          </div>
        )}
      </FramePanel>
    </Frame>
  );
}

function ChangeUsernameCard({
  currentUsername,
  totpRequired,
}: {
  currentUsername: string;
  totpRequired: boolean;
}) {
  const changeUsername = useChangeUsername();
  const form = useForm<ChangeUsernameForm>({
    resolver: zodResolver(changeUsernameSchema),
    mode: "onChange",
    defaultValues: {
      username: currentUsername,
      current_password: "",
      totp_code: "",
    },
  });

  useEffect(() => {
    form.reset({
      username: currentUsername,
      current_password: "",
      totp_code: "",
    });
  }, [currentUsername, form]);

  const username = form.watch("username");
  const totpCode = form.watch("totp_code");

  return (
    <Frame className="h-full min-w-0 w-full">
      <FramePanel className="flex-1">
        <FrameHeader className="px-0 pt-0">
          <div className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-3">
              <UserRound className="size-5 shrink-0" />
              <FrameTitle className="text-base">用户名</FrameTitle>
            </div>
            {currentUsername ? (
              <Badge variant="outline">{currentUsername}</Badge>
            ) : null}
          </div>
        </FrameHeader>

        <form
          className="mt-4 flex flex-col gap-4"
          onSubmit={form.handleSubmit((values) => {
            changeUsername.mutate(
              {
                username: values.username,
                current_password: values.current_password,
                totp_code: values.totp_code || undefined,
              },
              {
                onSuccess: () => {
                  toastSuccess("用户名已更新");
                  form.reset({
                    username: values.username,
                    current_password: "",
                    totp_code: "",
                  });
                },
              },
            );
          })}
        >
          <Field invalid={Boolean(form.formState.errors.username)}>
            <FieldLabel htmlFor="username">新用户名</FieldLabel>
            <Input
              id="username"
              data-testid="change-username"
              autoComplete="username"
              {...form.register("username")}
            />
            {form.formState.errors.username && (
              <FieldError>{form.formState.errors.username.message}</FieldError>
            )}
          </Field>
          <Field invalid={Boolean(form.formState.errors.current_password)}>
            <FieldLabel htmlFor="username-password">当前密码</FieldLabel>
            <Input
              id="username-password"
              data-testid="username-password"
              type="password"
              autoComplete="current-password"
              {...form.register("current_password")}
            />
            {form.formState.errors.current_password && (
              <FieldError>
                {form.formState.errors.current_password.message}
              </FieldError>
            )}
          </Field>
          {totpRequired && (
            <ProofOTPField
              id="username-totp"
              value={totpCode ?? ""}
              onChange={(value) =>
                form.setValue("totp_code", value, { shouldValidate: true })
              }
            />
          )}
          {changeUsername.isError && (
            <p role="alert" className="text-sm text-destructive">
              {String(changeUsername.error as Error)}
            </p>
          )}
          <Button
            type="submit"
            loading={changeUsername.isPending}
            disabled={
              !form.formState.isValid ||
              username === currentUsername ||
              (totpRequired && !totpCode)
            }
          >
            更新用户名
          </Button>
        </form>
      </FramePanel>
    </Frame>
  );
}

function ChangePasswordCard({ totpRequired }: { totpRequired: boolean }) {
  const changePassword = useChangePassword();
  const form = useForm<ChangePasswordForm>({
    resolver: zodResolver(changePasswordSchema),
    mode: "onChange",
    defaultValues: {
      current_password: "",
      new_password: "",
      totp_code: "",
    },
  });
  const totpCode = form.watch("totp_code");

  return (
    <Frame className="h-full min-w-0 w-full">
      <FramePanel className="flex-1">
        <FrameHeader className="px-0 pt-0">
          <div className="flex items-center gap-3">
            <Lock className="size-5 shrink-0" />
            <FrameTitle className="text-base">密码</FrameTitle>
          </div>
        </FrameHeader>

        <form
          className="mt-4 flex flex-col gap-4"
          onSubmit={form.handleSubmit((values) => {
            changePassword.mutate(
              {
                current_password: values.current_password,
                new_password: values.new_password,
                totp_code: values.totp_code || undefined,
              },
              {
                onSuccess: () => {
                  toastSuccess("密码已更新");
                  form.reset({
                    current_password: "",
                    new_password: "",
                    totp_code: "",
                  });
                },
              },
            );
          })}
        >
          <Field invalid={Boolean(form.formState.errors.current_password)}>
            <FieldLabel htmlFor="password-current">当前密码</FieldLabel>
            <Input
              id="password-current"
              data-testid="password-current"
              type="password"
              autoComplete="current-password"
              {...form.register("current_password")}
            />
            {form.formState.errors.current_password && (
              <FieldError>
                {form.formState.errors.current_password.message}
              </FieldError>
            )}
          </Field>
          <Field invalid={Boolean(form.formState.errors.new_password)}>
            <FieldLabel htmlFor="password-new">新密码</FieldLabel>
            <Input
              id="password-new"
              data-testid="password-new"
              type="password"
              autoComplete="new-password"
              {...form.register("new_password")}
            />
            {form.formState.errors.new_password && (
              <FieldError>
                {form.formState.errors.new_password.message}
              </FieldError>
            )}
          </Field>
          {totpRequired && (
            <ProofOTPField
              id="password-totp"
              value={totpCode ?? ""}
              onChange={(value) =>
                form.setValue("totp_code", value, { shouldValidate: true })
              }
            />
          )}
          {changePassword.isError && (
            <p role="alert" className="text-sm text-destructive">
              {String(changePassword.error as Error)}
            </p>
          )}
          <Button
            type="submit"
            loading={changePassword.isPending}
            disabled={!form.formState.isValid || (totpRequired && !totpCode)}
          >
            更新密码
          </Button>
        </form>
      </FramePanel>
    </Frame>
  );
}

function ProofOTPField({
  id,
  value,
  onChange,
}: {
  id: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <Field>
      <FieldLabel htmlFor={id}>当前动态验证码</FieldLabel>
      <InputOTP
        id={id}
        data-testid={id}
        maxLength={6}
        value={value}
        onChange={onChange}
      >
        <InputOTPGroup>
          {Array.from({ length: 6 }).map((_, index) => (
            <InputOTPSlot key={index} index={index} />
          ))}
        </InputOTPGroup>
      </InputOTP>
    </Field>
  );
}

type CredentialProofIdPrefix = "local" | "oidc" | "oauth" | "password-login";

function CredentialProofFields({
  idPrefix,
  value,
  onChange,
  requireTOTP,
  passwordLabel,
}: {
  idPrefix: CredentialProofIdPrefix;
  value: CredentialProof;
  onChange: (value: CredentialProof) => void;
  requireTOTP: boolean;
  passwordLabel: string;
}) {
  return (
    <div className="flex flex-col gap-3">
      <Field>
        <FieldLabel htmlFor={`${idPrefix}-password`}>
          {passwordLabel}
        </FieldLabel>
        <Input
          id={`${idPrefix}-password`}
          data-testid={`${idPrefix}-password`}
          type="password"
          autoComplete="current-password"
          value={value.password}
          onChange={(event) =>
            onChange({ ...value, password: event.target.value })
          }
        />
      </Field>
      {requireTOTP && (
        <ProofOTPField
          id={`${idPrefix}-totp`}
          value={value.totp_code ?? ""}
          onChange={(totp_code) => onChange({ ...value, totp_code })}
        />
      )}
    </div>
  );
}

export default SecurityPage;
