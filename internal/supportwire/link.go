package supportwire

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// Link carries only typed dashboard/control frames on an authenticated connection.
// ponytail: eight concurrent requests per installation, no replay or resumption.
type Link struct {
	Conn    *websocket.Conn
	Handler func(context.Context, Frame) Frame
	write   sync.Mutex
	mu      sync.Mutex
	pending map[string]chan Frame
	slots   chan struct{}
	done    chan struct{}
	once    sync.Once
}

// NewLink creates a bounded request multiplexer over an authenticated WebSocket.
func NewLink(c *websocket.Conn, handler func(context.Context, Frame) Frame) *Link {
	return &Link{Conn: c, Handler: handler, pending: map[string]chan Frame{}, slots: make(chan struct{}, 8), done: make(chan struct{})}
}

// Close terminates the connection and unblocks pending calls.
func (l *Link) Close() { l.once.Do(func() { close(l.done); _ = l.Conn.CloseNow() }) }

// Send serializes and writes one size-bounded frame.
func (l *Link) Send(ctx context.Context, f Frame) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	if len(b) > 6<<20 {
		return errors.New("frame too large")
	}
	l.write.Lock()
	defer l.write.Unlock()
	return l.Conn.Write(ctx, websocket.MessageText, b)
}

// Call sends a request and waits for its response within the call deadline.
func (l *Link) Call(ctx context.Context, f Frame) (Frame, error) {
	select {
	case l.slots <- struct{}{}:
		defer func() { <-l.slots }()
	default:
		return Frame{}, errors.New("connector busy")
	}
	if f.ID == "" {
		f.ID = uuid.NewString()
	}
	ch := make(chan Frame, 1)
	l.mu.Lock()
	if _, exists := l.pending[f.ID]; exists {
		l.mu.Unlock()
		return Frame{}, errors.New("duplicate operation ID")
	}
	l.pending[f.ID] = ch
	l.mu.Unlock()
	defer func() { l.mu.Lock(); delete(l.pending, f.ID); l.mu.Unlock() }()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := l.Send(ctx, f); err != nil {
		return Frame{}, err
	}
	select {
	case r := <-ch:
		if r.Error != "" {
			return r, errors.New("remote operation unavailable")
		}
		return r, nil
	case <-l.done:
		return Frame{}, errors.New("connector disconnected")
	case <-ctx.Done():
		l.Close()
		return Frame{}, ctx.Err()
	}
}

// Run dispatches incoming requests and responses until the link closes.
func (l *Link) Run(ctx context.Context) error {
	defer l.Close()
	l.Conn.SetReadLimit(6 << 20)
	active := make(chan struct{}, 8)
	for {
		_, b, err := l.Conn.Read(ctx)
		if err != nil {
			return err
		}
		var f Frame
		if err = DecodeFrame(b, &f); err != nil {
			return err
		}
		if f.Kind == "response" {
			l.mu.Lock()
			ch := l.pending[f.ID]
			l.mu.Unlock()
			if ch != nil {
				select {
				case ch <- f:
				default:
				}
			}
			continue
		}
		if len(f.Body) > MaxBody {
			return errors.New("request too large")
		}
		select {
		case active <- struct{}{}:
		default:
			return errors.New("connector concurrency exceeded")
		}
		go func(f Frame) {
			defer func() { <-active }()
			requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			r := Frame{Error: "operation unavailable"}
			if l.Handler != nil {
				r = l.Handler(requestCtx, f)
			}
			r.Kind = "response"
			r.ID = f.ID
			if err := l.Send(requestCtx, r); err != nil {
				l.Close()
			}
		}(f)
	}
}

// DecodeFrame decodes a JSON frame within the transport size limit.
func DecodeFrame(b []byte, v *Frame) error {
	if len(b) > 6<<20 {
		return errors.New("frame too large")
	}
	return json.Unmarshal(b, v)
}
