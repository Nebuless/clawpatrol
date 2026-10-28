CREATE TABLE support_cases (
 id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, installation_id TEXT NOT NULL,
 technician_id TEXT NOT NULL, technician_name TEXT NOT NULL, revision INTEGER NOT NULL,
 state TEXT NOT NULL, expires INTEGER NOT NULL, account_id TEXT NOT NULL REFERENCES dashboard_accounts(id)
);
CREATE TABLE support_grants (
 id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES support_cases(id),
 account_id TEXT NOT NULL REFERENCES dashboard_accounts(id), account_revision INTEGER NOT NULL,
 technician_id TEXT NOT NULL, assignment_revision INTEGER NOT NULL,
 code_hash TEXT, state TEXT NOT NULL CHECK(state IN ('issued','consumed','revoked')),
 created INTEGER NOT NULL, expires INTEGER NOT NULL
);
CREATE UNIQUE INDEX support_active_grant ON support_grants(case_id,account_id) WHERE state='issued';
CREATE TABLE support_sessions (
 id TEXT PRIMARY KEY, grant_id TEXT NOT NULL UNIQUE REFERENCES support_grants(id),
 case_id TEXT NOT NULL REFERENCES support_cases(id), account_id TEXT NOT NULL REFERENCES dashboard_accounts(id),
 account_revision INTEGER NOT NULL, technician_id TEXT NOT NULL, technician_name TEXT NOT NULL,
 assignment_revision INTEGER NOT NULL, state TEXT NOT NULL CHECK(state IN ('pending_activation','active','revoked')),
 origin TEXT NOT NULL, created INTEGER NOT NULL, pending_expires INTEGER NOT NULL, expires INTEGER NOT NULL
);
CREATE UNIQUE INDEX support_account_slot ON support_sessions(account_id) WHERE state IN ('pending_activation','active');
CREATE TABLE support_rate_limits(scope TEXT PRIMARY KEY, failures INTEGER NOT NULL, expires INTEGER NOT NULL);
CREATE TABLE support_audit (
 id TEXT PRIMARY KEY, at INTEGER NOT NULL, request_id TEXT NOT NULL,
 actor_id TEXT, actor_name TEXT, actor_issuer TEXT, effective_id TEXT, effective_name TEXT,
 organization_id TEXT NOT NULL, installation_id TEXT NOT NULL, case_id TEXT, grant_id TEXT, session_id TEXT,
 assignment_revision INTEGER, operation TEXT NOT NULL, method TEXT NOT NULL, outcome TEXT NOT NULL, reason TEXT NOT NULL
);
CREATE INDEX support_audit_time ON support_audit(at);
CREATE INDEX support_grant_expiry ON support_grants(expires);
CREATE INDEX support_session_expiry ON support_sessions(expires,state);
