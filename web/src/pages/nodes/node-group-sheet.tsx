import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useCreateAssignment } from "../../hooks/fleet/use-create-assignment";
import { useApplications } from "../../hooks/applications/use-applications";
import { useApplication } from "../../hooks/applications/use-application";
import { useAssignments } from "../../hooks/fleet/use-assignments";
import { useAddMember } from "../../hooks/fleet/use-add-member";
import { useRemoveMember } from "../../hooks/fleet/use-remove-member";
import type { NodeGroup } from "../../hooks/fleet/use-node-groups";
import type { ManagedNode } from "../../hooks/fleet/use-nodes";
import { Button } from "../../components/ui/button";
import {
  Combobox,
  ComboboxChip,
  ComboboxChips,
  ComboboxChipsInput,
  ComboboxEmpty,
  ComboboxItem,
  ComboboxList,
  ComboboxPopup,
  ComboboxValue,
} from "../../components/ui/combobox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../components/ui/select";
import {
  Sheet,
  SheetClose,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetPanel,
  SheetPopup,
  SheetTitle,
  SheetTrigger,
} from "../../components/ui/sheet";
import { Tabs, TabsContent, TabsList, TabsTab } from "../../components/ui/tabs";

interface NodeGroupSheetProps {
  group: NodeGroup;
  nodes: ManagedNode[];
}

interface NodeOption {
  label: string;
  value: string;
}

const assignmentSchema = z.object({
  application_id: z.string().min(1, "请选择应用"),
  environment_id: z.string().min(1, "请选择环境"),
});

/** Node Group management Sheet: 成员 tab (searchable multi-select) and
 * 分配 tab (Secret Bundle binding for this group). */
