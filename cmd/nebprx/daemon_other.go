//go:build !linux

package main

// The self-forking daemon and `nebprx run` are Linux-only. This stub
// lets the internal daemon subcommand compile on other platforms.

func runDaemon(_ []string) {
	fail("nebprx daemon-internal: linux only")
}
