// Centralized API paths. Components and Hooks must reference these
// constants instead of hardcoding route strings.
export const API_PATHS = {
  me: "/api/v1/me",
  bootstrap: "/api/v1/bootstrap",
  login: "/api/v1/auth/login",
  mfaVerify: "/api/v1/auth/mfa-enrollment/verify",
  mfaConfirm: "/api/v1/auth/mfa-enrollment/confirm",
  logout: "/api/v1/auth/logout",
  stepUp: "/api/v1/auth/step-up",
  renew: "/api/v1/auth/renew",
  overview: "/api/v1/overview",
  auditEvents: "/api/v1/audit-events",
  applications: "/api/v1/applications",
  application: (id: string) => `/api/v1/applications/${id}`,
  environments: (appId: string) => `/api/v1/applications/${appId}/environments`,
  secrets: (appId: string, envId: string) =>
    `/api/v1/applications/${appId}/environments/${envId}/secrets`,
  secretVersions: (secretId: string) => `/api/v1/secrets/${secretId}/versions`,
  secretRotate: (secretId: string) => `/api/v1/secrets/${secretId}/rotate`,
  secretBinding: (secretId: string) => `/api/v1/secrets/${secretId}/binding`,
  draft: (appId: string, envId: string) =>
    `/api/v1/applications/${appId}/environments/${envId}/draft`,
  publish: (appId: string, envId: string) =>
    `/api/v1/applications/${appId}/environments/${envId}/publish`,
  rollback: (appId: string, envId: string) =>
    `/api/v1/applications/${appId}/environments/${envId}/rollback`,
  revisions: (appId: string, envId: string) =>
    `/api/v1/applications/${appId}/environments/${envId}/revisions`,
  nodeGroups: "/api/v1/node-groups",
  groupMembers: (groupId: string) => `/api/v1/node-groups/${groupId}/nodes`,
  groupMember: (groupId: string, nodeId: string) =>
    `/api/v1/node-groups/${groupId}/nodes/${nodeId}`,
  assignments: "/api/v1/assignments",
  nodes: "/api/v1/nodes",
  installCommand: "/api/v1/nodes/install-command",
} as const;
