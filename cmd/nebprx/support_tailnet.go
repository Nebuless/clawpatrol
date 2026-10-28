package main

import (
	"context"
	"fmt"
	"github.com/Nebuless/nebprx/internal/config"
	"time"
)

func (w *webMux) tailnetAuthority() string {
	if !w.g.cfg.Load().IsTailscaleEnabled() {
		return ""
	}
	authority := w.g.cfg.Load().Join().ControlURL
	if authority == "" {
		authority = "https://controlplane.tailscale.com"
	}
	return authority
}

// Read current server-owned local-client membership on every admission.
// Login is display/allowlist data, never durable identity. No browser headers.
func (w *webMux) tailnetMembership(authority, subject, name string) bool {
	if authority == "" || authority != w.tailnetAuthority() || subject == "" || subject == "0" || !config.MatchDashboardOperator(name, w.g.cfg.Load().Operators()) || w.g.agents == nil {
		return false
	}
	w.g.agents.mu.RLock()
	lc := w.g.agents.lc
	w.g.agents.mu.RUnlock()
	if lc == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	status, err := lc.Status(ctx)
	if err != nil || status == nil || status.BackendState != "Running" {
		return false
	}
	for _, peer := range status.Peer {
		if peer == nil || fmt.Sprint(peer.UserID) != subject {
			continue
		}
		user, ok := status.User[peer.UserID]
		if ok && user.LoginName == name {
			return true
		}
	}
	return false
}
