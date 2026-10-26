//go:build !linux

package main

import "net/http"

// Only Linux has a per-process tsnet daemon to claim against.
func claimPeerAPIToken(gateway, deviceCode, hostname, authKey, controlURL, stateDir, caDir string, cli *http.Client) error {
	return nil
}
