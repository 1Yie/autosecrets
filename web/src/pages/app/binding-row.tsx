import { useEffect } from "react";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useUpdateBinding } from "../../hooks/applications/use-update-binding";
import { bindingSchema, type BindingForm } from "../../lib/constants/schemas";
import { SAFE_BINDING_MODES, DEFAULT_BINDING_MODE } from "../../lib/constants/modes";
import type { SecretRow } from "../../hooks/applications/use-secrets";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";

interface BindingRowProps {
  secret: SecretRow;
  appId: string;
  envId: string;
}

export function BindingRow({ secret, appId, envId }: BindingRowProps) {
  const update = useUpdateBinding(secret.id, appId, envId);
  const {
    register,
    control,
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

  useEffect(() => {
    reset({
      path: secret.binding?.path ?? "",
      mode: (secret.binding?.mode ?? DEFAULT_BINDING_MODE) as BindingForm["mode"],
    });
  }, [secret.binding, reset]);

  return (
    <form
      className="flex items-center gap-1"
      onSubmit={handleSubmit((v) => update.mutate({ ...v, uid: 0, gid: 0 }))}
    >
      <Input className="h-8 flex-1 font-mono"
        data-testid={`binding-${secret.name}`}
        {...register("path")}
      />
      <Controller
        name="mode"
        control={control}
        render={({ field }) => (
          <Select value={field.value} onValueChange={field.onChange}>
            <SelectTrigger className="h-8 w-24">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {SAFE_BINDING_MODES.map((m) => (
                <SelectItem key={m} value={m}>{m}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      />
      <Button
        type="submit"
        variant="outline"
        className="h-8"
        disabled={!isDirty || update.isPending}
      >
        Save
      </Button>
      {errors.path && <p className="text-xs text-red-500">{errors.path.message}</p>}
    </form>
  );
}
