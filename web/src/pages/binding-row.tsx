import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useUpdateBinding } from "../hooks/applications/use-update-binding";
import { bindingSchema, type BindingForm } from "../lib/constants/schemas";
import { SAFE_BINDING_MODES, DEFAULT_BINDING_MODE } from "../lib/constants/modes";
import type { SecretRow } from "../hooks/applications/use-secrets";

interface BindingRowProps {
  secret: SecretRow;
  appId: string;
  envId: string;
}

export function BindingRow({ secret, appId, envId }: BindingRowProps) {
  const update = useUpdateBinding(secret.id, appId, envId);
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isDirty },
  } = useForm<BindingForm>({
    resolver: zodResolver(bindingSchema),
    defaultValues: {
      path: secret.binding?.path ?? "",
      mode: (secret.binding?.mode ?? DEFAULT_BINDING_MODE) as BindingForm["mode"],
    },
  });

  // Refresh the form when the server-side binding changes (e.g. after save).
  useEffect(() => {
    reset({
      path: secret.binding?.path ?? "",
      mode: (secret.binding?.mode ?? DEFAULT_BINDING_MODE) as BindingForm["mode"],
    });
  }, [secret.binding, reset]);

  return (
    <form
      className="flex gap-1"
      onSubmit={handleSubmit((v) => update.mutate({ ...v, uid: 0, gid: 0 }))}
    >
      <input
        className="flex-1 rounded border p-1 font-mono"
        data-testid={`binding-${secret.name}`}
        {...register("path")}
      />
      <select className="rounded border p-1" {...register("mode")}>
        {SAFE_BINDING_MODES.map((m) => (
          <option key={m} value={m}>{m}</option>
        ))}
      </select>
      <button
        className="rounded border px-2 py-1 disabled:opacity-40"
        disabled={!isDirty || update.isPending}
        type="submit"
      >
        Save
      </button>
      {errors.path && <p className="text-xs text-red-500">{errors.path.message}</p>}
    </form>
  );
}
