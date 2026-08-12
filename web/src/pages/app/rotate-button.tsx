import { useState } from "react";
import { useCreateVersion } from "../../hooks/applications/use-create-version";
import type { SecretRow } from "../../hooks/applications/use-secrets";
import { Button } from "../../components/ui/button";

interface RotateButtonProps {
  secret: SecretRow;
  appId: string;
  envId: string;
}

/** Single-field transient input; the guide allows useState for this. */
export function RotateButton({ secret, appId, envId }: RotateButtonProps) {
  const rotate = useCreateVersion(secret.id, appId, envId);
  const [value, setValue] = useState("");

  return (
    <span className="flex gap-1">
      <input
        className="rounded border p-1"
        placeholder="新值"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        data-testid={`rotate-${secret.name}`}
      />
      <Button variant="outline"
        
        disabled={!value || rotate.isPending}
        onClick={() => {
          rotate.mutate(value);
          setValue("");
        }}
      >
        轮换
      </Button>
    </span>
  );
}
