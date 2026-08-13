import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useSecrets } from "../../hooks/applications/use-secrets";
import { useCreateSecret } from "../../hooks/applications/use-create-secret";
import { secretSchema, type SecretForm } from "../../lib/constants/schemas";
import { SecretTableRow } from "./binding-row";
import { Skeleton } from "../../components/ui/skeleton";
import { DraftPanel } from "./draft-panel";
import { RevisionsPanel } from "./revisions-panel";
import { ErrorBoundary } from "../../components/error-boundary";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Table, TableBody, TableHead, TableHeader, TableRow } from "../../components/ui/table";
import { Frame } from "../../components/ui/frame";
import { Tabs, TabsContent, TabsList, TabsTab } from "../../components/ui/tabs";
import {
  Dialog, DialogClose, DialogDescription, DialogFooter,
  DialogHeader, DialogPanel, DialogPopup, DialogTitle, DialogTrigger,
} from "../../components/ui/dialog";

interface SecretEditorProps {
  appId: string;
  envId: string;
}

export function SecretEditor({ appId, envId }: SecretEditorProps) {
  const secrets = useSecrets(appId, envId);
  const create = useCreateSecret(appId, envId);
  const [createOpen, setCreateOpen] = useState(false);
  const { register, handleSubmit, reset, formState: { errors } } =
    useForm<SecretForm>({ resolver: zodResolver(secretSchema) });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="font-semibold">密钥</h2>
        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger render={<Button variant="default" />}>添加密钥</DialogTrigger>
          <DialogPopup>
            <DialogHeader>
              <DialogTitle>添加密钥</DialogTitle>
              <DialogDescription>名称与值；发布后已分配的节点会同步该值。</DialogDescription>
            </DialogHeader>
            <form
              className="contents"
              onSubmit={handleSubmit((v) => {
                create.mutate(v, {
                  onSuccess: () => {
                    setCreateOpen(false);
                    reset();
                  },
                });
              })}
            >
              <DialogPanel>
                <div className="space-y-3">
                  <Input className="w-full" placeholder="密钥名称" data-testid="secret-name" {...register("name")} />
                  {errors.name && <p className="text-sm text-red-500">{errors.name.message}</p>}
                  <Input className="w-full" placeholder="密钥值" data-testid="secret-value" {...register("value")} />
                  {errors.value && <p className="text-sm text-red-500">{errors.value.message}</p>}
                  {create.isError && (
                    <p className="text-sm text-red-500">{String((create.error as Error).message)}</p>
                  )}
                </div>
              </DialogPanel>
              <DialogFooter>
                <DialogClose render={<Button variant="ghost" />}>取消</DialogClose>
                <Button variant="default" type="submit" disabled={create.isPending}>
                  添加
                </Button>
              </DialogFooter>
            </form>
          </DialogPopup>
        </Dialog>
      </div>

      {secrets.isLoading && <Skeleton className="h-24 w-full" />}
      {secrets.isError && <p className="text-sm text-red-500">密钥列表加载失败</p>}
      <Frame className="w-full">
        <Table variant="card" className="w-full text-left text-sm">
          <TableHeader>
            <TableRow className="border-b opacity-60">
              <TableHead className="p-2">名称</TableHead>
              <TableHead className="p-2">绑定路径</TableHead>
              <TableHead className="p-2">版本</TableHead>
              <TableHead className="p-2 text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {secrets.data?.map((s) => (
              <SecretTableRow key={s.id} secret={s} appId={appId} envId={envId} />
            ))}
          </TableBody>
        </Table>
      </Frame>

      <Tabs defaultValue="draft">
        <TabsList>
          <TabsTab value="draft">待发布内容</TabsTab>
          <TabsTab value="revisions">版本历史</TabsTab>
        </TabsList>
        <TabsContent value="draft" className="pt-2">
          <ErrorBoundary>
            <DraftPanel appId={appId} envId={envId} />
          </ErrorBoundary>
        </TabsContent>
        <TabsContent value="revisions" className="pt-2">
          <ErrorBoundary>
            <RevisionsPanel appId={appId} envId={envId} />
          </ErrorBoundary>
        </TabsContent>
      </Tabs>
    </div>
  );
}
