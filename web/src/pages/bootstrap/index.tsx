import { useNavigate } from "react-router-dom";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useBootstrap } from "../../hooks/auth/use-bootstrap";
import { bootstrapSchema, type BootstrapForm } from "../../lib/constants/schemas";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";

export function BootstrapPage() {
  const navigate = useNavigate();
  const bootstrap = useBootstrap();
  const form = useForm<BootstrapForm>({ resolver: zodResolver(bootstrapSchema), mode: "onChange" });

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold">初始化 AutoSecrets</h1>
      <p className="text-sm text-muted-foreground">
        从 Core 日志粘贴一次性初始化码，创建 Organization 和 Administrator。本地登录默认使用用户名与密码，之后可在登录安全中启用 TOTP。
      </p>
      <form
        onSubmit={form.handleSubmit((values) => bootstrap.mutate(values, {
          onSuccess: () => navigate("/dashboard/overview", { replace: true }),
        }))}
        className="space-y-3"
      >
        <Input className="w-full" placeholder="初始化码" data-testid="code" {...form.register("code")} />
        {form.formState.errors.code && <p className="text-sm text-destructive">{form.formState.errors.code.message}</p>}
        <Input className="w-full" placeholder="组织名称" data-testid="organization-name" {...form.register("organization_name")} />
        {form.formState.errors.organization_name && <p className="text-sm text-destructive">{form.formState.errors.organization_name.message}</p>}
        <Input className="w-full" placeholder="用户名" data-testid="username" {...form.register("username")} />
        {form.formState.errors.username && <p className="text-sm text-destructive">{form.formState.errors.username.message}</p>}
        <Input className="w-full" type="password" placeholder="密码（至少 12 位）" data-testid="password" {...form.register("password")} />
        {form.formState.errors.password && <p className="text-sm text-destructive">{form.formState.errors.password.message}</p>}
        {bootstrap.isError && <p role="alert" className="text-sm text-destructive">{String((bootstrap.error as Error).message)}</p>}
        <Button className="w-full" disabled={!form.formState.isValid} loading={bootstrap.isPending} type="submit">
          创建管理员
        </Button>
      </form>
    </div>
  );
}

export default BootstrapPage;
