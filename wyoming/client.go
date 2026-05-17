package wyoming

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/matthewjhunter/asrclient"
)

// Client is a Wyoming-protocol implementation of asrclient.Transcriber.
//
// One Client may be reused across many Transcribe calls. The TCP
// connection is opened lazily on the first call and held open
// thereafter; on a transport error the connection is dropped and the
// next call redials. There is no background reconnect goroutine.
type Client struct {
	addr        string
	dialTimeout time.Duration

	mu   sync.Mutex
	conn *Conn
}

// Option configures a Client.
type Option func(*Client)

// WithDialTimeout overrides the TCP dial timeout (default 5s).
// Zero means no timeout.
func WithDialTimeout(d time.Duration) Option {
	return func(c *Client) { c.dialTimeout = d }
}

// NewClient constructs a Client pointed at addr (host:port). The
// connection is not opened until the first Transcribe or Ping call.
func NewClient(addr string, opts ...Option) *Client {
	c := &Client{addr: addr, dialTimeout: 5 * time.Second}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Transcribe implements asrclient.Transcriber.
//
// audio must be PCM in the module's locked format (16 kHz mono int16-LE,
// see asrclient.FrameBytes). The implementation chunks it into
// FrameBytes-sized audio-chunk events at the protocol's expected cadence,
// sends audio-stop, and returns the next transcript event the server
// emits (other events are ignored).
func (c *Client) Transcribe(ctx context.Context, audio []byte, _ asrclient.Options) (asrclient.Transcript, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := c.acquireLocked()
	if err != nil {
		return asrclient.Transcript{}, err
	}
	stopWatch := c.bindCtxLocked(ctx, conn, 0)
	defer stopWatch()

	if err := conn.Write(AudioStart(asrclient.SampleRateHz, asrclient.SampleWidth, asrclient.Channels)); err != nil {
		c.dropLocked()
		return asrclient.Transcript{}, fmt.Errorf("wyoming: write audio-start: %w", err)
	}

	for i := 0; i < len(audio); i += asrclient.FrameBytes {
		end := min(i+asrclient.FrameBytes, len(audio))
		if err := conn.Write(AudioChunk(asrclient.SampleRateHz, asrclient.SampleWidth, asrclient.Channels, audio[i:end])); err != nil {
			c.dropLocked()
			return asrclient.Transcript{}, fmt.Errorf("wyoming: write audio-chunk: %w", err)
		}
		if err := ctx.Err(); err != nil {
			c.dropLocked()
			return asrclient.Transcript{}, err
		}
	}

	if err := conn.Write(AudioStop()); err != nil {
		c.dropLocked()
		return asrclient.Transcript{}, fmt.Errorf("wyoming: write audio-stop: %w", err)
	}

	for {
		ev, err := conn.Read()
		if err != nil {
			c.dropLocked()
			if errors.Is(err, io.EOF) {
				return asrclient.Transcript{}, errors.New("wyoming: server closed before transcript")
			}
			return asrclient.Transcript{}, fmt.Errorf("wyoming: read: %w", err)
		}
		if ev.Type != TypeTranscript {
			continue
		}
		text, _ := TranscriptText(ev)
		lang, _ := ev.Data["language"].(string)
		return asrclient.Transcript{Text: text, Language: lang}, nil
	}
}

// Ping implements asrclient.Transcriber by sending describe and waiting
// for the corresponding info response.
func (c *Client) Ping(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := c.acquireLocked()
	if err != nil {
		return err
	}
	stopWatch := c.bindCtxLocked(ctx, conn, 5*time.Second)
	defer stopWatch()

	if err := conn.Write(Describe()); err != nil {
		c.dropLocked()
		return fmt.Errorf("wyoming: write describe: %w", err)
	}
	for {
		ev, err := conn.Read()
		if err != nil {
			c.dropLocked()
			return fmt.Errorf("wyoming: read info: %w", err)
		}
		if ev.Type == TypeInfo {
			return nil
		}
	}
}

// Close closes any persistent connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

func (c *Client) acquireLocked() (*Conn, error) {
	if c.conn != nil {
		return c.conn, nil
	}
	conn, err := Dial(c.addr, c.dialTimeout)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	return conn, nil
}

func (c *Client) dropLocked() {
	if c.conn == nil {
		return
	}
	_ = c.conn.Close()
	c.conn = nil
}

// bindCtxLocked applies ctx's deadline (or fallback) to conn, and arms
// a context.AfterFunc so that ctx cancellation forces in-flight reads
// and writes to fail immediately. The returned function stops the
// AfterFunc and clears the deadline; callers must defer it.
func (c *Client) bindCtxLocked(ctx context.Context, conn *Conn, fallback time.Duration) func() {
	stop := context.AfterFunc(ctx, func() {
		// A long-past deadline makes any pending or future Read/Write
		// on the conn return immediately with a timeout error.
		_ = conn.SetDeadline(time.Unix(1, 0))
	})
	if d, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(d)
	} else if fallback > 0 {
		_ = conn.SetDeadline(time.Now().Add(fallback))
	} else {
		_ = conn.SetDeadline(time.Time{})
	}
	return func() {
		stop()
		_ = conn.SetDeadline(time.Time{})
	}
}

// Compile-time check that *Client satisfies asrclient.Transcriber.
var _ asrclient.Transcriber = (*Client)(nil)
