import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useBootstrap } from "../hooks/auth/use-bootstrap";
import { bootstrapSchema, type BootstrapForm } from "../lib/constants/schemas";

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
      <h1 className="text-xl font-bold">Initialize AutoSecrets</h1>
      <p className="text-sm opacity-70">
        Paste the one-time bootstrap code from the Core logs (
        <code>docker compose logs core</code>) to create the first
        Administrator. There is no default credential.
      </p>
      <form onSubmit={handleSubmit((v) => bootstrap.mutate(v))} className="space-y-3">
        <input
          className="w-full rounded border p-2"
          placeholder="Bootstrap code"
          data-testid="code"
          {...register("code")}
        />
        {errors.code && <p className="text-sm text-red-500">{errors.code.message}</p>}
        <input
          className="w-full rounded border p-2"
          placeholder="Username"
          data-testid="username"
          {...register("username")}
        />
        {errors.username && <p className="text-sm text-red-500">{errors.username.message}</p>}
        <input
          className="w-full rounded border p-2"
          type="password"
          placeholder="Password (min 10 chars)"
          data-testid="password"
          {...register("password")}
        />
        {errors.password && <p className="text-sm text-red-500">{errors.password.message}</p>}
        {bootstrap.isError && (
          <p className="text-sm text-red-500">
            {String((bootstrap.error as Error).message)}
          </p>
        )}
        <button
          className="w-full rounded bg-amber-500 p-2 font-semibold disabled:opacity-50"
          disabled={!isValid || isSubmitting || bootstrap.isPending}
          type="submit"
        >
          Create administrator
        </button>
      </form>
    </div>
  );
}
