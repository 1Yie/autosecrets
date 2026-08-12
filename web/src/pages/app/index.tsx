import { useState } from "react";
import { Link, useParams } from "react-router-dom";
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

const envSchema = z.object({ name: nameSchema });

export function AppPage() {
  const { appId } = useParams<{ appId: string }>();
  const app = useApplication(appId ?? "");
  const createEnv = useCreateEnvironment(appId ?? "");
  const [activeEnv, setActiveEnv] = useState<string | null>(null);
  const { register, handleSubmit, reset } = useForm<{ name: string }>({
    resolver: zodResolver(envSchema),
  });

  if (app.isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-6 w-32" />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }
  if (app.isError) return <p className="text-red-500">Application not found</p>;

  return (
    <div className="space-y-4">
      <Link to="/apps" className="text-sm opacity-70">← Applications</Link>
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
            createEnv.mutate(v.name);
            reset();
          })}
        >
          <Input className="" placeholder="new env" {...register("name")} />
          <Button variant="outline"  type="submit">+</Button>
        </form>
      </div>
      {activeEnv ? (
        <ErrorBoundary>
          <SecretEditor appId={appId ?? ""} envId={activeEnv} />
        </ErrorBoundary>
      ) : (
        <p className="opacity-60">Select an environment to edit its secrets.</p>
      )}
    </div>
  );
}


export default AppPage;
