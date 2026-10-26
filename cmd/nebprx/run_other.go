//go:build !linux && !darwin

package main

func runRun(args []string) {
	fail("nebprx run is only supported on Linux.")
}
