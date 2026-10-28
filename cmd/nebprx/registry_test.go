package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRegistryCommandFlags(t *testing.T) {
	for _, args := range [][]string{{}, {"--mode", "mirror"}, {"--mode", "server", "--action", "unknown"}, {"--mode", "client", "extra"}, {"--unknown"}, {"--mode", "server", "--listen", "0.0.0.0:0"}} {
		var out, errOut bytes.Buffer
		if err := registryCommand(args, &out, &errOut); err == nil {
			t.Fatal("invalid invocation succeeded", args)
		}
	}
	var out, errOut bytes.Buffer
	if err := registryCommand([]string{"--help"}, &out, &errOut); !errors.Is(err, flag.ErrHelp) || !strings.Contains(errOut.String(), "--mode server|client") {
		t.Fatal(err, errOut.String())
	}
}
func TestRegistryCLIInventoryLifecycle(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "client")
	base := []string{"--mode", "client", "--state-dir", dir}
	run := func(args ...string) string {
		t.Helper()
		var out, errOut bytes.Buffer
		if err := registryCommand(append(append([]string{}, base...), args...), &out, &errOut); err != nil {
			t.Fatal(err, errOut.String())
		}
		return out.String()
	}
	run("--action", "init")
	path := filepath.Join(t.TempDir(), "inventory.json")
	if err := os.WriteFile(path, []byte(`{"graph_complete":false,"packages":[{"ecosystem":"npm","name":"example","version":"1.0.0","public":true,"decision":"deny"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	run("--action", "inventory", "--input", path)
	if text := run("--action", "status"); !strings.Contains(text, `"inventory_packages": 1`) || !strings.Contains(text, `"graph_complete": false`) {
		t.Fatal(text)
	}
	if text := run("--action", "check", "--ecosystem", "npm", "--name", "example", "--package-version", "1.0.0", "--public"); !strings.Contains(text, `"finding": "indeterminate"`) {
		t.Fatal(text)
	}
}
func TestRegistryGatewayOptionsAndLifecycle(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	o := registryFlags(fs, "registry-")
	if err := fs.Parse([]string{"--registry-mode", "client", "--registry-state-dir", filepath.Join(t.TempDir(), "registry"), "--registry-listen", "127.0.0.1:0", "--registry-allow-loopback-http"}); err != nil {
		t.Fatal(err)
	}
	svc, err := startGatewayRegistry(o)
	if err != nil {
		t.Fatal(err)
	}
	if svc == nil {
		t.Fatal("no registry")
	}
	called := false
	shutdown := registryGatewayShutdown(svc, func(context.Context) error { called = true; return nil })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = shutdown(ctx); err != nil || !called {
		t.Fatal(err, called)
	}
	conn, err := net.DialTimeout("tcp", svc.Addr.String(), time.Second)
	if err == nil {
		if err := conn.Close(); err != nil {
			t.Fatal(err)
		}
		t.Fatal("registry listener remains")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ln.Close(); err != nil {
			t.Error(err)
		}
	}()
	o.listen = ln.Addr().String()
	if svc, err = startGatewayRegistry(o); err == nil {
		if err := svc.Close(ctx); err != nil {
			t.Fatal(err)
		}
		t.Fatal("registry bind error ignored")
	}
	o.mode = ""
	if svc, err = startGatewayRegistry(o); !errors.Is(err, errRegistryDisabled) || svc != nil {
		t.Fatal(svc, err)
	}
}
