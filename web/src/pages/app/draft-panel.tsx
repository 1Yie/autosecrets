import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useDraft } from "../../hooks/applications/use-draft";
import { usePublish } from "../../hooks/applications/use-publish";
import { operationReasonSchema, type OperationReasonForm } from "../../lib/constants/schemas";
import { ApiError } from "../../lib/api";
import { OperationReasonFields } from "../../components/operation-reason-fields";
import { StepUpPrompt } from "../../components/step-up-prompt";
import { Button } from "../../components/ui/button";

/** Draft summary plus the Publish flow: every Desired State change needs an
 * Operation Reason; Protected Environments additionally need Step-up. */
export function DraftPanel({ appId, envId }: { appId: string; envId: string }) {
  const draft = useDraft(appId, envId);
  const publish = usePublish(appId, envId);
  const [open, setOpen] = useState(false);
  const [stepUpNeeded, setStepUpNeeded] = useState(false);
  const form = useForm<OperationReasonForm>({
    resolver: zodResolver(operationReasonSchema),
    mode: "onChange",
    defaultValues: { category: "maintenance", explanation: "", external_ref: "" },
  });

  const onPublish = (values: OperationReasonForm) => {
    publish.mutate(values, {
      onSuccess: () => {
        setOpen(false);
        setStepUpNeeded(false);
        form.reset();
      },
      onError: (error) => {
        if (error instanceof ApiError && error.code === "step_up_required") {
          setStepUpNeeded(true);
        }
      },
    });
  };

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

      {!open ? (
        <Button variant="default" className="mt-3" disabled={publish.isPending} onClick={() => setOpen(true)}>
          发布修订
        </Button>
      ) : stepUpNeeded ? (
        <div className="mt-3 space-y-2">
          <StepUpPrompt
            prompt="发布到受保护环境需要密码确认。"
            onGranted={() => {
              setStepUpNeeded(false);
              void form.handleSubmit(onPublish)();
            }}
          />
          <Button variant="outline" onClick={() => setOpen(false)}>
            取消
          </Button>
        </div>
      ) : (
        <form
          className="mt-3 space-y-2"
          onSubmit={form.handleSubmit(onPublish)}
          data-testid="publish-form"
        >
          <p className="text-xs opacity-70">发布将更新 Desired State；节点随后异步收敛。</p>
          <OperationReasonFields register={form.register} errors={form.formState.errors} />
          {publish.isError && !stepUpNeeded && (
            <p className="text-sm text-red-500">{String((publish.error as Error).message)}</p>
          )}
          <div className="flex gap-2">
            <Button variant="default" type="submit" disabled={publish.isPending || !form.formState.isValid}>
              确认发布
            </Button>
            <Button variant="outline" type="button" onClick={() => setOpen(false)}>
              取消
            </Button>
          </div>
        </form>
      )}
      {publish.isSuccess && !open && (
        <p className="mt-1 text-sm text-green-600" data-testid="publish-success">
          Desired State 已更新（{publish.data?.id.slice(0, 8)}…）
        </p>
      )}
    </section>
  );
}
