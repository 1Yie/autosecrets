import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Controller } from "react-hook-form";
import { useApplication } from "../../hooks/applications/use-application";
import { useCreateEnvironment } from "../../hooks/applications/use-create-environment";
import { SecretEditor } from "./secret-editor";
import { ErrorBoundary } from "../../components/error-boundary";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { nameSchema } from "../../lib/constants/schemas";
import { z } from "zod";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Skeleton } from "../../components/ui/skeleton";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";

const envSchema = z.object({
  name: nameSchema,
  protection: z.enum(["standard", "protected"]),
});

export function AppPage() {
  const { appId } = useParams<{ appId: string }>();
  const app = useApplication(appId ?? "");
  const createEnv = useCreateEnvironment(appId ?? "");
  const [activeEnv, setActiveEnv] = useState<string | null>(null);
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
      <Link to="/apps" className="text-sm opacity-70">← 应用</Link>
      <h1 className="text-xl font-bold">{app.data?.name}</h1>
      <div className="flex flex-wrap items-center gap-2">
        {app.data?.environments.map((env) => (
          <Button
            key={env.id}
            className={`rounded px-3 py-1 ${activeEnv === env.id ? "bg-amber-500" : "border"}`}
            onClick={() => setActiveEnv(env.id)}
            data-testid={`env-${env.name}`}
          >
            {env.name}
          </Button>
        ))}
        <form
          className="flex gap-1"
          onSubmit={handleSubmit((v) => {
            createEnv.mutate({ name: v.name, protection: v.protection });
            reset();
          })}
        >
          <Input className="" placeholder="新环境" {...register("name")} />
          <Controller
            name="protection"
            control={control}
            render={({ field }) => (
              <Select value={field.value} onValueChange={field.onChange}>
                <SelectTrigger className="w-28" data-testid="env-protection">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="standard">standard</SelectItem>
                  <SelectItem value="protected">protected</SelectItem>
                </SelectContent>
              </Select>
            )}
          />
          <Button variant="outline"  type="submit">+</Button>
        </form>
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
