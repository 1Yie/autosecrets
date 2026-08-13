import { useDraft } from "../../hooks/applications/use-draft";
import { usePublish } from "../../hooks/applications/use-publish";
import { Button } from "../../components/ui/button";
import {
  Frame,
  FrameDescription,
  FrameFooter,
  FrameHeader,
  FramePanel,
  FrameTitle,
} from "../../components/ui/frame";

/** Draft summary plus one-click Publish. The server no longer requires an
 * Operation Reason or password confirmation (product decisions 2026-08). */
export function DraftPanel({ appId, envId }: { appId: string; envId: string }) {
  const draft = useDraft(appId, envId);
  const publish = usePublish(appId, envId);

  return (
    <Frame className="w-full">
      <FramePanel>
        <FrameHeader className="px-0 pt-0">
          <FrameTitle className="text-base">待发布内容</FrameTitle>
          <FrameDescription>
            版本 {draft.data?.version ?? "…"} · 发布后节点自动同步
          </FrameDescription>
        </FrameHeader>

        {draft.isError && (
          <p className="mt-3 text-sm text-red-500">待发布内容加载失败</p>
        )}
        {draft.isSuccess && (draft.data?.selections.length ?? 0) === 0 && (
          <p className="mt-3 text-sm opacity-60">暂无待发布内容。</p>
        )}

        <ul className="mt-3 space-y-1 text-sm">
          {draft.data?.selections.map((sel) => (
            <li
              key={sel.secret_id}
              className="flex items-center justify-between gap-2 rounded-md border px-3 py-2"
            >
              <span className="truncate font-mono">{sel.name}</span>
              <span className="shrink-0 text-xs opacity-60">版本 {sel.version_seq}</span>
            </li>
          ))}
        </ul>

        <FrameFooter className="px-0 pb-0">
          <Button
            variant="default"
            className="w-full"
            disabled={publish.isPending}
            onClick={() => publish.mutate()}
          >
            发布
          </Button>
          {publish.isError && (
            <p className="mt-2 text-sm text-red-500">
              {String((publish.error as Error).message)}
            </p>
          )}
          {publish.isSuccess && (
            <p className="mt-2 text-sm text-green-600" data-testid="publish-success">
              已更新下发目标版本（{publish.data?.id.slice(0, 8)}…），节点将自动同步
            </p>
          )}
        </FrameFooter>
      </FramePanel>
    </Frame>
  );
}
