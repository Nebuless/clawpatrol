//go:build darwin

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestDarwinRunUnsupported(t *testing.T) {
	if os.Getenv("NEBPRX_TEST_DARWIN_RUN") == "1" {
		runRun([]string{"sh", "-c", "exit 0"})
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestDarwinRunUnsupported$")
	cmd.Env = append(os.Environ(), "NEBPRX_TEST_DARWIN_RUN=1")
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 {
		t.Fatalf("exit = %v, output = %s", err, out)
	}
	if !strings.Contains(string(out), "nebprx run isn't supported on macOS; use `nebprx join --whole-machine` with a WireGuard gateway") {
		t.Fatalf("unexpected error: %s", out)
	}
}
