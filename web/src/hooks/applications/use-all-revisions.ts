import { useQuery } from "@tanstack/react-query";
import { apiGet } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export interface Application { id: string; name: string; created_at: string; }
export interface Environment { id: string; name: string; application_id: string; }
export interface Binding { path: string; uid: number; gid: number; mode: string; }
export interface SecretRow { id: string; name: string; binding: Binding | null; latest_version: number; selected_version: number; }
export interface DraftSelection { secret_id: string; name: string; version_seq: number; binding: Binding; }
export interface Draft { version: number; selections: DraftSelection[]; }
export interface Revision { id: string; draft_version: number; file_count: number; created_by: string; created_at: string; }
export interface RevisionRef { revision_id: string; label: string; }

/** All published revisions across every application/environment, for the
 * assignment UI. */
export function useAllRevisions() {
  return useQuery({
    queryKey: ["all-revisions"],
    queryFn: async () => {
      const apps = await apiGet<Application[]>(API_PATHS.applications);
      const refs: RevisionRef[] = [];
      for (const app of apps) {
        const detail = await apiGet<{ environments: Environment[] }>(API_PATHS.application(app.id));
        for (const env of detail.environments) {
          const revisions = await apiGet<Revision[]>(API_PATHS.revisions(app.id, env.id));
          for (const rev of revisions) {
            refs.push({ revision_id: rev.id, label: `${app.name}/${env.name} · ${rev.id.slice(0, 8)}…` });
          }
        }
      }
      return refs;
    },
  });
}
