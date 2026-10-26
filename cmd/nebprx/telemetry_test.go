package main

import "testing"

func TestTelemetryEndpoint(t *testing.T) {
	const want = "https://prx.nebuless.cloud/api/telemetry/v1/check"
	if telemetryEndpoint != want {
		t.Fatalf("telemetryEndpoint = %q, want %q", telemetryEndpoint, want)
	}
}
