import { useRevisions } from "../../hooks/applications/use-revisions";
import { useRollback } from "../../hooks/applications/use-publish";
import { Button } from "../../components/ui/button";
import { Skeleton } from "../../components/ui/skeleton";
import {
  Frame,
  FrameDescription,
  FrameHeader,
  FramePanel,
  FrameTitle,
} from "../../components/ui/frame";

/** Version history with one-click Rollback to an earlier snapshot. */
export function RevisionsPanel({ appId, envId }: { appId: string; envId: string }) {
  const revisions = useRevisions(appId, envId);
  const rollback = useRollback(appId, envId);

  return (
    <Frame className="w-full">
      <FramePanel>
        <FrameHeader className="px-0 pt-0">
          <FrameTitle className="text-base">版本历史</FrameTitle>
          <FrameDescription>已发布的版本快照；回滚后节点自动同步。</FrameDescription>
        </FrameHeader>

        {revisions.isLoading && <Skeleton className="mt-3 h-16 w-full" />}
        {revisions.isError && (
          <p className="mt-3 text-sm text-red-500">版本历史加载失败</p>
        )}
        {!revisions.isLoading && (revisions.data?.length ?? 0) === 0 && (
          <p className="mt-3 text-sm opacity-60">暂无发布记录。</p>
        )}

        {rollback.isError && (
          <p className="mt-3 text-sm text-red-500">
            {String((rollback.error as Error).message)}
          </p>
        )}
        {rollback.isSuccess && (
          <p className="mt-3 text-sm text-green-600" data-testid="rollback-success">
            已回滚，节点将自动同步
          </p>
        )}

        <ul className="mt-3 space-y-1 text-sm">
          {revisions.data?.map((revision) => (
            <li
              key={revision.id}
              className="flex items-center justify-between gap-2 rounded-md border px-3 py-2"
            >
              <div className="min-w-0">
                <div className="truncate font-mono text-xs">
                  {revision.id.slice(0, 8)}… · 版本 {revision.draft_version} ·{" "}
                  {revision.file_count} 个文件
                </div>
                <div className="truncate text-xs opacity-50">
                  由 {revision.created_by} ·{" "}
                  {new Date(revision.created_at).toLocaleString()}
                </div>
              </div>
              <Button
                variant="outline"
                size="sm"
                className="shrink-0"
                disabled={rollback.isPending}
                onClick={() => rollback.mutate({ source_revision_id: revision.id })}
              >
                回滚到此版本
              </Button>
            </li>
          ))}
        </ul>
      </FramePanel>
    </Frame>
  );
}
