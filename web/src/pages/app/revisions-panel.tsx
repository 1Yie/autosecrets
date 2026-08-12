import type { UseQueryResult } from "@tanstack/react-query";
import type { Revision } from "../../hooks/applications/use-revisions";
import { Skeleton } from "../../components/ui/skeleton";

interface RevisionsPanelProps {
  revisions: UseQueryResult<Revision[], Error>;
}

export function RevisionsPanel({ revisions }: RevisionsPanelProps) {
  return (
    <section className="rounded border p-4">
      <h2 className="font-semibold">修订历史</h2>
      {revisions.isLoading && <Skeleton className="h-4 w-48" />}
      {revisions.isError && <p className="text-sm text-red-500">Revisions 加载失败</p>}
      <ul className="mt-2 space-y-1 text-sm">
        {revisions.data?.map((r) => (
          <li key={r.id} className="font-mono">
            {r.id.slice(0, 8)}… · draft v{r.draft_version} · {r.file_count} 个文件 · 由 {r.created_by}
          </li>
        ))}
      </ul>
    </section>
  );
}
