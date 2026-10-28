package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Nebuless/nebprx/internal/registry"
)

type registryOptions struct {
	mode, dir, listen, cert, key, sources, upstream, installation, tokenFile, trustFile, inventory string
	allowHTTP                                                                                      bool
	interval                                                                                       time.Duration
}

func registryFlags(fs *flag.FlagSet, prefix string) *registryOptions {
	o := new(registryOptions)
	fs.StringVar(&o.mode, prefix+"mode", "", "registry role: server or client (gateway: empty disables)")
	fs.StringVar(&o.dir, prefix+"state-dir", "", "owner-only state directory (default ~/.nebprx/registry-<mode>)")
	fs.StringVar(&o.listen, prefix+"listen", "", "literal IP:port (server 127.0.0.1:8091, client 127.0.0.1:8092)")
	fs.StringVar(&o.cert, prefix+"tls-cert", "", "PEM TLS certificate chain")
	fs.StringVar(&o.key, prefix+"tls-key", "", "owner-only PEM TLS private key")
	fs.BoolVar(&o.allowHTTP, prefix+"allow-loopback-http", false, "explicit development-only loopback HTTP allowance")
	fs.StringVar(&o.sources, prefix+"sources", "", "server: JSON array of configured ecosystem export URLs")
	fs.StringVar(&o.upstream, prefix+"upstream", "", "client: global registry HTTPS origin")
	fs.StringVar(&o.installation, prefix+"installation", "", "client: enrolled installation ID")
	fs.StringVar(&o.tokenFile, prefix+"token-file", "", "client: owner-only installation bearer-token file")
	fs.StringVar(&o.trustFile, prefix+"trust-key", "", "client: explicitly bootstrapped base64 Ed25519 public-key file")
	fs.StringVar(&o.inventory, prefix+"inventory", "", "client: owner-only inventory JSON (reloaded on periodic sync)")
	fs.DurationVar(&o.interval, prefix+"interval", time.Hour, "refresh/sync interval, 1m..24h")
	return o
}
func (o *registryOptions) config() (registry.Config, error) {
	c := registry.Config{Mode: o.mode, Dir: o.dir, Listen: o.listen, TLSCert: o.cert, TLSKey: o.key, AllowHTTP: o.allowHTTP, Interval: o.interval, InventoryPath: o.inventory}
	if c.Mode != "client" && c.Mode != "server" {
		return c, errors.New("registry mode must be server or client")
	}
	if c.Dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return c, err
		}
		c.Dir = filepath.Join(home, ".nebprx", "registry-"+c.Mode)
	}
	if o.sources != "" {
		if err := readRegistryJSON(o.sources, &c.Sources); err != nil {
			return c, err
		}
	}
	c.Remote = registry.Remote{URL: o.upstream, Installation: o.installation, AllowHTTP: o.allowHTTP}
	if o.tokenFile != "" {
		b, err := registry.ReadSecret(o.tokenFile)
		if err != nil {
			return c, err
		}
		c.Remote.Token = strings.TrimSpace(string(b))
	}
	if o.trustFile != "" {
		f, err := os.Open(o.trustFile)
		if err != nil {
			return c, err
		}
		defer func() { _ = f.Close() }()
		b, err := io.ReadAll(io.LimitReader(f, 1025))
		if err != nil {
			return c, err
		}
		if len(b) > 1024 {
			return c, errors.New("trust key too large")
		}
		c.Remote.PublicKey = strings.TrimSpace(string(b))
	}
	c.Defaults()
	return c, nil
}
func readRegistryJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	d := json.NewDecoder(io.LimitReader(f, registry.MaxRecord+1))
	d.DisallowUnknownFields()
	if err = d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing JSON or size limit")
	}
	return nil
}
func runRegistry(args []string) {
	if err := registryCommand(args, os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "registry: %v\n", err)
		os.Exit(1)
	}
}
func registryCommand(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("registry", flag.ContinueOnError)
	fs.SetOutput(errOut)
	o := registryFlags(fs, "")
	action := fs.String("action", "serve", "serve|init|enroll|import|update|inventory|sync|check|subscribe|status")
	input := fs.String("input", "", "import: complete OSV array/ZIP; inventory: inventory JSON")
	ecosystem := fs.String("ecosystem", "", "import/check/subscribe: npm|PyPI|crates.io|Go")
	name := fs.String("name", "", "check/subscribe: package name")
	version := fs.String("package-version", "", "check/subscribe: exact installed/candidate version")
	public := fs.Bool("public", false, "check/subscribe: explicitly public identity; required before sending candidate")
	online := fs.Bool("online", false, "check: subscribe and verify missing/current scope before checking public candidate")
	output := fs.String("output", "", "enroll: new owner-only bearer-token output file")
	observed := fs.String("observed-at", "", "import: actual export observation RFC3339 time (required, not file mtime)")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(errOut, "usage: nebprx registry --mode server|client [flags]\nPRX vulnerability intelligence, not package artifact mirrors. See doc/registry.md.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	switch *action {
	case "serve", "init", "enroll", "import", "update", "inventory", "sync", "check", "subscribe", "status":
	default:
		return errors.New("unknown registry action")
	}
	c, err := o.config()
	if err != nil {
		return err
	}
	// Validate role-specific settings even for one-shot commands, without requiring TLS to init.
	if *action != "serve" && o.listen != "" {
		return errors.New("--listen only applies to serve")
	}
	check := c
	check.AllowHTTP = true
	check.Listen = "127.0.0.1:0"
	if err = check.Validate(); err != nil {
		return err
	}
	if *action == "serve" {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		svc, err := registry.Start(ctx, c)
		if err != nil {
			return err
		}
		log.Printf("registry: %s listening on %s", c.Mode, svc.Addr)
		select {
		case <-ctx.Done():
		case err = <-svc.Errors:
		}
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return errors.Join(err, svc.Close(shutdown))
	}
	s, err := registry.Open(c.Dir, c.Mode)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	emit := func(v any) error { enc := json.NewEncoder(out); enc.SetIndent("", "  "); return enc.Encode(v) }
	client := func() error {
		if c.Mode != "client" {
			return errors.New("action requires client mode")
		}
		return nil
	}
	loadInventory := func() error {
		if c.InventoryPath == "" {
			return nil
		}
		in, e := registry.LoadInventory(c.InventoryPath)
		if e != nil {
			return e
		}
		return s.SetInventory(in)
	}
	switch *action {
	case "init":
		if c.Mode == "server" {
			key, e := s.PublicKey()
			if e != nil {
				return e
			}
			return emit(map[string]string{"mode": c.Mode, "public_key": key, "state_dir": c.Dir})
		}
		return emit(map[string]string{"mode": c.Mode, "state_dir": c.Dir, "local_token_file": filepath.Join(c.Dir, "local.token")})
	case "enroll":
		return s.Enroll(o.installation, *output)
	case "import":
		t, e := time.Parse(time.RFC3339, *observed)
		if e != nil {
			return errors.New("import requires --observed-at RFC3339 export observation time")
		}
		return s.Import(ctx, *ecosystem, *input, "local-import", t)
	case "update":
		if c.Mode != "server" || len(c.Sources) == 0 {
			return errors.New("update requires server and --sources")
		}
		return s.Refresh(ctx, c.Sources)
	case "inventory":
		if err = client(); err != nil {
			return err
		}
		in, e := registry.LoadInventory(*input)
		if e != nil {
			return e
		}
		return s.SetInventory(in)
	case "sync":
		if err = client(); err != nil {
			return err
		}
		if err = loadInventory(); err != nil {
			return err
		}
		return s.Sync(ctx, c.Remote)
	case "check", "subscribe":
		if err = client(); err != nil {
			return err
		}
		p := registry.Package{Identity: registry.Identity{Ecosystem: *ecosystem, Name: *name}, Version: *version, Public: *public}
		if *action == "subscribe" || *online {
			if err = s.Subscribe(p); err != nil {
				return err
			}
			if err = s.Sync(ctx, c.Remote); err != nil {
				return err
			}
		}
		result, e := s.Check(p, time.Now())
		if e != nil {
			return e
		}
		return emit(result)
	case "status":
		status, e := s.Status(ctx)
		if e != nil {
			return e
		}
		return emit(status)
	}
	return nil
}

// Gateway starts registry before opening gateway listeners. Any serve failure
// terminates process instead of leaving gateway silently without its registry.
var errRegistryDisabled = errors.New("registry disabled")

func startGatewayRegistry(o *registryOptions) (*registry.Service, error) {
	if o.mode == "" {
		return nil, errRegistryDisabled
	}
	c, err := o.config()
	if err != nil {
		return nil, err
	}
	return registry.Start(context.Background(), c)
}
func registryGatewayShutdown(svc *registry.Service, otel func(context.Context) error) func(context.Context) error {
	if svc == nil {
		return otel
	}
	return func(ctx context.Context) error {
		err := svc.Close(ctx)
		if otel != nil {
			err = errors.Join(err, otel(ctx))
		}
		return err
	}
}
