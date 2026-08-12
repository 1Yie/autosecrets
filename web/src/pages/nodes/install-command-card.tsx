import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useInstallCommand } from "../../hooks/fleet/use-install-command";
import { nameSchema } from "../../lib/constants/schemas";
import { parseInstallCommand } from "../../lib/utils/command";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";

const commandFormSchema = z.object({ name: nameSchema });

export function InstallCommandCard() {
  const install = useInstallCommand();
  const [copied, setCopied] = useState(false);
  const { register, handleSubmit } = useForm<{ name: string }>({
    resolver: zodResolver(commandFormSchema),
    defaultValues: { name: "node" },
  });

  const parsed = install.data ? parseInstallCommand(install.data.command) : null;

  return (
    <section className="rounded border p-4">
      <h2 className="font-semibold">Add a server</h2>
      <p className="mt-1 text-sm opacity-70">
        Generate a one-time install command, run it on the server, and its
        secrets will converge automatically. The token is shown once and
        expires in 10 minutes.
      </p>
      <form className="mt-2 flex gap-2" onSubmit={handleSubmit((v) => install.mutate(v.name))}>
        <Input className="flex-1"
          placeholder="server name (e.g. web-1)"
          data-testid="node-name"
          {...register("name")} />
        <Button variant="default"
          
          disabled={install.isPending}
          type="submit"
        >
          Generate
        </Button>
      </form>
      {install.isError && (
        <p className="mt-2 text-sm text-red-500">
          {String((install.error as Error).message)}
        </p>
      )}
      {install.data && parsed && (
        <div className="mt-3 space-y-2">
          <p className="text-sm font-semibold">
            Run on the target server (token shown once, expires{" "}
            {new Date(install.data.expires_at).toLocaleString()}):
          </p>
          <pre
            className="overflow-x-auto rounded bg-black/80 p-3 text-sm text-green-400"
            data-testid="install-command"
          >
            {install.data.command}
          </pre>
          <Button variant="outline"
            
            onClick={async () => {
              await navigator.clipboard.writeText(install.data.command);
              setCopied(true);
            }}
          >
            {copied ? "Copied" : "Copy command"}
          </Button>
        </div>
      )}
    </section>
  );
}
