import { useState } from "react";
import { Check, Copy, Terminal } from "lucide-react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useInstallCommand } from "../../hooks/fleet/use-install-command";
import { nameSchema } from "../../lib/constants/schemas";
import { parseInstallCommand } from "../../lib/utils/command";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import {
  Field,
  FieldDescription,
  FieldLabel,
} from "../../components/ui/field";
import {
  Frame,
  FrameDescription,
  FrameHeader,
  FramePanel,
  FrameTitle,
} from "../../components/ui/frame";
import { ScrollArea } from "../../components/ui/scroll-area";

const commandFormSchema = z.object({
  name: nameSchema,
  bundle_dir: z
    .string()
    .trim()
    .refine(
      (path) => path === "" || path.startsWith("/") || path.startsWith("~/"),
      "部署路径必须是绝对路径或以 ~/ 开头",
    ),
});

type CommandForm = z.infer<typeof commandFormSchema>;

export function InstallCommandCard() {
  const install = useInstallCommand();
  const [copied, setCopied] = useState(false);
  const { register, handleSubmit, formState: { errors } } = useForm<CommandForm>({
    resolver: zodResolver(commandFormSchema),
    defaultValues: { name: "node", bundle_dir: "~/.autosecrets" },
  });

  const parsed = install.data ? parseInstallCommand(install.data.command) : null;

  const copy = async () => {
    if (!install.data) return;
    await navigator.clipboard.writeText(install.data.command);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Frame className="w-full">
      <FramePanel>
        <FrameHeader className="px-0 pt-0">
          <FrameTitle className="text-base">添加服务器</FrameTitle>
          <FrameDescription>
            生成一次性安装命令，在目标服务器上执行后，密钥将自动同步到
            ~/.autosecrets。令牌仅显示一次，10 分钟后过期。
          </FrameDescription>
        </FrameHeader>
        <form
          className="mt-4 grid gap-4 sm:grid-cols-2"
          onSubmit={handleSubmit((values) => install.mutate({
            name: values.name,
            bundle_dir: values.bundle_dir || undefined,
          }))}
        >
          <Field>
            <FieldLabel htmlFor="node-name">服务器名称</FieldLabel>
            <Input
              id="node-name"
              type="text"
              placeholder="如 web-1"
              data-testid="node-name"
              aria-invalid={Boolean(errors.name)}
              {...register("name")}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="node-bundle-dir">部署路径</FieldLabel>
            <Input
              id="node-bundle-dir"
              type="text"
              data-testid="node-bundle-dir"
              aria-invalid={Boolean(errors.bundle_dir)}
              {...register("bundle_dir")}
            />
            <FieldDescription>
              密钥文件写入此目录；支持绝对路径或 ~/ 路径。
            </FieldDescription>
            {errors.bundle_dir && (
              <p className="text-destructive text-sm">{errors.bundle_dir.message}</p>
            )}
          </Field>
          <Button
            className="sm:col-span-2 sm:justify-self-start"
            variant="default"
            disabled={install.isPending}
            type="submit"
          >
            生成
          </Button>
        </form>
        {install.isError && (
          <p className="mt-3 text-sm text-red-500">
            {String((install.error as Error).message)}
          </p>
        )}
        {install.data && parsed && (
          <div className="mt-4 overflow-hidden rounded-lg border bg-zinc-950 dark:bg-black/70">
            <div className="flex flex-wrap items-center justify-between gap-2 border-b border-white/10 px-3 py-2">
              <p className="flex min-w-0 items-center gap-1.5 font-mono text-xs text-zinc-400">
                <Terminal className="size-3.5 shrink-0" />
                <span className="truncate">
                  安装命令 · 过期时间{" "}
                  {new Date(install.data.expires_at).toLocaleString()}
                </span>
              </p>
              <Button
                variant="outline"
                size="xs"
                className="border-white/20 bg-transparent text-zinc-200 hover:bg-white/10"
                onClick={copy}
              >
                {copied ? (
                  <>
                    <Check className="size-3.5" />
                    已复制
                  </>
                ) : (
                  <>
                    <Copy className="size-3.5" />
                    复制命令
                  </>
                )}
              </Button>
            </div>
            <ScrollArea className="w-full">
              <pre
                className="p-3 font-mono text-sm text-green-400"
                data-testid="install-command"
              >
                {install.data.command}
              </pre>
            </ScrollArea>
          </div>
        )}
      </FramePanel>
    </Frame>
  );
}
