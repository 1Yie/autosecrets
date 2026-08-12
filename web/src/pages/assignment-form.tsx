import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useCreateAssignment } from "../hooks/fleet/use-create-assignment";
import { useAllRevisions } from "../hooks/applications/use-all-revisions";
import type { UseQueryResult } from "@tanstack/react-query";
import type { Assignment } from "../hooks/fleet/use-assignments";
import type { NodeGroup } from "../hooks/fleet/use-node-groups";

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
  const { register, handleSubmit, reset, formState: { errors } } =
    useForm<AssignmentFormValues>({ resolver: zodResolver(assignmentSchema) });

  return (
    <section className="rounded border p-4">
      <h2 className="font-semibold">Assignments</h2>
      <form
        className="mt-2 flex gap-2 text-sm"
        onSubmit={handleSubmit((v) => {
          createAssignment.mutate(v);
          reset();
        })}
      >
        <select className="flex-1 rounded border p-2" {...register("group_id")}>
          <option value="">group…</option>
          {groups.map((g) => (
            <option key={g.id} value={g.id}>{g.name}</option>
          ))}
        </select>
        <select
          className="flex-1 rounded border p-2"
          data-testid="assignment-revision"
          {...register("revision_id")}
        >
          <option value="">revision…</option>
          {allRevisions.data?.map((r) => (
            <option key={r.revision_id} value={r.revision_id}>{r.label}</option>
          ))}
        </select>
        <button
          className="rounded bg-amber-500 px-4 font-semibold disabled:opacity-40"
          disabled={createAssignment.isPending}
          type="submit"
        >
          Assign
        </button>
      </form>
      {errors.group_id && <p className="mt-1 text-sm text-red-500">{errors.group_id.message}</p>}
      {errors.revision_id && <p className="mt-1 text-sm text-red-500">{errors.revision_id.message}</p>}
      {createAssignment.isError && (
        <p className="mt-1 text-sm text-red-500">
          {String((createAssignment.error as Error).message)}
        </p>
      )}
      {assignments.isLoading && <p className="mt-2 text-sm opacity-60">Loading…</p>}
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
