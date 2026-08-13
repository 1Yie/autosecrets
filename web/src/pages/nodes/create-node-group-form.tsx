import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useCreateNodeGroup } from "../../hooks/fleet/use-create-node-group";
import { nameSchema } from "../../lib/constants/schemas";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import {
  Frame,
  FrameDescription,
  FrameHeader,
  FramePanel,
  FrameTitle,
} from "../../components/ui/frame";

const groupSchema = z.object({ name: nameSchema });

export function CreateNodeGroupForm() {
  const createGroup = useCreateNodeGroup();
  const { register, handleSubmit, reset } = useForm<{ name: string }>({
    resolver: zodResolver(groupSchema),
  });

  return (
    <Frame className="w-full">
      <FramePanel>
        <FrameHeader className="px-0 pt-0">
          <FrameTitle className="text-base">新建节点组</FrameTitle>
          <FrameDescription>将托管节点分组，以便按组下发密钥版本。</FrameDescription>
        </FrameHeader>
        <form
          className="mt-4 flex gap-2"
          onSubmit={handleSubmit((v) => {
            createGroup.mutate(v.name);
            reset();
          })}
        >
          <Input className="flex-1" placeholder="节点组名称" {...register("name")} />
          <Button variant="outline" type="submit">创建</Button>
        </form>
        {createGroup.isError && (
          <p className="mt-3 text-sm text-red-500">
            {String((createGroup.error as Error).message)}
          </p>
        )}
      </FramePanel>
    </Frame>
  );
}