export function NodeGroupSheet({ group, nodes }: NodeGroupSheetProps) {
  const addMember = useAddMember(group.id);
  const removeMember = useRemoveMember(group.id);
  const memberIds = group.member_ids;
  const options: NodeOption[] = nodes.map((node) => ({
    label: node.name,
    value: node.id,
  }));
  const memberValue = options.filter((option) => memberIds.includes(option.value));

  const onMembersChange = (next: NodeOption[] | null) => {
    const nextIds = new Set((next ?? []).map((option) => option.value));
    for (const id of memberIds) {
      if (!nextIds.has(id)) removeMember.mutate(id);
    }
    for (const option of next ?? []) {
      if (!memberIds.includes(option.value)) addMember.mutate(option.value);
    }
  };

  const createAssignment = useCreateAssignment();
  const applications = useApplications();
  const { control, handleSubmit, watch, setValue, reset, formState: { errors, isValid } } =
    useForm<{ application_id: string; environment_id: string }>({
      resolver: zodResolver(assignmentSchema),
      defaultValues: { application_id: "", environment_id: "" },
    });
  const appId = watch("application_id");
  const app = useApplication(appId);
  const assignments = useAssignments();
  const groupAssignments = assignments.items.filter((a) => a.group_id === group.id);
  const bind = (values: { application_id: string; environment_id: string }) => {
    createAssignment.mutate(
      { group_id: group.id, ...values },
      {
        onSuccess: () => {
          reset();
          setValue("application_id", "");
        },
      },
    );
  };

  return (
    <Sheet>
      <SheetTrigger
        render={<Button variant="outline" size="sm" />}
        aria-label={`管理 ${group.name}`}
      >
        管理
      </SheetTrigger>
      <SheetPopup>
        <SheetHeader>
          <SheetTitle>{group.name}</SheetTitle>
          <SheetDescription>成员与分配的密钥绑定，变更立即生效。</SheetDescription>
        </SheetHeader>
        <SheetPanel>
          <Tabs defaultValue="members">
            <TabsList>
              <TabsTab value="members">成员</TabsTab>
              <TabsTab value="assignments">分配</TabsTab>
            </TabsList>

            <TabsContent value="members" className="pt-2">
              {nodes.length === 0 ? (
                <p className="text-muted-foreground text-sm">
                  还没有注册节点，先到节点列表添加服务器。
                </p>
              ) : (
                <Combobox<NodeOption, true>
                  items={options}
                  multiple
                  onValueChange={onMembersChange}
                  value={memberValue}
                >
                  <ComboboxChips>
                    <ComboboxValue>
                      {(selected: NodeOption[]) => (
                        <>
                          {selected?.map((item) => (
                            <ComboboxChip aria-label={item.label} key={item.value}>
                              {item.label}
                            </ComboboxChip>
                          ))}
                          <ComboboxChipsInput
                            aria-label="添加节点"
                            placeholder={selected.length > 0 ? undefined : "搜索节点添加…"}
                          />
                        </>
                      )}
                    </ComboboxValue>
                  </ComboboxChips>
                  <ComboboxPopup>
                    <ComboboxEmpty>没有匹配的节点</ComboboxEmpty>
                    <ComboboxList>
                      {(item) => (
                        <ComboboxItem key={item.value} value={item}>
                          {item.label}
                        </ComboboxItem>
                      )}
                    </ComboboxList>
                  </ComboboxPopup>
                </Combobox>
              )}
            </TabsContent>

            <TabsContent value="assignments" className="space-y-3 pt-2">
              <form
                className="space-y-3"
                onSubmit={handleSubmit(bind)}
              >
                <Controller
                  name="application_id"
                  control={control}
                  render={({ field }) => (
                    <Select
                      value={field.value}
                      onValueChange={(value) => {
                        field.onChange(value);
                        setValue("environment_id", "");
                      }}
                    >
                      <SelectTrigger className="w-full" data-testid="assignment-application">
                        <SelectValue placeholder="选择应用…" />
                      </SelectTrigger>
                      <SelectContent>
                        {applications.items.map((item) => (
                          <SelectItem key={item.id} value={item.id}>
                            {item.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                />
                <Controller
                  name="environment_id"
                  control={control}
                  render={({ field }) => (
                    <Select value={field.value} onValueChange={field.onChange} disabled={!appId}>
                      <SelectTrigger className="w-full" data-testid="assignment-environment">
                        <SelectValue placeholder="选择环境…" />
                      </SelectTrigger>
                      <SelectContent>
                        {app.data?.environments.map((env) => (
                          <SelectItem key={env.id} value={env.id}>
                            {env.name}（{env.protection}）
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                />
                {errors.application_id && (
                  <p className="text-sm text-red-500">{errors.application_id.message}</p>
                )}
                {errors.environment_id && (
                  <p className="text-sm text-red-500">{errors.environment_id.message}</p>
                )}
                {createAssignment.isError && (
                  <p className="text-sm text-red-500">
                    {String((createAssignment.error as Error).message)}
                  </p>
                )}
                <Button variant="default" type="submit" disabled={!isValid || createAssignment.isPending}>
                  绑定应用
                </Button>
              </form>

              <div className="space-y-1.5">
                <p className="text-muted-foreground text-sm">已绑定</p>
                {groupAssignments.length === 0 ? (
                  <p className="text-muted-foreground/72 text-sm">该组还没有绑定任何应用。</p>
                ) : (
                  groupAssignments.map((assignment) => (
                    <div
                      key={assignment.id}
                      className="flex items-center justify-between rounded-md border px-3 py-2 text-sm"
                    >
                      <span className="font-mono">
                        {assignment.application_id.slice(0, 8)}/{assignment.environment_id.slice(0, 8)}
                      </span>
                      <span className="text-muted-foreground/72">
                        {assignment.status === "active" ? "已绑定" : "卸载中"}
                      </span>
                    </div>
                  ))
                )}
              </div>
            </TabsContent>
          </Tabs>
        </SheetPanel>
        <SheetFooter>
          <SheetClose render={<Button variant="default" />}>完成</SheetClose>
        </SheetFooter>
      </SheetPopup>
    </Sheet>
  );
}
