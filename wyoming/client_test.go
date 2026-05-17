package wyoming

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/matthewjhunter/asrclient"
)

// pairedClient returns a *Client bound to one end of a net.Pipe and the
// other end as a raw net.Conn for the test to drive as a fake server.
// The Client's Dial path is bypassed by injecting the conn directly via
// the unexported field — kept self-contained inside the package test.
func pairedClient(t *testing.T) (*Client, net.Conn) {
	t.Helper()
	clientSide, serverSide := net.Pipe()
	c := &Client{addr: "pipe", dialTimeout: time.Second, conn: NewConn(clientSide)}
	t.Cleanup(func() {
		_ = c.Close()
		_ = serverSide.Close()
	})
	return c, serverSide
}

func TestClient_TranscribeStreamsExpectedSequence(t *testing.T) {
	c, server := pairedClient(t)
	br := bufio.NewReader(server)

	// 1280 samples (1 frame) of int16 zeros + 1 frame of int16 ones.
	audio := make([]byte, asrclient.FrameBytes*2+100) // not a perfect multiple → exercises the trailing partial chunk
	for i := range audio {
		audio[i] = byte(i & 0xFF)
	}

	done := make(chan struct {
		t   asrclient.Transcript
		err error
	}, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		tr, err := c.Transcribe(ctx, audio, asrclient.Options{})
		done <- struct {
			t   asrclient.Transcript
			err error
		}{tr, err}
	}()

	// Server side: read events in order and verify the protocol shape,
	// then send a transcript event back.
	events := []string{TypeAudioStart, TypeAudioChunk, TypeAudioChunk, TypeAudioChunk, TypeAudioStop}
	totalPayload := 0
	var firstStartData map[string]any
	for i, want := range events {
		ev, err := ReadEvent(br)
		if err != nil {
			t.Fatalf("event %d: ReadEvent: %v", i, err)
		}
		if ev.Type != want {
			t.Fatalf("event %d: got Type %q, want %q", i, ev.Type, want)
		}
		switch ev.Type {
		case TypeAudioStart:
			firstStartData = ev.Data
		case TypeAudioChunk:
			totalPayload += len(ev.Payload)
		}
	}
	if firstStartData["rate"] != float64(asrclient.SampleRateHz) {
		t.Errorf("audio-start rate: got %v want %d", firstStartData["rate"], asrclient.SampleRateHz)
	}
	if firstStartData["width"] != float64(asrclient.SampleWidth) {
		t.Errorf("audio-start width: got %v want %d", firstStartData["width"], asrclient.SampleWidth)
	}
	if firstStartData["channels"] != float64(asrclient.Channels) {
		t.Errorf("audio-start channels: got %v want %d", firstStartData["channels"], asrclient.Channels)
	}
	if totalPayload != len(audio) {
		t.Errorf("payload bytes: got %d want %d (chunking lost data)", totalPayload, len(audio))
	}

	// Reply with a transcript and (optionally) confound the client by
	// pushing an unrelated info event first.
	if err := WriteEvent(server, Event{Type: TypeInfo, Data: map[string]any{"version": "test"}}); err != nil {
		t.Fatalf("WriteEvent info: %v", err)
	}
	if err := WriteEvent(server, Transcript("hello world")); err != nil {
		t.Fatalf("WriteEvent transcript: %v", err)
	}

	res := <-done
	if res.err != nil {
		t.Fatalf("Transcribe: %v", res.err)
	}
	if res.t.Text != "hello world" {
		t.Errorf("Transcript.Text: got %q want %q", res.t.Text, "hello world")
	}
}

