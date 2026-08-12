import { useAddMember } from "../hooks/fleet/use-add-member";
import { useRemoveMember } from "../hooks/fleet/use-remove-member";
import type { NodeGroup } from "../hooks/fleet/use-node-groups";
import type { ManagedNode } from "../hooks/fleet/use-nodes";

interface NodeGroupRowProps {
  group: NodeGroup;
  nodes: ManagedNode[];
}

export function NodeGroupRow({ group, nodes }: NodeGroupRowProps) {
  const addMember = useAddMember(group.id);
  const removeMember = useRemoveMember(group.id);

  return (
    <li className="flex items-center gap-2">
      <span className="font-semibold">{group.name}</span>
      <span className="opacity-60">({group.member_ids.length} nodes)</span>
      {nodes.map((n) => {
        const isMember = group.member_ids.includes(n.id);
        return (
          <button
            key={n.id}
            className={`rounded border px-2 py-0.5 text-xs ${isMember ? "bg-amber-500" : ""}`}
            disabled={addMember.isPending || removeMember.isPending}
            onClick={() => (isMember ? removeMember.mutate(n.id) : addMember.mutate(n.id))}
          >
            {n.name}
          </button>
        );
      })}
    </li>
  );
}
