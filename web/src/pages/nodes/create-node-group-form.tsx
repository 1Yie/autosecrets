import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useCreateNodeGroup } from "../../hooks/fleet/use-create-node-group";
import { nameSchema } from "../../lib/constants/schemas";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";

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
      <Input className="flex-1"
        placeholder="group name"
        {...register("name")} />
      <Button variant="outline"  type="submit">Create</Button>
    </form>
  );
}
