import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useSecrets } from "../../hooks/applications/use-secrets";
import { useCreateSecret } from "../../hooks/applications/use-create-secret";
import { useDraft } from "../../hooks/applications/use-draft";
import { usePublish } from "../../hooks/applications/use-publish";
import { useRevisions } from "../../hooks/applications/use-revisions";
import { secretSchema, type SecretForm } from "../../lib/constants/schemas";
import { BindingRow } from "./binding-row";
import { Skeleton } from "../../components/ui/skeleton";
import { RotateButton } from "./rotate-button";
import { useRotateSecret } from "../../hooks/applications/use-rotate-secret";
import { DraftPanel } from "./draft-panel";
import { RevisionsPanel } from "./revisions-panel";
import { ErrorBoundary } from "../../components/error-boundary";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../components/ui/table";

interface SecretEditorProps {
  appId: string;
  envId: string;
}

export function SecretEditor({ appId, envId }: SecretEditorProps) {
  const secrets = useSecrets(appId, envId);
  const create = useCreateSecret(appId, envId);
  const draft = useDraft(appId, envId);
  const publish = usePublish(appId, envId);
  const revisions = useRevisions(appId, envId);
  const { register, handleSubmit, reset, formState: { errors } } =
    useForm<SecretForm>({ resolver: zodResolver(secretSchema) });

  return (
    <div className="space-y-6">
      <form
        className="flex gap-2"
        onSubmit={handleSubmit((v) => {
          create.mutate(v);
          reset();
        })}
      >
        <Input className="flex-1" placeholder="密钥名称"
          data-testid="secret-name" {...register("name")} />
        <Input className="flex-1" placeholder="密钥值"
          data-testid="secret-value" {...register("value")} />
        <Button variant="default"  type="submit">
          添加密钥
        </Button>
      </form>
      {errors.name && <p className="text-sm text-red-500">{errors.name.message}</p>}
      {errors.value && <p className="text-sm text-red-500">{errors.value.message}</p>}

      {secrets.isLoading && <Skeleton className="h-24 w-full" />}
      {secrets.isError && <p className="text-red-500">Failed to load secrets</p>}
      <Table className="w-full text-left text-sm">
        <TableHeader>
          <TableRow className="border-b opacity-60">
            <TableHead className="p-2">名称</TableHead>
            <TableHead className="p-2">Binding</TableHead>
            <TableHead className="p-2">Version</TableHead>
            <TableHead className="p-2">Rotate</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {secrets.data?.map((s) => (
            <TableRow key={s.id} className="border-b">
              <TableCell className="p-2 font-mono">{s.name}</TableCell>
              <TableCell className="p-2">
                <BindingRow secret={s} appId={appId} envId={envId} />
              </TableCell>
              <TableCell className="p-2">
                {s.selected_version}/{s.latest_version}
              </TableCell>
              <TableCell className="p-2">
                <RotateButton secret={s} appId={appId} envId={envId} />
                {s.latest_version > 1 && (
                  <RotateNextButton secretId={s.id} appId={appId} envId={envId} />
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <ErrorBoundary>
        <DraftPanel draft={draft} publish={publish} />
      </ErrorBoundary>
      <ErrorBoundary>
        <RevisionsPanel revisions={revisions} />
      </ErrorBoundary>
    </div>
  );
}

/** Rotates the Secret to its next candidate version (old-project style). */
function RotateNextButton({ secretId, appId, envId }: {
  secretId: string;
  appId: string;
  envId: string;
}) {
  const rotate = useRotateSecret(secretId, appId, envId);
  return (
    <Button
      variant="outline"
      className="ml-1"
      disabled={rotate.isPending}
      onClick={() => rotate.mutate()}
      title="轮换到下一个候选值"
      data-testid={`rotate-next-${secretId}`}
    >
      下一候选
    </Button>
  );
}
