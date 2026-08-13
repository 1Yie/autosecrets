import { Fragment, useEffect, useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { ChevronDown, ChevronRight } from "lucide-react";
import { useUpdateBinding } from "../../hooks/applications/use-update-binding";
import { bindingSchema, type BindingForm } from "../../lib/constants/schemas";
import { SAFE_BINDING_MODES, DEFAULT_BINDING_MODE } from "../../lib/constants/modes";
import type { SecretRow } from "../../hooks/applications/use-secrets";
import { Button } from "../../components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { TableCell, TableRow } from "../../components/ui/table";
import { SegmentedPathInput } from "../../components/segmented-path-input";
import { UpdateValueButton } from "./update-value-button";

interface SecretTableRowProps {
  secret: SecretRow;
  appId: string;
  envId: string;
}

/** One row of the secrets table. The binding path and permission mode share
 * one react-hook-form instance so the single 保存 button persists both. */
export function SecretTableRow({ secret, appId, envId }: SecretTableRowProps) {
  const update = useUpdateBinding(secret.id, appId, envId);
  const [advanced, setAdvanced] = useState(false);
  const methods = useForm<BindingForm>({
    resolver: zodResolver(bindingSchema),
    defaultValues: {
      path: secret.binding?.path ?? "",
      mode: (secret.binding?.mode ?? DEFAULT_BINDING_MODE) as BindingForm["mode"],
    },
  });

  useEffect(() => {
    methods.reset({
      path: secret.binding?.path ?? "",
      mode: (secret.binding?.mode ?? DEFAULT_BINDING_MODE) as BindingForm["mode"],
    });
  }, [secret.binding, methods.reset]);

  const onSubmit = (v: BindingForm) => update.mutate({ ...v, uid: 0, gid: 0 });

  return (
    <Fragment>
      <TableRow className="border-b">
        <TableCell className="p-2 font-mono">{secret.name}</TableCell>
        <TableCell className="p-2">
          <Controller
            name="path"
            control={methods.control}
            render={({ field }) => (
              <SegmentedPathInput
                value={field.value}
                onChange={field.onChange}
                onBlur={field.onBlur}
                name={field.name}
                testId={`binding-${secret.name}`}
              />
            )}
          />
          {methods.formState.errors.path && (
            <p className="text-xs text-red-500">{methods.formState.errors.path.message}</p>
          )}
        </TableCell>
        <TableCell className="p-2">
          {secret.selected_version}/{secret.latest_version}
        </TableCell>
        <TableCell className="p-2">
          <div className="flex items-center justify-end gap-1">
            <Button
              type="button"
              variant="outline"
              className="h-8"
              disabled={!methods.formState.isDirty || update.isPending}
              onClick={() => methods.handleSubmit(onSubmit)()}
            >
              保存
            </Button>
            <UpdateValueButton secret={secret} appId={appId} envId={envId} />
            <Button
              type="button"
              variant="ghost"
              className="h-6 w-6 p-0 text-muted-foreground"
              aria-expanded={advanced}
              aria-label="高级设置"
              onClick={() => setAdvanced((v) => !v)}
            >
              {advanced ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
            </Button>
          </div>
        </TableCell>
      </TableRow>
      {advanced && (
        <TableRow className="border-b bg-muted/30">
          <TableCell colSpan={4} className="p-2">
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground">权限</span>
              <Controller
                name="mode"
                control={methods.control}
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger className="h-8 w-24" data-testid={`mode-${secret.name}`}>
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
            </div>
          </TableCell>
        </TableRow>
      )}
    </Fragment>
  );
}
