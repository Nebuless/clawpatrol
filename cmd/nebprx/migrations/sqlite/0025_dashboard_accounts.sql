CREATE TABLE installation_identity (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1),
 id TEXT NOT NULL UNIQUE,
 organization_id TEXT NOT NULL DEFAULT '',
 tailnet_authority TEXT NOT NULL DEFAULT ''
);
CREATE TABLE dashboard_accounts (
 id TEXT PRIMARY KEY,
 auth_kind TEXT NOT NULL CHECK(auth_kind IN ('password','tailnet')),
 auth_authority TEXT NOT NULL,
 auth_subject TEXT NOT NULL,
 display_name TEXT NOT NULL,
 active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)),
 permission_revision INTEGER NOT NULL DEFAULT 1,
 UNIQUE(auth_kind,auth_authority,auth_subject)
);
CREATE TABLE dashboard_context (
 account_id TEXT PRIMARY KEY REFERENCES dashboard_accounts(id),
 context_json TEXT NOT NULL DEFAULT '{}',
 revision INTEGER NOT NULL DEFAULT 1
);
