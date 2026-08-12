import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useCreateAssignment } from "../../hooks/fleet/use-create-assignment";
import { useAllRevisions } from "../../hooks/applications/use-all-revisions";
import type { UseQueryResult } from "@tanstack/react-query";
import type { Assignment } from "../../hooks/fleet/use-assignments";
import type { NodeGroup } from "../../hooks/fleet/use-node-groups";
import { Button } from "../../components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { Skeleton } from "../../components/ui/skeleton";

const assignmentSchema = z.object({
  group_id: z.string().min(1, "请选择节点组"),
  revision_id: z.string().min(1, "请选择 revision"),
});

type AssignmentFormValues = z.infer<typeof assignmentSchema>;

interface AssignmentFormProps {
  groups: NodeGroup[];
  assignments: UseQueryResult<Assignment[], Error>;
}

export function AssignmentForm({ groups, assignments }: AssignmentFormProps) {
  const createAssignment = useCreateAssignment();
  const allRevisions = useAllRevisions();
  const { control, handleSubmit, reset, formState: { errors } } =
    useForm<AssignmentFormValues>({
      resolver: zodResolver(assignmentSchema),
      defaultValues: { group_id: "", revision_id: "" },
    });

  return (
    <section className="rounded border p-4">
      <h2 className="font-semibold">分配</h2>
      <form
        className="mt-2 flex gap-2 text-sm"
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
          name="revision_id"
          control={control}
          render={({ field }) => (
            <Select value={field.value} onValueChange={field.onChange}>
              <SelectTrigger className="flex-1" data-testid="assignment-revision">
                <SelectValue placeholder="选择修订…" />
              </SelectTrigger>
              <SelectContent>
                {allRevisions.data?.map((r) => (
                  <SelectItem key={r.revision_id} value={r.revision_id}>
                    {r.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
        <Button type="submit" disabled={createAssignment.isPending}>
          分配
        </Button>
      </form>
      {errors.group_id && <p className="mt-1 text-sm text-red-500">{errors.group_id.message}</p>}
      {errors.revision_id && <p className="mt-1 text-sm text-red-500">{errors.revision_id.message}</p>}
      {createAssignment.isError && (
        <p className="mt-1 text-sm text-red-500">
          {String((createAssignment.error as Error).message)}
        </p>
      )}
      {assignments.isLoading && <Skeleton className="mt-2 h-4 w-40" />}
      {assignments.isError && <p className="mt-1 text-sm text-red-500">Assignments 加载失败</p>}
      <ul className="mt-2 space-y-1 text-sm">
        {assignments.data?.map((a) => (
          <li key={a.id} className="font-mono">
            {a.group_name} ← {a.revision_id.slice(0, 8)}…
          </li>
        ))}
      </ul>
    </section>
  );
}
