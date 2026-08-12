import { useState } from "react";
import { useStepUp } from "../hooks/auth/use-mfa-enrollment";
import { Button } from "./ui/button";
import { Input } from "./ui/input";

/** Server-side Step-up prompt: high-risk actions need a recent password
 * confirmation; a fresh grant is issued for five minutes. */
export function StepUpPrompt({
  onGranted,
  prompt,
}: {
  onGranted: () => void;
  prompt?: string;
}) {
  const stepUp = useStepUp();
  const [password, setPassword] = useState("");
  return (
    <div className="space-y-2 rounded border border-amber-300/40 bg-amber-50 p-3 dark:bg-amber-950/20">
      <p className="text-sm font-medium">需要密码确认</p>
      <p className="text-xs opacity-70">
        {prompt ?? "此操作需要最近一次密码确认（Step-up Authentication）。"}
      </p>
      <Input
        type="password"
        data-testid="step-up-password"
        value={password}
        onChange={(event) => setPassword(event.target.value)}
        placeholder="当前密码"
      />
      {stepUp.isError && (
        <p className="text-sm text-red-500">{String((stepUp.error as Error).message)}</p>
      )}
      <Button
        variant="default"
        disabled={!password || stepUp.isPending}
        onClick={() => stepUp.mutate({ password }, { onSuccess: onGranted })}
      >
        确认
      </Button>
    </div>
  );
}
