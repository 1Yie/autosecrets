import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useSecrets } from "../../hooks/applications/use-secrets";
import { useCreateSecret } from "../../hooks/applications/use-create-secret";
import { useDraft } from "../../hooks/applications/use-draft";
import { usePublish } from "../../hooks/applications/use-publish";
import { useRevisions } from "../../hooks/applications/use-revisions";
import { secretSchema, type SecretForm } from "../../lib/constants/schemas";
import { BindingRow } from "./binding-row";
import { RotateButton } from "./rotate-button";
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
        <Input className="flex-1" placeholder="Secret name"
          data-testid="secret-name" {...register("name")} />
        <Input className="flex-1" placeholder="Secret value"
          data-testid="secret-value" {...register("value")} />
        <Button variant="default"  type="submit">
          Add secret
        </Button>
      </form>
      {errors.name && <p className="text-sm text-red-500">{errors.name.message}</p>}
      {errors.value && <p className="text-sm text-red-500">{errors.value.message}</p>}

      {secrets.isLoading && <p>Loading secrets…</p>}
      {secrets.isError && <p className="text-red-500">Failed to load secrets</p>}
      <Table className="w-full text-left text-sm">
        <TableHeader>
          <TableRow className="border-b opacity-60">
            <TableHead className="p-2">Name</TableHead>
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
