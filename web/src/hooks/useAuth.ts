import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiGet, apiPost, setCsrfToken } from "../lib/api";

export interface Me {
  bootstrap_required: boolean;
  admin?: { id: string; username: string };
  csrf_token?: string;
}

export function useMe() {
  return useQuery({
    queryKey: ["me"],
    queryFn: async () => {
      const me = await apiGet<Me>("/api/v1/me");
      if (me.csrf_token) setCsrfToken(me.csrf_token);
      return me;
    },
    retry: false,
  });
}

export function useBootstrap() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { code: string; username: string; password: string }) =>
      apiPost("/api/v1/bootstrap", body),
    onSuccess: () => {
      // The refetched /me would 401 (no session yet) and leave stale data
      // behind; set the transition explicitly so the UI moves to login.
      qc.setQueryData(["me"], { bootstrap_required: false } satisfies Me);
    },
  });
}

export function useLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: { username: string; password: string }) => {
      const res = await apiPost<{ csrf_token: string }>("/api/v1/auth/login", body);
      setCsrfToken(res.csrf_token);
      return res;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["me"] }),
  });
}

export function useLogout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => apiPost("/api/v1/auth/logout"),
    onSuccess: () => {
      setCsrfToken("");
      qc.clear();
    },
  });
}
