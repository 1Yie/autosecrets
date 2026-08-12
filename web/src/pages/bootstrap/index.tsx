import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useBootstrap } from "../../hooks/auth/use-bootstrap";
import { type BootstrapEnrollment } from "../../hooks/auth/use-mfa-enrollment";
import { MFAEnrollmentSteps } from "../../components/mfa-enrollment-steps";
import { bootstrapSchema, type BootstrapForm } from "../../lib/constants/schemas";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";

type Phase = "enroll" | "verify" | "recovery" | "done";

export function BootstrapPage() {
  const [phase, setPhase] = useState<Phase>("enroll");
  const [enrollment, setEnrollment] = useState<BootstrapEnrollment | null>(null);
  const [enrollError, setEnrollError] = useState("");

  const bootstrap = useBootstrap();

  const enroll = useForm<BootstrapForm>({
    resolver: zodResolver(bootstrapSchema),
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

  if ((phase === "verify" || phase === "recovery") && enrollment) {
    return (
      <MFAEnrollmentSteps
        enrollment={{
          username: enrollment.username,
          enrollment_token: enrollment.enrollment_token,
          totp_uri: enrollment.totp_uri,
        }}
        onDone={() => setPhase("done")}
      />
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
