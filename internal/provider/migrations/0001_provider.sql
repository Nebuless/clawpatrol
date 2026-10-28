CREATE TABLE organizations(id TEXT PRIMARY KEY, name TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1);
CREATE TABLE installations(
 id TEXT PRIMARY KEY, organization_id TEXT NOT NULL REFERENCES organizations(id),
 name TEXT NOT NULL, hosting TEXT NOT NULL CHECK(hosting IN ('customer','nebuless')),
 enabled INTEGER NOT NULL DEFAULT 1, public_key BLOB, credential_hash TEXT,
 enrollment_hash TEXT, enrollment_expires INTEGER,
 UNIQUE(organization_id,id)
);
CREATE TABLE technicians(
 id TEXT PRIMARY KEY, issuer TEXT NOT NULL, subject TEXT NOT NULL, name TEXT NOT NULL,
 administrator INTEGER NOT NULL DEFAULT 0, enabled INTEGER NOT NULL DEFAULT 1,
 UNIQUE(issuer,subject)
);
CREATE TABLE organization_assignments(
 technician_id TEXT NOT NULL REFERENCES technicians(id),
 organization_id TEXT NOT NULL REFERENCES organizations(id),
 PRIMARY KEY(technician_id,organization_id)
);
CREATE TABLE cases(
 id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, installation_id TEXT NOT NULL,
 intent_id TEXT NOT NULL, account_id TEXT NOT NULL, summary TEXT NOT NULL CHECK(length(summary)<=240), category TEXT NOT NULL,
 technician_id TEXT REFERENCES technicians(id), revision INTEGER NOT NULL DEFAULT 1,
 state TEXT NOT NULL DEFAULT 'submitted' CHECK(state IN ('submitted','assigned','closed')),
 expires INTEGER NOT NULL,
 FOREIGN KEY(organization_id,installation_id) REFERENCES installations(organization_id,id),
 UNIQUE(installation_id,intent_id)
);
CREATE TABLE oauth_pending(
 state_hash TEXT PRIMARY KEY, browser_hash TEXT NOT NULL, nonce TEXT NOT NULL,
 verifier TEXT NOT NULL, expires INTEGER NOT NULL
);
CREATE TABLE sessions(
 token_hash TEXT PRIMARY KEY, technician_id TEXT NOT NULL REFERENCES technicians(id),
 access_token BLOB NOT NULL, auth_time INTEGER NOT NULL, expires INTEGER NOT NULL
);
CREATE TABLE handoffs(
 token_hash TEXT PRIMARY KEY, session_id TEXT NOT NULL UNIQUE,
 provider_session_hash TEXT NOT NULL REFERENCES sessions(token_hash),
 organization_id TEXT NOT NULL, installation_id TEXT NOT NULL,
 host TEXT NOT NULL, expires INTEGER NOT NULL, case_id TEXT NOT NULL REFERENCES cases(id),
 FOREIGN KEY(organization_id,installation_id) REFERENCES installations(organization_id,id)
);
CREATE TABLE support_routes(
 token_hash TEXT PRIMARY KEY, session_id TEXT NOT NULL UNIQUE,
 provider_session_hash TEXT NOT NULL REFERENCES sessions(token_hash),
 organization_id TEXT NOT NULL, installation_id TEXT NOT NULL,
 case_id TEXT NOT NULL REFERENCES cases(id), host TEXT NOT NULL UNIQUE, expires INTEGER NOT NULL,
 FOREIGN KEY(organization_id,installation_id) REFERENCES installations(organization_id,id)
);
CREATE TABLE provider_audit(
 id TEXT PRIMARY KEY, at INTEGER NOT NULL, actor_id TEXT, actor_name TEXT, actor_issuer TEXT,
 effective_id TEXT, effective_name TEXT, organization_id TEXT, installation_id TEXT,
 case_id TEXT, session_id TEXT, operation TEXT NOT NULL, outcome TEXT NOT NULL,
 CHECK(effective_id IS NULL AND effective_name IS NULL)
);
CREATE TABLE rate_limits(scope TEXT PRIMARY KEY, failures INTEGER NOT NULL, expires INTEGER NOT NULL);
CREATE INDEX provider_audit_time ON provider_audit(at);