func TestClient_TranscribeChunksAtFrameSize(t *testing.T) {
	c, server := pairedClient(t)
	br := bufio.NewReader(server)

	// Three full frames + half a frame.
	frames := 3
	tail := asrclient.FrameBytes / 2
	audio := bytes.Repeat([]byte{0xAB}, asrclient.FrameBytes*frames+tail)

	done := make(chan error, 1)
	go func() {
		_, err := c.Transcribe(context.Background(), audio, asrclient.Options{})
		done <- err
	}()

	// audio-start
	ev, err := ReadEvent(br)
	if err != nil || ev.Type != TypeAudioStart {
		t.Fatalf("audio-start: ev=%+v err=%v", ev, err)
	}

	// 3 full chunks of FrameBytes
	for i := 0; i < frames; i++ {
		ev, err := ReadEvent(br)
		if err != nil {
			t.Fatalf("chunk %d: %v", i, err)
		}
		if ev.Type != TypeAudioChunk {
			t.Fatalf("chunk %d: type %q", i, ev.Type)
		}
		if len(ev.Payload) != asrclient.FrameBytes {
			t.Errorf("chunk %d: payload %d bytes, want %d", i, len(ev.Payload), asrclient.FrameBytes)
		}
	}

	// One trailing chunk of `tail` bytes
	ev, err = ReadEvent(br)
	if err != nil {
		t.Fatalf("trailing chunk: %v", err)
	}
	if ev.Type != TypeAudioChunk {
		t.Errorf("trailing chunk type: %q", ev.Type)
	}
	if len(ev.Payload) != tail {
		t.Errorf("trailing chunk payload: %d bytes, want %d", len(ev.Payload), tail)
	}

	// audio-stop
	ev, err = ReadEvent(br)
	if err != nil || ev.Type != TypeAudioStop {
		t.Fatalf("audio-stop: ev=%+v err=%v", ev, err)
	}

	// Send a transcript so the goroutine completes.
	_ = WriteEvent(server, Transcript(""))

	if err := <-done; err != nil {
		t.Errorf("Transcribe: %v", err)
	}
}

func TestClient_TranscribeReturnsLanguage(t *testing.T) {
	c, server := pairedClient(t)
	br := bufio.NewReader(server)

	go func() {
		// Drain audio-start + audio-stop (no chunks for empty audio).
		_, _ = ReadEvent(br)
		_, _ = ReadEvent(br)
		ev := Event{Type: TypeTranscript, Data: map[string]any{"text": "bonjour", "language": "fr"}}
		_ = WriteEvent(server, ev)
	}()
	tr, err := c.Transcribe(context.Background(), nil, asrclient.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if tr.Text != "bonjour" || tr.Language != "fr" {
		t.Errorf("got %+v", tr)
	}
}

func TestClient_TranscribeContextCancel(t *testing.T) {
	c, server := pairedClient(t)

	// Read just enough so the client doesn't block on Write before we
	// can cancel — drain one event so the pipe has space.
	go func() {
		br := bufio.NewReader(server)
		_, _ = ReadEvent(br)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before sending so the loop sees ctx.Err()

	_, err := c.Transcribe(ctx, bytes.Repeat([]byte{0xCC}, asrclient.FrameBytes*4), asrclient.Options{})
	if err == nil {
		t.Fatal("expected error for cancelled ctx")
	}
}

func TestClient_PingRoundTrip(t *testing.T) {
	c, server := pairedClient(t)
	br := bufio.NewReader(server)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ev, err := ReadEvent(br)
		if err != nil || ev.Type != TypeDescribe {
			t.Errorf("server: expected describe, got %+v err=%v", ev, err)
			return
		}
		if err := WriteEvent(server, Event{Type: TypeInfo, Data: map[string]any{"version": "test"}}); err != nil {
			t.Errorf("WriteEvent info: %v", err)
		}
	}()

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	wg.Wait()
}

func TestClient_CloseClosesConn(t *testing.T) {
	c, server := pairedClient(t)
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Subsequent server read should hit EOF.
	br := bufio.NewReader(server)
	if _, err := ReadEvent(br); err == nil {
		t.Error("expected error on closed pipe")
	}
}

func TestClient_DialFailureSurfaces(t *testing.T) {
	// Pick an unreachable address. 0 port + 127.0.0.1 should fail
	// fast on dial.
	c := NewClient("127.0.0.1:1", WithDialTimeout(200*time.Millisecond))
	defer c.Close()
	_, err := c.Transcribe(context.Background(), nil, asrclient.Options{})
	if err == nil {
		t.Fatal("expected dial error")
	}
}

func TestClient_ServerCloseBeforeTranscript(t *testing.T) {
	c, server := pairedClient(t)
	br := bufio.NewReader(server)

	go func() {
		// Drain audio-start, audio-stop (no audio bytes), then close.
		_, _ = ReadEvent(br)
		_, _ = ReadEvent(br)
		_ = server.Close()
	}()

	_, err := c.Transcribe(context.Background(), nil, asrclient.Options{})
	if err == nil {
		t.Fatal("expected error on server close before transcript")
	}
	if !errors.Is(err, errors.New("dummy")) && err.Error() == "" {
		t.Errorf("got error: %v", err)
	}
}
