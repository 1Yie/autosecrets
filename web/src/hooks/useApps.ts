import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiGet, apiPost, apiPut } from "../lib/api";

export interface Application {
  id: string;
  name: string;
  created_at: string;
}

export interface Environment {
  id: string;
  name: string;
  application_id: string;
}

export interface Binding {
  path: string;
  uid: number;
  gid: number;
  mode: string;
}

export interface SecretRow {
  id: string;
  name: string;
  binding: Binding | null;
  latest_version: number;
  selected_version: number;
}

export interface DraftSelection {
  secret_id: string;
  name: string;
  version_seq: number;
  binding: Binding;
}

export interface Draft {
  version: number;
  selections: DraftSelection[];
}

export interface Revision {
  id: string;
  draft_version: number;
  file_count: number;
  created_by: string;
  created_at: string;
}

export function useApplications() {
  return useQuery({
    queryKey: ["applications"],
    queryFn: () => apiGet<Application[]>("/api/v1/applications"),
  });
}

export function useCreateApplication() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => apiPost("/api/v1/applications", { name }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["applications"] }),
  });
}

export function useCreateEnvironment(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) =>
      apiPost(`/api/v1/applications/${appId}/environments`, { name }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["application", appId] }),
  });
}

export function useApplication(appId: string) {
  return useQuery({
    queryKey: ["application", appId],
    queryFn: () =>
      apiGet<{ id: string; name: string; environments: Environment[] }>(
        `/api/v1/applications/${appId}`,
      ),
  });
}

export function useSecrets(appId: string, envId: string) {
  return useQuery({
    queryKey: ["secrets", appId, envId],
    queryFn: () =>
      apiGet<SecretRow[]>(
        `/api/v1/applications/${appId}/environments/${envId}/secrets`,
      ),
  });
}

export function useCreateSecret(appId: string, envId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { name: string; value: string }) =>
      apiPost(
        `/api/v1/applications/${appId}/environments/${envId}/secrets`,
        body,
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["secrets", appId, envId] });
      qc.invalidateQueries({ queryKey: ["draft", appId, envId] });
    },
  });
}

export function useCreateVersion(secretId: string, appId: string, envId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (value: string) =>
      apiPost(`/api/v1/secrets/${secretId}/versions`, { value }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["secrets", appId, envId] });
      qc.invalidateQueries({ queryKey: ["draft", appId, envId] });
    },
  });
}

export function useUpdateBinding(secretId: string, appId: string, envId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (binding: { path: string; uid: number; gid: number; mode: string }) =>
      apiPut(`/api/v1/secrets/${secretId}/binding`, binding),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["secrets", appId, envId] });
      qc.invalidateQueries({ queryKey: ["draft", appId, envId] });
    },
  });
}

export function useDraft(appId: string, envId: string) {
  return useQuery({
    queryKey: ["draft", appId, envId],
    queryFn: () =>
      apiGet<Draft>(`/api/v1/applications/${appId}/environments/${envId}/draft`),
  });
}

export function usePublish(appId: string, envId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiPost<Revision>(
        `/api/v1/applications/${appId}/environments/${envId}/publish`,
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["draft", appId, envId] });
      qc.invalidateQueries({ queryKey: ["revisions", appId, envId] });
    },
  });
}

export interface RevisionRef {
  revision_id: string;
  label: string;
}

/** All published revisions across every application/environment, for the
 * assignment UI. */
export function useAllRevisions() {
  return useQuery({
    queryKey: ["all-revisions"],
    queryFn: async () => {
      const apps = await apiGet<Application[]>("/api/v1/applications");
      const refs: RevisionRef[] = [];
      for (const app of apps) {
        const detail = await apiGet<{ environments: Environment[] }>(
          `/api/v1/applications/${app.id}`,
        );
        for (const env of detail.environments) {
          const revisions = await apiGet<Revision[]>(
            `/api/v1/applications/${app.id}/environments/${env.id}/revisions`,
          );
          for (const rev of revisions) {
            refs.push({ revision_id: rev.id, label: `${app.name}/${env.name} · ${rev.id.slice(0, 8)}…` });
          }
        }
      }
      return refs;
    },
  });
}

export function useRevisions(appId: string, envId: string) {
  return useQuery({
    queryKey: ["revisions", appId, envId],
    queryFn: () =>
      apiGet<Revision[]>(
        `/api/v1/applications/${appId}/environments/${envId}/revisions`,
      ),
  });
}
