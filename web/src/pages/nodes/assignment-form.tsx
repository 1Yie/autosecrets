import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useCreateAssignment } from "../../hooks/fleet/use-create-assignment";
import { useApplications } from "../../hooks/applications/use-applications";
import { useApplication } from "../../hooks/applications/use-application";
import type { Assignment } from "../../hooks/fleet/use-assignments";
import type { NodeGroup } from "../../hooks/fleet/use-node-groups";
import { Button } from "../../components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { Skeleton } from "../../components/ui/skeleton";

const assignmentSchema = z.object({
  group_id: z.string().min(1, "请选择节点组"),
  application_id: z.string().min(1, "请选择应用"),
  environment_id: z.string().min(1, "请选择环境"),
});

type AssignmentFormValues = z.infer<typeof assignmentSchema>;

interface AssignmentFormProps {
  groups: NodeGroup[];
  assignments: { items: Assignment[]; isLoading?: boolean; isError?: boolean };
}

/** Bundle Assignment (ADR-0018): relates a Node Group to a Secret Bundle
 * (Application + Environment); the Assignment follows the Desired Revision. */
export function AssignmentForm({ groups, assignments }: AssignmentFormProps) {
  const createAssignment = useCreateAssignment();
  const applications = useApplications();
  const { control, handleSubmit, reset, watch, setValue, formState: { errors, isValid } } =
    useForm<AssignmentFormValues>({
      resolver: zodResolver(assignmentSchema),
      defaultValues: {
        group_id: "",
        application_id: "",
        environment_id: "",
      },
    });
  const appId = watch("application_id");
  const app = useApplication(appId);

  return (
    <section className="rounded border p-4">
      <h2 className="font-semibold">分配</h2>
      <form
        className="mt-2 flex flex-wrap gap-2 text-sm"
        onSubmit={handleSubmit((v) => {
          createAssignment.mutate(v);
          reset();
        })}
      >
        <Controller
          name="group_id"
          control={control}
          render={({ field }) => (
            <Select value={field.value} onValueChange={field.onChange}>
              <SelectTrigger className="flex-1">
                <SelectValue placeholder="选择节点组…" />
              </SelectTrigger>
              <SelectContent>
                {groups.map((g) => (
                  <SelectItem key={g.id} value={g.id}>{g.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
        <Controller
          name="application_id"
          control={control}
          render={({ field }) => (
            <Select value={field.value} onValueChange={(value) => {
              field.onChange(value);
              setValue("environment_id", "");
            }}>
              <SelectTrigger className="flex-1" data-testid="assignment-application">
                <SelectValue placeholder="选择应用…" />
              </SelectTrigger>
              <SelectContent>
                {applications.items.map((app) => (
                  <SelectItem key={app.id} value={app.id}>{app.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
        <Controller
          name="environment_id"
          control={control}
          render={({ field }) => (
            <Select value={field.value} onValueChange={field.onChange} disabled={!appId}>
              <SelectTrigger className="flex-1" data-testid="assignment-environment">
                <SelectValue placeholder="选择环境…" />
              </SelectTrigger>
              <SelectContent>
                {app.data?.environments.map((env) => (
                  <SelectItem key={env.id} value={env.id}>
                    {env.name}（{env.protection}）
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
        <Button type="submit" disabled={createAssignment.isPending || !isValid}>
          分配
        </Button>
      </form>
      {errors.group_id && <p className="mt-1 text-sm text-red-500">{errors.group_id.message}</p>}
      {errors.application_id && <p className="mt-1 text-sm text-red-500">{errors.application_id.message}</p>}
      {errors.environment_id && <p className="mt-1 text-sm text-red-500">{errors.environment_id.message}</p>}
      {createAssignment.isError && (
        <p className="mt-1 text-sm text-red-500">
          {String((createAssignment.error as Error).message)}
        </p>
      )}
      {assignments.isLoading && <Skeleton className="mt-2 h-4 w-40" />}
      {assignments.isError && <p className="mt-1 text-sm text-red-500">Assignments 加载失败</p>}
      <ul className="mt-2 space-y-1 text-sm">
        {assignments.items.map((a) => (
          <li key={a.id} className="font-mono">
            {a.group_name} ← {a.application_id.slice(0, 8)}/{a.environment_id.slice(0, 8)}
          </li>
        ))}
      </ul>
    </section>
  );
}
