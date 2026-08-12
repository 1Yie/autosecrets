import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiPost } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export interface Application { id: string; name: string; created_at: string; }
export interface Environment { id: string; name: string; application_id: string; }
export interface Binding { path: string; uid: number; gid: number; mode: string; }
export interface SecretRow { id: string; name: string; binding: Binding | null; latest_version: number; selected_version: number; }
export interface DraftSelection { secret_id: string; name: string; version_seq: number; binding: Binding; }
export interface Draft { version: number; selections: DraftSelection[]; }
export interface Revision { id: string; draft_version: number; file_count: number; created_by: string; created_at: string; }
export interface RevisionRef { revision_id: string; label: string; }

export function usePublish(appId: string, envId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => apiPost<Revision>(API_PATHS.publish(appId, envId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["draft", appId, envId] });
      qc.invalidateQueries({ queryKey: ["revisions", appId, envId] });
    },
  });
}
