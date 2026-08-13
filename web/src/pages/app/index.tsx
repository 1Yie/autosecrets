import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Controller, useForm } from "react-hook-form";
import { Plus } from "lucide-react";
import { useApplication } from "../../hooks/applications/use-application";
import { useCreateEnvironment } from "../../hooks/applications/use-create-environment";
import { SecretEditor } from "./secret-editor";
import { ErrorBoundary } from "../../components/error-boundary";
import { zodResolver } from "@hookform/resolvers/zod";
import { nameSchema } from "../../lib/constants/schemas";
import { z } from "zod";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Skeleton } from "../../components/ui/skeleton";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { Tabs, TabsList, TabsTab } from "../../components/ui/tabs";
import {
  Dialog, DialogClose, DialogDescription, DialogFooter,
  DialogHeader, DialogPanel, DialogPopup, DialogTitle, DialogTrigger,
} from "../../components/ui/dialog";

const envSchema = z.object({
  name: nameSchema,
  protection: z.enum(["standard", "protected"]),
});

export function AppPage() {
  const { appId } = useParams<{ appId: string }>();
  const app = useApplication(appId ?? "");
  const createEnv = useCreateEnvironment(appId ?? "");
  const [activeEnv, setActiveEnv] = useState<string | null>(null);
  const [envDialogOpen, setEnvDialogOpen] = useState(false);
  const { register, handleSubmit, reset, control } = useForm<{ name: string; protection: "standard" | "protected" }>({
    resolver: zodResolver(envSchema),
    defaultValues: { protection: "standard" },
  });

  if (app.isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-6 w-32" />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }
  if (app.isError) return <p className="text-red-500">应用不存在</p>;

  return (
    <div className="space-y-4">
      <div className="space-y-1">
        <Link
          to="/dashboard/apps"
          className="text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
        >
          ← 应用
        </Link>
        <h1 className="text-xl font-bold">{app.data?.name}</h1>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        {app.data && app.data.environments.length > 0 && (
          <Tabs
            value={activeEnv}
            onValueChange={(value) => setActiveEnv(value ?? null)}
            className="min-w-0"
          >
            <TabsList className="flex-wrap">
              {app.data.environments.map((env) => (
                <TabsTab key={env.id} value={env.id} data-testid={`env-${env.name}`}>
                  {env.name}
                </TabsTab>
              ))}
            </TabsList>
          </Tabs>
        )}
        <Dialog open={envDialogOpen} onOpenChange={setEnvDialogOpen}>
          <DialogTrigger render={<Button variant="outline" className="shrink-0" />}>
            <Plus className="size-4" />
            新建环境
          </DialogTrigger>
          <DialogPopup>
            <DialogHeader>
              <DialogTitle>新建环境</DialogTitle>
              <DialogDescription>环境是密钥的分组；保护级别决定发布时是否需要密码确认。</DialogDescription>
            </DialogHeader>
            <form
              className="contents"
              onSubmit={handleSubmit((v) => {
                createEnv.mutate(
                  { name: v.name, protection: v.protection },
                  {
                    onSuccess: () => {
                      setEnvDialogOpen(false);
                      reset();
                    },
                  },
                );
              })}
            >
              <DialogPanel>
                <div className="space-y-3">
                  <Input className="w-full" placeholder="环境名称" data-testid="env-name" {...register("name")} />
                  <Controller
                    name="protection"
                    control={control}
                    render={({ field }) => (
                      <Select value={field.value} onValueChange={field.onChange}>
                        <SelectTrigger className="w-full" data-testid="env-protection">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="standard">standard（常规）</SelectItem>
                          <SelectItem value="protected">protected（发布需确认密码）</SelectItem>
                        </SelectContent>
                      </Select>
                    )}
                  />
                  {createEnv.isError && (
                    <p className="text-sm text-red-500">{String((createEnv.error as Error).message)}</p>
                  )}
                </div>
              </DialogPanel>
              <DialogFooter>
                <DialogClose render={<Button variant="ghost" />}>取消</DialogClose>
                <Button variant="default" type="submit" disabled={createEnv.isPending}>
                  创建
                </Button>
              </DialogFooter>
            </form>
          </DialogPopup>
        </Dialog>
      </div>
      {activeEnv ? (
        <ErrorBoundary>
          <SecretEditor appId={appId ?? ""} envId={activeEnv} />
        </ErrorBoundary>
      ) : (
        <p className="opacity-60">选择环境以编辑其密钥。</p>
      )}
    </div>
  );
}


export default AppPage;
