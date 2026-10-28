//go:build darwin

package main

func runRun(_ []string) {
	fail("nebprx run isn't supported on macOS; use `nebprx join --whole-machine` with a WireGuard gateway")
}
