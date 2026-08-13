import { useState } from "react";
import { Link } from "react-router-dom";
import { ChevronRight } from "lucide-react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useApplications } from "../../hooks/applications/use-applications";
import { useCreateApplication } from "../../hooks/applications/use-create-application";
import { nameSchema } from "../../lib/constants/schemas";
import { z } from "zod";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Skeleton } from "../../components/ui/skeleton";
import { Frame, FrameFooter } from "../../components/ui/frame";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../components/ui/table";
import {
  Dialog, DialogClose, DialogDescription, DialogFooter,
  DialogHeader, DialogPanel, DialogPopup, DialogTitle, DialogTrigger,
} from "../../components/ui/dialog";

const createAppSchema = z.object({ name: nameSchema });

export function AppsPage() {
  const apps = useApplications();
  const create = useCreateApplication();
  const [open, setOpen] = useState(false);
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<{ name: string }>({ resolver: zodResolver(createAppSchema) });

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">应用</h1>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button variant="default" />}>新建应用</DialogTrigger>
          <DialogPopup>
            <DialogHeader>
              <DialogTitle>新建应用</DialogTitle>
              <DialogDescription>应用是密钥的顶层分组。</DialogDescription>
            </DialogHeader>
            <form
              className="contents"
              onSubmit={handleSubmit((values) => {
                create.mutate(values.name, {
                  onSuccess: () => {
                    setOpen(false);
                    reset();
                  },
                });
              })}
            >
              <DialogPanel>
                <div className="space-y-3">
                  <Input
                    className="w-full"
                    placeholder="应用名称"
                    data-testid="app-name"
                    {...register("name")}
                  />
                  {errors.name && <p className="text-sm text-red-500">{errors.name.message}</p>}
                  {create.isError && (
                    <p className="text-sm text-red-500">{String((create.error as Error).message)}</p>
                  )}
                </div>
              </DialogPanel>
              <DialogFooter>
                <DialogClose render={<Button variant="ghost" />}>取消</DialogClose>
                <Button variant="default" type="submit" disabled={create.isPending}>
                  创建
                </Button>
              </DialogFooter>
            </form>
          </DialogPopup>
        </Dialog>
      </div>

      {apps.isLoading && (
        <div className="space-y-2">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      )}
      {apps.isError && <p className="text-sm text-red-500">应用加载失败</p>}
      {apps.items.length === 0 && !apps.isLoading && (
        <p className="opacity-60">还没有应用，点右上角「新建应用」开始。</p>
      )}
      {apps.items.length > 0 && (
        <Frame className="w-full">
          <Table variant="card">
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead>名称</TableHead>
                <TableHead>创建时间</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {apps.items.map((app) => (
                <TableRow key={app.id} className="hover:bg-muted/50">
                  <TableCell>
                    <Link
                      to={`/dashboard/apps/${app.id}`}
                      className="flex items-center gap-1.5 font-medium text-primary hover:underline"
                    >
                      {app.name}
                      <ChevronRight className="size-4 opacity-50" />
                    </Link>
                  </TableCell>
                  <TableCell className="text-muted-foreground tabular-nums">
                    {new Date(app.created_at).toLocaleString()}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <FrameFooter className="p-2">
            <div className="flex items-center justify-between gap-2">
              <p className="text-muted-foreground text-sm">
                共 <strong className="font-medium text-foreground">{apps.items.length}</strong> 个应用
              </p>
              <div className="flex items-center gap-2">
                <Button variant="outline" size="sm" disabled={apps.isFirstPage} onClick={apps.prev}>
                  上一页
                </Button>
                <Button variant="outline" size="sm" disabled={!apps.nextCursor} onClick={apps.next}>
                  下一页
                </Button>
              </div>
            </div>
          </FrameFooter>
        </Frame>
      )}
    </div>
  );
}

export default AppsPage;
