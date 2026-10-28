package main

import (
	"database/sql"
	"errors"
	"net/netip"
	"regexp"
	"slices"
	"strings"

	"github.com/Nebuless/nebprx/internal/provider"
)

// Deliberately not a URL: credentials, arbitrary query keys and onboarding
// tokens have no representation in saved dashboard preferences.
type dashboardContext struct {
	Profile string                  `json:"profile,omitempty"`
	Theme   string                  `json:"theme,omitempty"`
	Route   dashboardContextRoute   `json:"route,omitempty"`
	Filters dashboardContextFilters `json:"filters,omitempty"`
}
type dashboardContextRoute struct {
	Page      string `json:"page,omitempty"`
	DeviceIP  string `json:"device_ip,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}
type dashboardContextFilters struct {
	Range    string `json:"range,omitempty"`
	Color    string `json:"color,omitempty"`
	Scale    string `json:"scale,omitempty"`
	DeviceIP string `json:"device_ip,omitempty"`
	Host     string `json:"host,omitempty"`
}

const dashboardContextLimit = 4096

var dashboardRequestID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var dashboardHost = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)

func validContextIP(ip string) bool {
	if ip == "" {
		return true
	}
	a, err := netip.ParseAddr(ip)
	return err == nil && a.Zone() == "" && len(ip) <= 45
}
func (w *webMux) validateDashboardContext(c dashboardContext) error {
	bad := errors.New("invalid saved dashboard context")
	if len(c.Profile) > 128 || c.Profile != "" && !slices.Contains(orderedProfileNames(w.g.cfg.Load().Policy), c.Profile) {
		return bad
	}
	if !slices.Contains([]string{"", "light", "dark", "system"}, c.Theme) {
		return bad
	}
	r, f := c.Route, c.Filters
	if !slices.Contains([]string{"", "main", "devices", "device", "analytics", "request", "settings", "plugins", "account"}, r.Page) {
		return bad
	}
	if !validContextIP(r.DeviceIP) || r.DeviceIP != "" && r.Page != "device" && r.Page != "analytics" || r.Page == "device" && r.DeviceIP == "" {
		return bad
	}
	if r.RequestID != "" && (r.Page != "request" || !dashboardRequestID.MatchString(r.RequestID)) || r.Page == "request" && r.RequestID == "" {
		return bad
	}
	if !slices.Contains([]string{"", "1m", "5m", "15m", "30m", "1h", "6h", "24h"}, f.Range) || !slices.Contains([]string{"", "device", "host", "status"}, f.Color) || !slices.Contains([]string{"", "log", "linear"}, f.Scale) {
		return bad
	}
	if !validContextIP(f.DeviceIP) || len(f.Host) > 253 || f.Host != "" && !dashboardHost.MatchString(f.Host) {
		return bad
	}
	return nil
}
func (w *webMux) loadDashboardContext(accountID string) (dashboardContext, int64, error) {
	var c dashboardContext
	var raw string
	var revision int64
	err := w.g.db.QueryRow("SELECT context_json,revision FROM dashboard_context WHERE account_id=?", accountID).Scan(&raw, &revision)
	if err == sql.ErrNoRows {
		return c, 0, nil
	}
	if err != nil {
		return c, revision, err
	}
	if len(raw) > dashboardContextLimit || provider.DecodeStrict(strings.NewReader(raw), dashboardContextLimit, &c) != nil {
		return c, revision, errors.New("invalid saved dashboard context")
	}
	return c, revision, w.validateDashboardContext(c)
}
