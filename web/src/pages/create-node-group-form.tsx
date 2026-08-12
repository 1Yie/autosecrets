import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useCreateNodeGroup } from "../hooks/fleet/use-create-node-group";
import { nameSchema } from "../lib/constants/schemas";

const groupSchema = z.object({ name: nameSchema });

export function CreateNodeGroupForm() {
  const createGroup = useCreateNodeGroup();
  const { register, handleSubmit, reset } = useForm<{ name: string }>({
    resolver: zodResolver(groupSchema),
  });

  return (
    <form
      className="mt-2 flex gap-2"
      onSubmit={handleSubmit((v) => {
        createGroup.mutate(v.name);
        reset();
      })}
    >
      <input
        className="flex-1 rounded border p-2"
        placeholder="group name"
        {...register("name")}
      />
      <button className="rounded border px-4" type="submit">Create</button>
    </form>
  );
}
