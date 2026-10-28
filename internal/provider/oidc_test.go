package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIntrospectionProfileFailClosed(t *testing.T) {
	claims := map[string]any{"active": true, "sub": "subject", "client_id": "dashboard", "iss": "https://issuer.example", "exp": time.Now().Add(time.Minute).Unix()}
	idp := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "introspection" || password != "secret" {
			t.Error("missing authenticated introspection")
		}
		if err := json.NewEncoder(w).Encode(claims); err != nil {
			t.Error(err)
		}
	}))
	defer idp.Close()
	a := &Authenticator{config: OIDCConfig{Issuer: "https://issuer.example", ClientID: "dashboard", IntrospectionEndpoint: idp.URL, IntrospectionClientID: "introspection"}, introspectionSecret: "secret", client: idp.Client(), now: time.Now}
	if _, err := a.introspect(context.Background(), "token", "subject"); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"active", "sub", "client_id", "iss", "exp"} {
		v := claims[key]
		delete(claims, key)
		if _, err := a.introspect(context.Background(), "token", "subject"); err == nil {
			t.Errorf("accepted missing %s", key)
		}
		claims[key] = v
	}
	for key, value := range map[string]any{"active": false, "sub": "other", "client_id": "other", "iss": "https://other.example", "exp": time.Now().Add(-time.Minute).Unix()} {
		v := claims[key]
		claims[key] = value
		if _, err := a.introspect(context.Background(), "token", "subject"); err == nil {
			t.Errorf("accepted wrong %s", key)
		}
		claims[key] = v
	}
}
