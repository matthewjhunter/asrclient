package httpcore

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/matthewjhunter/asrclient"
)

func TestPCMToWav_HeaderShape(t *testing.T) {
	pcm := bytes.Repeat([]byte{0x01, 0x00}, 16000) // 1 second of audio
	wav := PCMToWav(pcm)

	if len(wav) != 44+len(pcm) {
		t.Fatalf("wav length: got %d want %d", len(wav), 44+len(pcm))
	}
	if string(wav[0:4]) != "RIFF" {
		t.Errorf("RIFF tag missing")
	}
	if string(wav[8:12]) != "WAVE" {
		t.Errorf("WAVE tag missing")
	}
	if string(wav[12:16]) != "fmt " {
		t.Errorf("fmt tag missing")
	}
	if string(wav[36:40]) != "data" {
		t.Errorf("data tag missing")
	}
	if got := binary.LittleEndian.Uint32(wav[24:28]); got != 16000 {
		t.Errorf("sample rate: got %d want 16000", got)
	}
	if got := binary.LittleEndian.Uint16(wav[22:24]); got != 1 {
		t.Errorf("channels: got %d want 1", got)
	}
	if got := binary.LittleEndian.Uint16(wav[34:36]); got != 16 {
		t.Errorf("bits per sample: got %d want 16", got)
	}
}

func TestPostTranscription_RoundTrip(t *testing.T) {
	pcm := bytes.Repeat([]byte{0x12, 0x34}, 1280) // 1 frame
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method: got %q want POST", r.Method)
		}
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Errorf("Path: got %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("Authorization: got %q want %q", got, "Bearer sk-test")
		}

		ct, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || ct != "multipart/form-data" {
			t.Fatalf("Content-Type: %q (%v)", r.Header.Get("Content-Type"), err)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		fields := map[string]string{}
		var fileBytes []byte
		var fileName string
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart: %v", err)
			}
			b, _ := io.ReadAll(p)
			if p.FormName() == "file" {
				fileBytes = b
				fileName = p.FileName()
			} else {
				fields[p.FormName()] = string(b)
			}
		}
		if fields["model"] != "whisper-1" {
			t.Errorf("model: got %q", fields["model"])
		}
		if fields["language"] != "en" {
			t.Errorf("language: got %q", fields["language"])
		}
		if fields["prompt"] != "biased term" {
			t.Errorf("prompt: got %q", fields["prompt"])
		}
		if !strings.HasSuffix(fileName, ".wav") {
			t.Errorf("file name: %q", fileName)
		}
		if string(fileBytes[0:4]) != "RIFF" {
			t.Errorf("uploaded file is not WAV")
		}
		if len(fileBytes) != 44+len(pcm) {
			t.Errorf("uploaded length: got %d want %d", len(fileBytes), 44+len(pcm))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"hello world","language":"en","duration":1.25}`))
	}))
	defer srv.Close()

	tr, err := PostTranscription(context.Background(), srv.Client(), Request{
		Endpoint: srv.URL + "/v1/audio/transcriptions",
		APIKey:   "sk-test",
		Model:    "whisper-1",
		Language: "en",
		Prompt:   "biased term",
		PCMAudio: pcm,
	})
	if err != nil {
		t.Fatalf("PostTranscription: %v", err)
	}
	if tr.Text != "hello world" {
		t.Errorf("Text: got %q", tr.Text)
	}
	if tr.Language != "en" {
		t.Errorf("Language: got %q", tr.Language)
	}
	if tr.Duration != 1250*time.Millisecond {
		t.Errorf("Duration: got %v want 1.25s", tr.Duration)
	}
}

func TestPostTranscription_NoAPIKey_OmitsHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization should be empty, got %q", got)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":""}`))
	}))
	defer srv.Close()

	_, err := PostTranscription(context.Background(), srv.Client(), Request{
		Endpoint: srv.URL,
		Model:    "whisper-1",
		PCMAudio: []byte{0x00, 0x00},
	})
	if err != nil {
		t.Fatalf("PostTranscription: %v", err)
	}
}

func TestPostTranscription_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer srv.Close()

	_, err := PostTranscription(context.Background(), srv.Client(), Request{
		Endpoint: srv.URL,
		Model:    "whisper-1",
		APIKey:   "sk-bad",
		PCMAudio: []byte{0x00, 0x00},
	})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention status code: %v", err)
	}
}

func TestPostTranscription_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not json`))
	}))
	defer srv.Close()

	_, err := PostTranscription(context.Background(), srv.Client(), Request{
		Endpoint: srv.URL,
		Model:    "whisper-1",
		PCMAudio: []byte{0x00, 0x00},
	})
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestPostTranscription_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := PostTranscription(ctx, srv.Client(), Request{
		Endpoint: srv.URL,
		Model:    "whisper-1",
		PCMAudio: []byte{0x00, 0x00},
	})
	if err == nil {
		t.Fatal("expected error for cancelled ctx")
	}
}

func TestHealthyHEAD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("Method: got %q want HEAD", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := HealthyHEAD(context.Background(), srv.Client(), srv.URL); err != nil {
		t.Errorf("HealthyHEAD: %v", err)
	}
}

func TestHealthyHEAD_FailsOnNetworkError(t *testing.T) {
	// Pick a port that should fail fast.
	if err := HealthyHEAD(context.Background(), &http.Client{Timeout: 200 * time.Millisecond}, "http://127.0.0.1:1/"); err == nil {
		t.Fatal("expected dial failure")
	}
}

// silence unused-import nags when only Transcript fields are exercised
var _ = asrclient.Transcript{}
