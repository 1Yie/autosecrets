import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useInstallCommand } from "../../hooks/fleet/use-install-command";
import { nameSchema } from "../../lib/constants/schemas";
import { parseInstallCommand } from "../../lib/utils/command";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";

const commandFormSchema = z.object({ name: nameSchema });

export function InstallCommandCard() {
  const install = useInstallCommand();
  const [copied, set已复制] = useState(false);
  const { register, handleSubmit } = useForm<{ name: string }>({
    resolver: zodResolver(commandFormSchema),
    defaultValues: { name: "node" },
  });

  const parsed = install.data ? parseInstallCommand(install.data.command) : null;

  return (
    <section>
      <h2 className="font-semibold">添加服务器</h2>
      <p className="mt-1 text-sm opacity-70">
        生成一次性安装命令，在目标服务器上执行后，密钥将自动同步到
        ~/.autosecrets。令牌仅显示一次，10 分钟后过期。
      </p>
      <form className="mt-2 flex gap-2" onSubmit={handleSubmit((v) =>
        install.mutate({ name: v.name }))}>
        <Input className="flex-1"
          placeholder="服务器名称（如 web-1）"
          data-testid="node-name"
          {...register("name")} />
        <Button variant="default"
          disabled={install.isPending}
          type="submit"
        >
          生成
        </Button>
      </form>
      {install.isError && (
        <p className="mt-2 text-sm text-red-500">
          {String((install.error as Error).message)}
        </p>
      )}
      {install.data && parsed && (
        <div className="mt-3 space-y-2">
          <p className="text-sm font-semibold">
            在目标服务器上执行（令牌仅显示一次，过期时间：{" "}
            {new Date(install.data.expires_at).toLocaleString()}):
          </p>
          <pre
            className="overflow-x-auto rounded bg-black/80 p-3 text-sm text-green-400"
            data-testid="install-command"
          >
            {install.data.command}
          </pre>
          <Button variant="outline"
            
            onClick={async () => {
              await navigator.clipboard.writeText(install.data.command);
              set已复制(true);
            }}
          >
            {copied ? "已复制" : "复制命令"}
          </Button>
        </div>
      )}
    </section>
  );
}
