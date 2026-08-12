import { useState } from "react";
import { useCreateVersion } from "../../hooks/applications/use-create-version";
import type { SecretRow } from "../../hooks/applications/use-secrets";
import { Button } from "../../components/ui/button";

interface UpdateValueButtonProps {
  secret: SecretRow;
  appId: string;
  envId: string;
}

/** Single-field transient input; the guide allows useState for this. */
export function UpdateValueButton({ secret, appId, envId }: UpdateValueButtonProps) {
  const updateValue = useCreateVersion(secret.id, appId, envId);
  const [value, setValue] = useState("");

  return (
    <span className="flex gap-1">
      <input
        className="rounded border p-1"
        placeholder="新值"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        data-testid={`update-${secret.name}`}
      />
      <Button variant="outline"
        
        disabled={!value || updateValue.isPending}
        onClick={() => {
          updateValue.mutate(value);
          setValue("");
        }}
      >
        更新值
      </Button>
    </span>
  );
}
