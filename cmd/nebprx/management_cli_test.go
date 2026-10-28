package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestManagementCommandHelp(t *testing.T) {
	if command := os.Getenv("NEBPRX_TEST_MANAGEMENT_HELP"); command != "" {
		switch command {
		case "provider":
			runProvider([]string{"--help"})
		case "support":
			runSupport([]string{"--help"})
		default:
			t.Fatal("unknown management help command")
		}
		return
	}
	for _, command := range []string{"provider", "support"} {
		t.Run(command, func(t *testing.T) {
			process := exec.Command(os.Args[0], "-test.run=^TestManagementCommandHelp$")
			process.Env = append(os.Environ(), "NEBPRX_TEST_MANAGEMENT_HELP="+command)
			process.Dir = t.TempDir()
			output, err := process.CombinedOutput()
			if err != nil {
				t.Fatalf("%s --help must exit successfully: %v", command, err)
			}
			if !strings.Contains(string(output), "Usage of "+command+":") || strings.Contains(string(output), "help requested") {
				t.Fatalf("%s --help did not print ordinary usage", command)
			}
		})
	}
}
