CREATE TABLE support_tunnel_revocations (
 pubkey TEXT PRIMARY KEY, session_id TEXT NOT NULL REFERENCES support_sessions(id)
);
