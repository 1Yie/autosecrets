import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useSecrets } from "../../hooks/applications/use-secrets";
import { useCreateSecret } from "../../hooks/applications/use-create-secret";
import { secretSchema, type SecretForm } from "../../lib/constants/schemas";
import { BindingRow } from "./binding-row";
import { Skeleton } from "../../components/ui/skeleton";
import { UpdateValueButton } from "./update-value-button";
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
      {secrets.isError && <p className="text-sm text-red-500">密钥列表加载失败</p>}
      <Table className="w-full text-left text-sm">
        <TableHeader>
          <TableRow className="border-b opacity-60">
            <TableHead className="p-2">名称</TableHead>
            <TableHead className="p-2">Binding</TableHead>
            <TableHead className="p-2">Version</TableHead>
            <TableHead className="p-2">更新</TableHead>
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
                <UpdateValueButton secret={s} appId={appId} envId={envId} />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <ErrorBoundary>
        <DraftPanel appId={appId} envId={envId} />
      </ErrorBoundary>
      <ErrorBoundary>
        <RevisionsPanel appId={appId} envId={envId} />
      </ErrorBoundary>
    </div>
  );
}
