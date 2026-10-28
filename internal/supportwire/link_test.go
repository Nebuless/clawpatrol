package supportwire

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestLinkRealWebSocketConcurrencyAndDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		link := NewLink(c, func(_ context.Context, f Frame) Frame { return Frame{Body: f.Body} })
		_ = link.Run(r.Context())
	}))
	defer server.Close()
	c, response, err := websocket.Dial(ctx, "ws"+server.URL[4:], nil)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	link := NewLink(c, nil)
	defer link.Close()
	done := make(chan struct{})
	go func() { _ = link.Run(ctx); close(done) }()
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := link.Call(ctx, Frame{Kind: "echo", Body: []byte("synthetic")})
			if err != nil || string(result.Body) != "synthetic" {
				t.Errorf("echo: %s %v", result.Body, err)
			}
		}()
	}
	wg.Wait()
	link.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("disconnect did not close reader")
	}
	if _, err = link.Call(ctx, Frame{Kind: "echo"}); err == nil {
		t.Fatal("closed link accepted call")
	}
}
