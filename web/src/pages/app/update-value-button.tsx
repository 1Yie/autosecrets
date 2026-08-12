import { useState } from "react";
import { useCreateVersion } from "../../hooks/applications/use-create-version";
import type { SecretRow } from "../../hooks/applications/use-secrets";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import {
  Dialog, DialogClose, DialogDescription, DialogFooter,
  DialogHeader, DialogPanel, DialogPopup, DialogTitle, DialogTrigger,
} from "../../components/ui/dialog";

interface UpdateValueButtonProps {
  secret: SecretRow;
  appId: string;
  envId: string;
}

/** Single-field transient input; the guide allows useState for this. */
export function UpdateValueButton({ secret, appId, envId }: UpdateValueButtonProps) {
  const updateValue = useCreateVersion(secret.id, appId, envId);
  const [value, setValue] = useState("");
  const [open, setOpen] = useState(false);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="outline" size="sm" />}>更新值</DialogTrigger>
      <DialogPopup>
        <DialogHeader>
          <DialogTitle>更新 {secret.name} 的值</DialogTitle>
          <DialogDescription>将创建新版本并加入待发布内容；发布后节点同步新值。</DialogDescription>
        </DialogHeader>
        <div className="contents">
          <DialogPanel>
            <div className="space-y-3">
              <Input
                className="w-full"
                placeholder="新值"
                value={value}
                onChange={(e) => setValue(e.target.value)}
                data-testid={`update-${secret.name}`}
              />
              {updateValue.isError && (
                <p className="text-sm text-red-500">{String((updateValue.error as Error).message)}</p>
              )}
            </div>
          </DialogPanel>
          <DialogFooter>
            <DialogClose render={<Button variant="ghost" />}>取消</DialogClose>
            <Button
              variant="default"
              disabled={!value || updateValue.isPending}
              onClick={() => {
                updateValue.mutate(value, {
                  onSuccess: () => {
                    setOpen(false);
                    setValue("");
                  },
                });
              }}
            >
              更新
            </Button>
          </DialogFooter>
        </div>
      </DialogPopup>
    </Dialog>
  );
}
