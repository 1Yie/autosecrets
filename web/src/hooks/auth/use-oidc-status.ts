import { useQuery } from "@tanstack/react-query";
import { apiGet } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export interface OIDCPublicStatus {
  available: boolean;
  bound: boolean;
  login_available: boolean;
}

export function useOIDCStatus() {
  return useQuery({
    queryKey: ["oidc-status"],
    queryFn: () => apiGet<OIDCPublicStatus>(API_PATHS.oidcStatus),
    retry: false,
  });
}
