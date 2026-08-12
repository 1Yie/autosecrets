import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useRevisions } from "../../hooks/applications/use-revisions";
import { useRollback } from "../../hooks/applications/use-publish";
import { operationReasonSchema, type OperationReasonForm } from "../../lib/constants/schemas";
import { ApiError } from "../../lib/api";
import { OperationReasonFields } from "../../components/operation-reason-fields";
import { StepUpPrompt } from "../../components/step-up-prompt";
import { Button } from "../../components/ui/button";
import { Skeleton } from "../../components/ui/skeleton";

/** Revision history with metadata-only display and Rollback as a new
 * immutable Revision (same Operation Reason and Step-up policy as Publish). */
export function RevisionsPanel({ appId, envId }: { appId: string; envId: string }) {
  const revisions = useRevisions(appId, envId);
  const rollback = useRollback(appId, envId);
  const [target, setTarget] = useState<string | null>(null);
  const [stepUpNeeded, setStepUpNeeded] = useState(false);
  const form = useForm<OperationReasonForm>({
    resolver: zodResolver(operationReasonSchema),
    mode: "onChange",
    defaultValues: { category: "incident_response", explanation: "", external_ref: "" },
  });

  const onRollback = (values: OperationReasonForm) => {
    if (!target) return;
    rollback.mutate(
      { source_revision_id: target, operation_reason: values },
      {
        onSuccess: () => {
          setTarget(null);
          setStepUpNeeded(false);
          form.reset();
        },
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

      {target && (
        <div className="mt-3 space-y-2 rounded border p-3">
          {stepUpNeeded ? (
            <StepUpPrompt
              prompt="回滚到受保护环境的旧版本需要密码确认。"
              onGranted={() => {
                setStepUpNeeded(false);
                void form.handleSubmit(onRollback)();
              }}
            />
          ) : (
            <form onSubmit={form.handleSubmit(onRollback)} data-testid="rollback-form">
              <p className="text-sm">
                回滚到 <span className="font-mono">{target.slice(0, 8)}…</span>：将生成一个新版本恢复当时的文件内容，历史记录不会被改写。
              </p>
              <div className="mt-2 space-y-2">
                <OperationReasonFields register={form.register} errors={form.formState.errors} />
              </div>
              {rollback.isError && !stepUpNeeded && (
                <p className="mt-1 text-sm text-red-500">{String((rollback.error as Error).message)}</p>
              )}
              <div className="mt-2 flex gap-2">
                <Button variant="default" type="submit" disabled={rollback.isPending || !form.formState.isValid}>
                  确认回滚
                </Button>
                <Button variant="outline" type="button" onClick={() => { setTarget(null); setStepUpNeeded(false); }}>
                  取消
                </Button>
              </div>
            </form>
          )}
        </div>
      )}

      <ul className="mt-2 space-y-1 text-sm">
        {revisions.data?.map((revision) => (
          <li key={revision.id} className="flex items-center justify-between gap-2 rounded border px-3 py-2">
            <div className="min-w-0">
              <div className="font-mono">
                {revision.id.slice(0, 8)}… · draft v{revision.draft_version} · {revision.file_count} 个文件
              </div>
              <div className="truncate opacity-70">
                {revision.operation_reason.category} · {revision.operation_reason.explanation}
              </div>
              <div className="opacity-50">
                由 {revision.created_by} · {new Date(revision.created_at).toLocaleString()}
              </div>
            </div>
            <Button
              variant="outline"
              size="sm"
              disabled={rollback.isPending}
              onClick={() => { setTarget(revision.id); setStepUpNeeded(false); form.reset(); }}
            >
              回滚到此版本
            </Button>
          </li>
        ))}
      </ul>
    </section>
  );
}
