import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiGet, apiPost } from "../lib/api";

export interface NodeGroup {
  id: string;
  name: string;
  member_ids: string[];
}

export interface Assignment {
  id: string;
  group_id: string;
  group_name: string;
  revision_id: string;
  created_at: string;
}

export interface ManagedNode {
  id: string;
  name: string;
  serial: string;
  created_at: string;
  last_seen_at: string | null;
  desired_etag: string;
  observed_revision: string;
  last_result: string;
}

export function useNodeGroups() {
  return useQuery({
    queryKey: ["node-groups"],
    queryFn: () => apiGet<NodeGroup[]>("/api/v1/node-groups"),
  });
}

export function useCreateNodeGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => apiPost("/api/v1/node-groups", { name }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["node-groups"] }),
  });
}

export function useAddMember(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (nodeId: string) =>
      apiPost(`/api/v1/node-groups/${groupId}/nodes`, { node_id: nodeId }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["node-groups"] }),
  });
}

export function useAssignments() {
  return useQuery({
    queryKey: ["assignments"],
    queryFn: () => apiGet<Assignment[]>("/api/v1/assignments"),
  });
}

export function useCreateAssignment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { group_id: string; revision_id: string }) =>
      apiPost("/api/v1/assignments", body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["assignments"] }),
  });
}

export function useNodes() {
  return useQuery({
    queryKey: ["nodes"],
    queryFn: () => apiGet<ManagedNode[]>("/api/v1/nodes"),
    refetchInterval: 10_000,
  });
}

export function useInstallCommand() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) =>
      apiPost<{ command: string; expires_at: string }>(
        "/api/v1/nodes/install-command",
        { name },
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["nodes"] }),
  });
}
