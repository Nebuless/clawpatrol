package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"github.com/Nebuless/nebprx/internal/provider"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func runProvider(args []string) {
	if err := providerCommand(args, os.Stdout); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "provider:", err)
		os.Exit(1)
	}
}
func providerCommand(args []string, _ io.Writer) error {
	fs := flag.NewFlagSet("provider", flag.ContinueOnError)
	action := fs.String("action", "serve", "serve|org-add|installation-enroll|technician-add|assign|revoke")
	dir := fs.String("state-dir", "", "owner-only independent Provider state directory")
	listen := fs.String("listen", "127.0.0.1:8093", "HTTPS listen address")
	public := fs.String("public-url", "", "exact Provider HTTPS origin")
	domain := fs.String("support-domain", "", "separate wildcard support-session domain")
	cert := fs.String("tls-cert", "", "PEM TLS certificate chain")
	key := fs.String("tls-key", "", "owner-only PEM TLS key")
	oidcFile := fs.String("oidc-config-file", "", "strict OIDC assurance configuration")
	org := fs.String("organization-id", "", "organization ID")
	installation := fs.String("installation-id", "", "durable gateway installation ID")
	name := fs.String("name", "", "display name")
	hosting := fs.String("hosting", "customer", "customer|nebuless")
	tech := fs.String("technician-id", "", "immutable technician ID")
	issuer := fs.String("issuer", "", "technician OIDC issuer")
	subject := fs.String("subject", "", "technician immutable OIDC subject")
	admin := fs.Bool("administrator", false, "enroll Provider administrator")
	output := fs.String("output", "", "new owner-only one-time enrollment-secret output file")
	retention := fs.Int("audit-retention-days", 90, "Provider audit retention (30..365 days)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dir == "" || fs.NArg() != 0 {
		return errors.New("state-dir required; no positional arguments")
	}
	if err := os.MkdirAll(*dir, 0700); err != nil {
		return err
	}
	st, err := os.Stat(*dir)
	if err != nil {
		return err
	}
	if st.Mode().Perm()&0077 != 0 {
		return errors.New("provider state directory must be owner-only")
	}
	store, err := provider.Open(filepath.Join(*dir, "provider.db"))
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	switch *action {
	case "org-add":
		return store.AddOrganization(*org, *name)
	case "technician-add":
		return store.AddTechnician(*tech, *issuer, *subject, *name, *admin)
	case "assign":
		return store.AssignOrganization(*tech, *org)
	case "revoke":
		return store.RevokeTechnician(*tech)
	case "installation-enroll":
		if *output == "" {
			return errors.New("enrollment output file required")
		}
		if err = store.RegisterInstallation(provider.Installation{ID: *installation, OrganizationID: *org, Name: *name, Hosting: *hosting}); err != nil {
			return err
		}
		secret, err := store.Enrollment(*org, *installation)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, err = f.WriteString(secret)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		return closeErr
	case "serve":
		if !validSupportOrigin(*public) || *domain == "" || strings.ContainsAny(*domain, "/: ") || strings.HasSuffix(strings.TrimPrefix(*public, "https://"), "."+*domain) || *cert == "" || *key == "" {
			return errors.New("isolated Provider HTTPS origin, support domain and explicit TLS files required")
		}
		if _, err = provider.ReadPrivate(*key); err != nil {
			return err
		}
		cfg, err := provider.LoadOIDCConfig(*oidcFile)
		if err != nil {
			return err
		}
		signing, err := ensureSupportSecret(filepath.Join(*dir, "support-signing.key"), ed25519.PrivateKeySize)
		if err != nil {
			return err
		}
		encryption, err := ensureSupportSecret(filepath.Join(*dir, "storage-encryption.key"), 32)
		if err != nil {
			return err
		}
		auth, err := provider.NewAuthenticator(cfg, store, encryption, *public, nil)
		if err != nil {
			return err
		}
		handler := &provider.Server{Store: store, Auth: auth, Key: signing, PublicURL: *public, SupportDomain: *domain}
		srv := &http.Server{Addr: *listen, Handler: handler, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13}, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384}
		if err = store.Maintain(time.Now(), *retention); err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		maintenanceDone := make(chan struct{})
		go func() {
			defer close(maintenanceDone)
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case now := <-ticker.C:
					if err := store.Maintain(now, *retention); err != nil {
						slog.Error("Provider retention cleanup failed")
					}
				}
			}
		}()
		defer func() { stop(); <-maintenanceDone }()
		go func() {
			<-ctx.Done()
			shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdown)
		}()
		err = srv.ListenAndServeTLS(*cert, *key)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	default:
		return errors.New("unknown Provider action")
	}
}
