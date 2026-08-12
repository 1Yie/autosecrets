import type { UseMutationResult, UseQueryResult } from "@tanstack/react-query";
import type { Draft } from "../../hooks/applications/use-draft";
import type { Revision } from "../../hooks/applications/use-revisions";
import { Button } from "../../components/ui/button";

interface DraftPanelProps {
  draft: UseQueryResult<Draft, Error>;
  publish: UseMutationResult<Revision, Error, void, unknown>;
}

export function DraftPanel({ draft, publish }: DraftPanelProps) {
  return (
    <section className="rounded border p-4">
      <h2 className="font-semibold">草稿（v{draft.data?.version ?? "…"}）</h2>
      {draft.isError && <p className="text-sm text-red-500">Draft 加载失败</p>}
      <ul className="mt-2 space-y-1 text-sm">
        {draft.data?.selections.map((sel) => (
          <li key={sel.secret_id} className="flex items-center gap-2">
            <span className="font-mono">{sel.name}</span>
            <span className="opacity-60">seq {sel.version_seq}</span>
          </li>
        ))}
      </ul>
      <Button variant="default" className="mt-3"
        
        disabled={publish.isPending}
        onClick={() => publish.mutate()}
      >
        发布修订
      </Button>
      {publish.isError && (
        <p className="mt-1 text-sm text-red-500">
          {String((publish.error as Error).message)}
        </p>
      )}
      {publish.isSuccess && (
        <p className="mt-1 text-sm text-green-600">已发布 {publish.data?.id}</p>
      )}
    </section>
  );
}
