//go:build !linux

package main

// The self-forking daemon is Linux-only. macOS has the NE provider
// (which already serves the equivalent role); other platforms aren't
// supported for `nebprx run`. Stubs let the `daemon` subcommand
// compile cross-platform — it just refuses to run.

func runDaemon(_ []string) {
	fail("nebprx daemon-internal: linux only")
}
