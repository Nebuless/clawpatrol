package main

import (
	"context"
	"database/sql"
	"errors"
	"github.com/Nebuless/nebprx/internal/provider"
	"net"
	"net/http"
)

func (w *webMux) supportDeviceMutation(ctx context.Context, s *supportStore, c provider.Case, session supportSession, r *http.Request) error {
	ip := r.URL.Query().Get("ip")
	if net.ParseIP(ip) == nil || w.onboard == nil {
		return errors.New("invalid device")
	}
	profile := r.URL.Query().Get("profile")
	deleting := r.URL.Path == "/api/agents/delete"
	if !deleting {
		known := false
		for _, p := range orderedProfileNames(w.g.cfg.Load().Policy) {
			if p == profile {
				known = true
			}
		}
		if !known {
			return errors.New("unknown profile")
		}
	}
	// The support admission lock remains held through commit and memory update.
	// External WG revocation has its own durable intent before IpcSet.
	registry := w.onboard
	canonical := ip
	mutate := func() error {
		registry.mu.Lock()
		defer registry.mu.Unlock()
		if v := registry.canonicalByAlias[ip]; v != "" {
			canonical = v
		}
		var exists int
		if err := s.conn.QueryRowContext(ctx, "SELECT count(*) FROM devices WHERE id=?", canonical).Scan(&exists); err != nil {
			return err
		}
		if exists != 1 {
			return errors.New("device unavailable")
		}
		var keys []string
		if deleting {
			rows, err := s.conn.QueryContext(ctx, "SELECT pubkey FROM wg_peers WHERE ip=?", canonical)
			if err != nil {
				return err
			}
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var key string
				if err = rows.Scan(&key); err != nil {
					_ = rows.Close()
					return err
				}
				keys = append(keys, key)
			}
			err = rows.Err()
			_ = rows.Close()
			if err != nil {
				return err
			}
		}
		err := s.transaction(func(tx *sql.Tx) error {
			var admitted int
			if err := tx.QueryRow("SELECT count(*) FROM support_sessions s JOIN dashboard_accounts a ON a.id=s.account_id JOIN support_cases c ON c.id=s.case_id WHERE s.id=? AND s.state='active' AND s.expires>? AND a.active=1 AND a.permission_revision=s.account_revision AND c.state='assigned' AND c.revision=s.assignment_revision AND EXISTS(SELECT 1 FROM dashboard_users WHERE username='root')", session.ID, s.now().Unix()).Scan(&admitted); err != nil {
				return err
			}
			if admitted != 1 {
				return errGrantUnavailable
			}
			var res sql.Result
			var err error
			if deleting {
				res, err = tx.ExecContext(ctx, "DELETE FROM devices WHERE id=?", canonical)
			} else {
				res, err = tx.ExecContext(ctx, "UPDATE devices SET profile=? WHERE id=?", profile, canonical)
			}
			if err != nil {
				return err
			}
			n, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if n != 1 {
				return errors.New("device unavailable")
			}
			if deleting {
				for _, key := range keys {
					if _, err = tx.Exec("INSERT INTO support_tunnel_revocations VALUES(?,?)", key, session.ID); err != nil {
						return err
					}
				}
				if len(keys) > 0 {
					if err = s.audit(tx, c, session.AccountID, session.ID, session.GrantID, "tunnel_revoke", "unknown", "external_intent"); err != nil {
						return err
					}
				}
				if _, err = tx.ExecContext(ctx, "DELETE FROM peer_api_tokens WHERE peer_ip=?", canonical); err != nil {
					return err
				}
				if _, err = tx.ExecContext(ctx, "DELETE FROM wg_peers WHERE ip=?", canonical); err != nil {
					return err
				}
			}
			return s.audit(tx, c, session.AccountID, session.ID, session.GrantID, r.URL.Path, "success", "mutation_committed", r.Method)
		})
		if err != nil {
			return err
		}
		if deleting {
			registry.forgetMemoryLocked(canonical)
		} else {
			registry.profileByIP[canonical] = profile
			for alias, target := range registry.canonicalByAlias {
				if target == canonical {
					registry.profileByIP[alias] = profile
				}
			}
		}
		return nil
	}
	if err := mutate(); err != nil {
		return err
	}
	// snapshot holds the agent lock before reading onboard profiles. Release
	// onboard first so deletion cannot invert that order and block revocation.
	if deleting && w.g.agents != nil {
		w.g.agents.Delete(canonical)
	}
	return s.drainTunnelRevocations()
}

// Called under support admission lock. Durable removals survive a lost RPC reply
// or process crash. Missing wg_peers rows also prevent replay on restart.
func (s *supportStore) drainTunnelRevocations() error {
	rows, err := s.conn.QueryContext(context.Background(), "SELECT pubkey,session_id FROM support_tunnel_revocations")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type removal struct{ key, session string }
	var pending []removal
	for rows.Next() {
		var v removal
		if err = rows.Scan(&v.key, &v.session); err != nil {
			_ = rows.Close()
			return err
		}
		pending = append(pending, v)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, v := range pending {
		var c provider.Case
		var account, grant string
		err = s.conn.QueryRowContext(context.Background(), "SELECT case_id,technician_id,technician_name,assignment_revision,account_id,grant_id FROM support_sessions WHERE id=?", v.session).Scan(&c.ID, &c.TechnicianID, &c.TechnicianName, &c.Revision, &account, &grant)
		if err != nil {
			return err
		}
		externalErr := globalWG.removeSupportPeers([]string{v.key})
		outcome, reason := "success", "external_complete"
		if externalErr != nil {
			outcome, reason = "failure", "external_failure"
		}
		err = s.transaction(func(tx *sql.Tx) error {
			if err := s.audit(tx, c, account, v.session, grant, "tunnel_revoke", outcome, reason); err != nil {
				return err
			}
			if externalErr == nil {
				_, err := tx.Exec("DELETE FROM support_tunnel_revocations WHERE pubkey=?", v.key)
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
		if externalErr != nil {
			return externalErr
		}
	}
	return nil
}
