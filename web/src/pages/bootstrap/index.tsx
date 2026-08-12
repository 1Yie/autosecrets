import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useBootstrap } from "../../hooks/auth/use-bootstrap";
import { bootstrapSchema, type BootstrapForm } from "../../lib/constants/schemas";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";

export function BootstrapPage() {
  const bootstrap = useBootstrap();
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting, isValid },
  } = useForm<BootstrapForm>({
    resolver: zodResolver(bootstrapSchema),
    mode: "onChange",
  });

  return (
    <div className="mx-auto mt-16 max-w-md space-y-4 rounded-lg border p-6">
      <h1 className="text-xl font-bold">初始化 AutoSecrets</h1>
      <p className="text-sm opacity-70">
        从 Core 日志（<code>docker compose logs core</code>）粘贴一次性初始化码，
        以创建首位管理员。系统没有默认凭据。
      </p>
      <form onSubmit={handleSubmit((v) => bootstrap.mutate(v))} className="space-y-3">
        <Input className="w-full"
          placeholder="初始化码"
          data-testid="code"
          {...register("code")} />
        {errors.code && <p className="text-sm text-red-500">{errors.code.message}</p>}
        <Input className="w-full"
          placeholder="用户名"
          data-testid="username"
          {...register("username")} />
        {errors.username && <p className="text-sm text-red-500">{errors.username.message}</p>}
        <Input className="w-full"
          type="password"
          placeholder="密码（至少 10 位）"
          data-testid="password"
          {...register("password")} />
        {errors.password && <p className="text-sm text-red-500">{errors.password.message}</p>}
        {bootstrap.isError && (
          <p className="text-sm text-red-500">
            {String((bootstrap.error as Error).message)}
          </p>
        )}
        <Button variant="default" className="w-full"
          
          disabled={!isValid || isSubmitting || bootstrap.isPending}
          type="submit"
        >
          创建管理员
        </Button>
      </form>
    </div>
  );
}
