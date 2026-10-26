package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// findGatewayStateLeak returns the path of a gateway sqlite db that
// is readable from the current process, or "" if none is found.
// Almost always means `nebprx run` is being invoked on the
// gateway host itself — the intended workflow runs on a client
// device that joined over WireGuard. The wrapped agent inherits the
// calling user's filesystem access, so the gateway's sqlite db
// (CA private key, OAuth tokens, audit log) would be reachable to it.
//
// This is a UX hint, not a security boundary: even if `nebprx
// run` refuses, the same exposure exists for any agent the operator
// launches directly. The actual fix is locking down state_dir's
// permissions and running the gateway under a dedicated service
// user — see site/doc/getting-started.md.
func findGatewayStateLeak() string {
	candidates := []string{
		"/opt/nebprx/nebprx.db",
		"/opt/nebprx/state/nebprx.db", // legacy
		"/opt/nebprx/oauth/nebprx.db", // legacy
		"/srv/nebprx/nebprx.db",
		"/var/lib/nebprx/nebprx.db",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".nebprx/nebprx.db"),
			filepath.Join(home, ".nebprx/state/nebprx.db"), // legacy
			filepath.Join(home, ".nebprx/oauth/nebprx.db"), // legacy
		)
	}
	for _, p := range candidates {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		_ = f.Close()
		return p
	}
	return ""
}

// warnIfOnGatewayHost prints a stderr hint when a gateway state db
// is readable from the current process. Non-fatal — `nebprx run`
// continues either way.
func warnIfOnGatewayHost() {
	p := findGatewayStateLeak()
	if p == "" {
		return
	}
	fmt.Fprintf(os.Stderr,
		"nebprx: heads up — gateway state db %q is readable from this process.\n"+
			"  `nebprx run` is meant for client devices that joined the gateway over\n"+
			"  WireGuard, not the gateway host itself. The wrapped agent will have direct\n"+
			"  filesystem access to the CA private key, OAuth tokens, and audit log.\n",
		p)
}
