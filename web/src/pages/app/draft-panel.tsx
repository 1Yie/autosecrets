import { useState } from "react";
import { useDraft } from "../../hooks/applications/use-draft";
import { usePublish } from "../../hooks/applications/use-publish";
import { ApiError } from "../../lib/api";
import { StepUpPrompt } from "../../components/step-up-prompt";
import { Button } from "../../components/ui/button";

/** Draft summary plus one-click Publish. The server no longer requires an
 * Operation Reason (product decision 2026-08); Protected Environments still
 * ask for a Step-up password confirmation when the grant is stale. */
export function DraftPanel({ appId, envId }: { appId: string; envId: string }) {
  const draft = useDraft(appId, envId);
  const publish = usePublish(appId, envId);
  const [stepUpNeeded, setStepUpNeeded] = useState(false);

  const onPublish = () => {
    publish.mutate(undefined, {
      onError: (error) => {
        if (error instanceof ApiError && error.code === "step_up_required") {
          setStepUpNeeded(true);
        }
      },
    });
  };

  return (
    <section className="rounded border p-4">
      <h2 className="font-semibold">待发布内容（版本 {draft.data?.version ?? "…"}）</h2>
      {draft.isError && <p className="text-sm text-red-500">待发布内容加载失败</p>}
      <ul className="mt-2 space-y-1 text-sm">
        {draft.data?.selections.map((sel) => (
          <li key={sel.secret_id} className="flex items-center gap-2">
            <span className="font-mono">{sel.name}</span>
            <span className="opacity-60">版本 {sel.version_seq}</span>
          </li>
        ))}
      </ul>

      {stepUpNeeded ? (
        <div className="mt-3 space-y-2">
          <StepUpPrompt
            prompt="发布到受保护环境需要密码确认。"
            onGranted={() => {
              setStepUpNeeded(false);
              onPublish();
            }}
          />
        </div>
      ) : (
        <Button variant="default" className="mt-3" disabled={publish.isPending} onClick={onPublish}>
          发布
        </Button>
      )}
      {publish.isError && !stepUpNeeded && (
        <p className="mt-1 text-sm text-red-500">{String((publish.error as Error).message)}</p>
      )}
      {publish.isSuccess && (
        <p className="mt-1 text-sm text-green-600" data-testid="publish-success">
          已更新下发目标版本（{publish.data?.id.slice(0, 8)}…），节点将自动同步
        </p>
      )}
    </section>
  );
}
