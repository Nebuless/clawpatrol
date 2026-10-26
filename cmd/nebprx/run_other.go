//go:build !linux && !darwin

package main

func runRun(args []string) {
	fail("nebprx run is not supported on this platform — linux + macOS only.")
}
