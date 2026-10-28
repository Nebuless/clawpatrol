package provider

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Nebuless/nebprx/internal/supportwire"
)

// Management detail is authorized even when disabled, so an operator can recover it.
func (s *Server) installationHealth(ctx context.Context, actor Technician, org, id string) (supportwire.ManagementHealth, error) {
	var enabled, orgEnabled, enrolled bool
	err := s.Store.db.QueryRow("SELECT i.enabled,o.enabled,i.credential_hash IS NOT NULL FROM installations i JOIN organizations o ON o.id=i.organization_id JOIN organization_assignments a ON a.organization_id=o.id WHERE i.id=? AND i.organization_id=? AND a.technician_id=?", id, org, actor.ID).Scan(&enabled, &orgEnabled, &enrolled)
	if err != nil {
		return supportwire.ManagementHealth{}, err
	}
	h := supportwire.ManagementHealth{Connector: "not_enrolled", Registry: "unknown"}
	if !enabled || !orgEnabled {
		h.Connector = "disabled"
		return h, nil
	}
	if !enrolled {
		return h, nil
	}
	h.Connector = "disconnected"
	s.mu.Lock()
	c := s.connections[id]
	s.mu.Unlock()
	if c == nil {
		return h, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	frame, err := c.link.Call(ctx, supportwire.Frame{Kind: "management_health"})
	if err != nil {
		h.Connector = "unavailable"
		return h, nil
	}
	var remote supportwire.ManagementHealth
	if len(frame.Body) > 1024 || json.Unmarshal(frame.Body, &remote) != nil {
		return h, errors.New("invalid management health")
	}
	if remote.Connector != "connected" && remote.Connector != "disabled" {
		return h, errors.New("invalid connector health")
	}
	switch remote.Registry {
	case "disabled", "not_client", "local_only", "unavailable", "sync_failed", "stale", "verified":
	default:
		return h, errors.New("invalid registry health")
	}
	return remote, nil
}
