// Centralized API paths. Components and Hooks must reference these
// constants instead of hardcoding route strings.
export const API_PATHS = {
	me: "/api/v1/me",
	health: "/api/v1/health",
	bootstrap: "/api/v1/bootstrap",
	login: "/api/v1/auth/login",
	loginSecondFactor: "/api/v1/auth/login/second-factor",
	oidcStatus: "/api/v1/auth/oidc/status",
	oidcLogin: "/api/v1/auth/oidc/login",
	authSecurity: "/api/v1/auth/security",
	authUsername: "/api/v1/auth/username",
	authPassword: "/api/v1/auth/password",
	authPasswordLogin: "/api/v1/auth/password-login",
	totpEnrollment: "/api/v1/auth/totp/enrollment",
	totp: "/api/v1/auth/totp",
	oidcBinding: "/api/v1/auth/oidc/binding",
	oauthLogin: "/api/v1/auth/oauth/login",
	oauthBinding: "/api/v1/auth/oauth/binding",
	mfaVerify: "/api/v1/auth/mfa-enrollment/verify",
	mfaConfirm: "/api/v1/auth/mfa-enrollment/confirm",
	logout: "/api/v1/auth/logout",
	renew: "/api/v1/auth/renew",
	overview: "/api/v1/overview",
	search: "/api/v1/search",
	auditEvents: "/api/v1/audit-events",
	applications: "/api/v1/applications",
	application: (id: string) => `/api/v1/applications/${id}`,
	environments: (appId: string) => `/api/v1/applications/${appId}/environments`,
	environment: (appId: string, envId: string) =>
		`/api/v1/applications/${appId}/environments/${envId}`,
	secrets: (appId: string, envId: string) =>
		`/api/v1/applications/${appId}/environments/${envId}/secrets`,
	secret: (secretId: string) => `/api/v1/secrets/${secretId}`,
	secretVersions: (secretId: string) => `/api/v1/secrets/${secretId}/versions`,
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
	nodeGroup: (groupId: string) => `/api/v1/node-groups/${groupId}`,
	groupMembers: (groupId: string) => `/api/v1/node-groups/${groupId}/nodes`,
	groupMember: (groupId: string, nodeId: string) =>
		`/api/v1/node-groups/${groupId}/nodes/${nodeId}`,
	assignments: "/api/v1/assignments",
	nodes: "/api/v1/nodes",
	node: (nodeId: string) => `/api/v1/nodes/${nodeId}`,
	installCommand: "/api/v1/nodes/install-command",
	nodeInstallCommand: (nodeId: string) =>
		`/api/v1/nodes/${nodeId}/install-command`,
} as const;
