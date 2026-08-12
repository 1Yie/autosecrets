import { useState } from "react";
import { useRevisions } from "../../hooks/applications/use-revisions";
import { useRollback } from "../../hooks/applications/use-publish";
import { ApiError } from "../../lib/api";
import { StepUpPrompt } from "../../components/step-up-prompt";
import { Button } from "../../components/ui/button";
import { Skeleton } from "../../components/ui/skeleton";

/** Version history with one-click Rollback to an earlier snapshot. */
export function RevisionsPanel({ appId, envId }: { appId: string; envId: string }) {
  const revisions = useRevisions(appId, envId);
  const rollback = useRollback(appId, envId);
  const [target, setTarget] = useState<string | null>(null);
  const [stepUpNeeded, setStepUpNeeded] = useState(false);

  const onRollback = (revisionID: string) => {
    setTarget(revisionID);
    rollback.mutate(
      { source_revision_id: revisionID },
      {
        onError: (error) => {
          if (error instanceof ApiError && error.code === "step_up_required") {
            setStepUpNeeded(true);
          }
        },
      },
    );
  };

  return (
    <section className="rounded border p-4">
      <h2 className="font-semibold">版本历史</h2>
      {revisions.isLoading && <Skeleton className="h-4 w-48" />}
      {revisions.isError && <p className="text-sm text-red-500">版本历史加载失败</p>}

      {stepUpNeeded && target && (
        <div className="mt-3 rounded border p-3">
          <StepUpPrompt
            prompt="回滚到受保护环境的旧版本需要密码确认。"
            onGranted={() => {
              setStepUpNeeded(false);
              onRollback(target);
            }}
          />
        </div>
      )}
      {rollback.isError && !stepUpNeeded && (
        <p className="mt-1 text-sm text-red-500">{String((rollback.error as Error).message)}</p>
      )}
      {rollback.isSuccess && (
        <p className="mt-1 text-sm text-green-600" data-testid="rollback-success">
          已回滚到 {target?.slice(0, 8)}…，节点将自动同步
        </p>
      )}

      <ul className="mt-2 space-y-1 text-sm">
        {revisions.data?.map((revision) => (
          <li key={revision.id} className="flex items-center justify-between gap-2 rounded border px-3 py-2">
            <div className="min-w-0">
              <div className="font-mono">
                {revision.id.slice(0, 8)}… · 版本 {revision.draft_version} · {revision.file_count} 个文件
              </div>
              <div className="opacity-50">
                由 {revision.created_by} · {new Date(revision.created_at).toLocaleString()}
              </div>
            </div>
            <Button
              variant="outline"
              size="sm"
              disabled={rollback.isPending}
              onClick={() => onRollback(revision.id)}
            >
              回滚到此版本
            </Button>
          </li>
        ))}
      </ul>
    </section>
  );
}
