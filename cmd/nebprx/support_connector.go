package main

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Nebuless/nebprx/internal/provider"
	"github.com/Nebuless/nebprx/internal/supportwire"
	"github.com/coder/websocket"
)

type supportConfig struct {
	Broker       string
	Organization string
	Installation string
	Credential   string
	PrivateKey   []byte
	ProviderKey  []byte
	Pepper       []byte
}
type supportConnector struct {
	config  supportConfig
	store   *supportStore
	gateway *Gateway
	client  *http.Client
	mu      sync.Mutex
	link    *supportwire.Link
}

func (c *supportConnector) run(ctx context.Context) (result error) {
	if !c.store.enabled() {
		return errors.New("support disabled")
	}
	u, err := url.Parse(c.config.Broker)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return errors.New("support broker requires exact HTTPS origin")
	}
	u.Scheme = "wss"
	u.Path = "/__connector/connect"
	u.RawQuery = url.Values{"organization": {c.config.Organization}, "installation": {c.config.Installation}}.Encode()
	header := http.Header{"Authorization": {"Bearer " + c.config.Credential}}
	conn, response, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{HTTPClient: c.client, HTTPHeader: header})
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		return errors.New("support connector unavailable")
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(16384)
	handshake, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, b, err := conn.Read(handshake)
	if err != nil {
		return err
	}
	var challenge struct{ Nonce, Proof string }
	if json.Unmarshal(b, &challenge) != nil || challenge.Nonce == "" || !supportwire.CheckProof(c.config.ProviderKey, challenge.Proof, "provider", challenge.Nonce, c.config.Organization, c.config.Installation) {
		return errors.New("broker proof rejected")
	}
	proof, _ := json.Marshal(map[string]string{"Proof": supportwire.Proof(c.config.PrivateKey, "gateway", challenge.Nonce, c.config.Organization, c.config.Installation)})
	if err = conn.Write(handshake, websocket.MessageText, proof); err != nil {
		return err
	}
	var admission sync.Mutex
	var sequence uint64
	w := &webMux{g: c.gateway, onboard: c.gateway.onboard}
	link := supportwire.NewLink(conn, func(ctx context.Context, f supportwire.Frame) supportwire.Frame {
		if f.Kind == "management_health" {
			h := supportwire.ManagementHealth{Connector: "connected", Registry: "disabled"}
			if !c.store.enabled() {
				h.Connector = "disabled"
			} else if c.gateway.supportRegistryHealth != nil {
				h.Registry = c.gateway.supportRegistryHealth(ctx)
			}
			b, _ := json.Marshal(h)
			return supportwire.Frame{Body: b}
		}
		if f.Kind != "dashboard" {
			return supportwire.Frame{Error: "unsupported control"}
		}
		admission.Lock()
		defer admission.Unlock()
		e, err := supportwire.Verify(ed25519.PublicKey(c.config.ProviderKey), f.Signed, challenge.Nonce, c.config.Organization, c.config.Installation, sequence, time.Now())
		if err != nil || e.BodyDigest != supportwire.Hash(f.Body) {
			return supportwire.Frame{Status: 403, Error: "envelope denied"}
		}
		sequence = e.Sequence
		return w.dispatchSupport(ctx, c.store, e, f.Body, f.Headers)
	})
	c.mu.Lock()
	c.link = link
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.link = nil
		c.mu.Unlock()
		cleanupErr := c.store.revokeAllSupport(systemSupportActor, "connector_disconnect", false)
		if cleanupErr != nil {
			slog.Error("Support disconnect revocation persistence failed; admission blocked; inspect storage")
		}
		result = errors.Join(result, cleanupErr)
	}()
	return link.Run(ctx)
}
func (c *supportConnector) currentCase(ctx context.Context, id string) (provider.Case, error) {
	c.mu.Lock()
	link := c.link
	c.mu.Unlock()
	if link == nil {
		return provider.Case{}, errors.New("support connector unavailable")
	}
	b, _ := json.Marshal(map[string]string{"Case": id})
	f, err := link.Call(ctx, supportwire.Frame{Kind: "case_current", Body: b})
	if err != nil {
		return provider.Case{}, err
	}
	var result provider.Case
	if err = json.Unmarshal(f.Body, &result); err != nil {
		return result, err
	}
	if err = c.store.mirrorCase(result); err != nil {
		return result, err
	}
	return result, nil
}
func (c *supportConnector) submitCase(ctx context.Context, intent, account, summary, category string) (provider.Case, error) {
	c.mu.Lock()
	link := c.link
	c.mu.Unlock()
	if link == nil {
		return provider.Case{}, errors.New("support connector unavailable")
	}
	b, _ := json.Marshal(map[string]string{"Intent": intent, "Account": account, "Summary": summary, "Category": category})
	f, err := link.Call(ctx, supportwire.Frame{Kind: "case_submit", Body: b})
	if err != nil {
		return provider.Case{}, err
	}
	var result provider.Case
	if err = json.Unmarshal(f.Body, &result); err != nil {
		return result, err
	}
	return result, c.store.mirrorCase(result)
}
func validSupportOrigin(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == "" && !strings.Contains(u.Host, "..")
}
