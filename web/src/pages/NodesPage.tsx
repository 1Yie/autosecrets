import { useState } from "react";
import {
  useAssignments,
  useCreateAssignment,
  useInstallCommand,
  useNodeGroups,
  useNodes,
  useCreateNodeGroup,
  useAddMember,
  type ManagedNode,
  type NodeGroup,
} from "../hooks/useFleet";
import { useAllRevisions } from "../hooks/useApps";

function GroupRow({ group, nodes }: { group: NodeGroup; nodes: ManagedNode[] }) {
  const addMember = useAddMember(group.id);
  return (
    <li className="flex items-center gap-2">
      <span className="font-semibold">{group.name}</span>
      <span className="opacity-60">({group.member_ids.length} nodes)</span>
      {nodes.map((n) => (
        <button
          key={n.id}
          className={`rounded border px-2 py-0.5 text-xs ${
            group.member_ids.includes(n.id) ? "bg-amber-500" : ""
          }`}
          disabled={addMember.isPending}
          onClick={() => {
            if (!group.member_ids.includes(n.id)) {
              addMember.mutate(n.id);
            }
          }}
        >
          {n.name}
        </button>
      ))}
    </li>
  );
}

function InstallCommandCard() {
  const install = useInstallCommand();
  const [name, setName] = useState("");
  const [copied, setCopied] = useState(false);

  return (
    <section className="rounded border p-4">
      <h2 className="font-semibold">Add a server</h2>
      <p className="mt-1 text-sm opacity-70">
        Generate a one-time install command, run it on the server, and its
        secrets will converge automatically. The token is shown once and
        expires in 10 minutes.
      </p>
      <div className="mt-2 flex gap-2">
        <input
          className="flex-1 rounded border p-2"
          placeholder="server name (e.g. web-1)"
          value={name}
          onChange={(e) => setName(e.target.value)}
          data-testid="node-name"
        />
        <button
          className="rounded bg-amber-500 px-4 font-semibold disabled:opacity-50"
          disabled={install.isPending}
          onClick={() => install.mutate(name.trim() || "node")}
        >
          Generate
        </button>
      </div>
      {install.isError && (
        <p className="mt-2 text-sm text-red-500">
          {String((install.error as Error).message)}
        </p>
      )}
      {install.data && (
        <div className="mt-3 space-y-2">
          <p className="text-sm font-semibold">
            Run on the target server (token shown once, expires{" "}
            {new Date(install.data.expires_at).toLocaleString()}):
          </p>
          <pre
            className="overflow-x-auto rounded bg-black/80 p-3 text-sm text-green-400"
            data-testid="install-command"
          >
            {install.data.command}
          </pre>
          <button
            className="rounded border px-3 py-1 text-sm"
            onClick={async () => {
              await navigator.clipboard.writeText(install.data.command);
              setCopied(true);
            }}
          >
            {copied ? "Copied" : "Copy command"}
          </button>
        </div>
      )}
    </section>
  );
}

export function NodesPage() {
  const nodes = useNodes();
  const groups = useNodeGroups();
  const allRevisions = useAllRevisions();
  const createGroup = useCreateNodeGroup();
  const assignments = useAssignments();
  const createAssignment = useCreateAssignment();
  const [groupName, setGroupName] = useState("");
  const [asg, setAsg] = useState({ group_id: "", revision_id: "" });

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-bold">Nodes</h1>
      <InstallCommandCard />

      <section className="rounded border p-4">
        <h2 className="font-semibold">Managed nodes</h2>
        <table className="mt-2 w-full text-left text-sm">
          <thead>
            <tr className="border-b opacity-60">
              <th className="p-2">Name</th>
              <th className="p-2">Observed</th>
              <th className="p-2">Last result</th>
              <th className="p-2">Last seen</th>
            </tr>
          </thead>
          <tbody>
            {nodes.data?.map((n) => (
              <tr key={n.id} className="border-b">
                <td className="p-2 font-mono">{n.name}</td>
                <td className="p-2 font-mono">{n.observed_revision.slice(0, 8)}…</td>
                <td className="p-2">{n.last_result}</td>
                <td className="p-2">
                  {n.last_seen_at
                    ? new Date(n.last_seen_at).toLocaleTimeString()
                    : "never"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section className="rounded border p-4">
        <h2 className="font-semibold">Node groups</h2>
        <form
          className="mt-2 flex gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (groupName.trim()) {
              createGroup.mutate(groupName.trim());
              setGroupName("");
            }
          }}
        >
          <input className="flex-1 rounded border p-2" placeholder="group name"
            value={groupName} onChange={(e) => setGroupName(e.target.value)} />
          <button className="rounded border px-4">Create</button>
        </form>
        <ul className="mt-2 space-y-1 text-sm">
          {groups.data?.map((g) => (
            <GroupRow key={g.id} group={g} nodes={nodes.data ?? []} />
          ))}
        </ul>
      </section>

      <section className="rounded border p-4">
        <h2 className="font-semibold">Assignments</h2>
        <div className="mt-2 flex gap-2 text-sm">
          <select
            className="flex-1 rounded border p-2"
            value={asg.group_id}
            onChange={(e) => setAsg((a) => ({ ...a, group_id: e.target.value }))}
          >
            <option value="">group…</option>
            {groups.data?.map((g) => (
              <option key={g.id} value={g.id}>{g.name}</option>
            ))}
          </select>
          <select
            className="flex-1 rounded border p-2"
            value={asg.revision_id}
            onChange={(e) => setAsg((a) => ({ ...a, revision_id: e.target.value }))}
            data-testid="assignment-revision"
          >
            <option value="">revision…</option>
            {allRevisions.data?.map((r) => (
              <option key={r.revision_id} value={r.revision_id}>{r.label}</option>
            ))}
          </select>
          <button
            className="rounded bg-amber-500 px-4 font-semibold disabled:opacity-40"
            disabled={!asg.group_id || !asg.revision_id}
            onClick={() =>
              createAssignment.mutate({
                group_id: asg.group_id,
                revision_id: asg.revision_id,
              })
            }
          >
            Assign
          </button>
        </div>
        {createAssignment.isError && (
          <p className="mt-1 text-sm text-red-500">
            {String((createAssignment.error as Error).message)}
          </p>
        )}
        <ul className="mt-2 space-y-1 text-sm">
          {assignments.data?.map((a) => (
            <li key={a.id} className="font-mono">
              {a.group_name} ← {a.revision_id.slice(0, 8)}…
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
