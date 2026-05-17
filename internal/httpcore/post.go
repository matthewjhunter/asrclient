package httpcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/matthewjhunter/asrclient"
)

// Request is the input to PostTranscription, capturing the per-call
// fields plus the per-Backend fields the backend has already resolved.
type Request struct {
	Endpoint string
	APIKey   string // Bearer token; "" omits the Authorization header.
	Model    string
	Language string // optional ISO-639-1 hint
	Prompt   string // optional bias text
	PCMAudio []byte // raw int16-LE PCM in the asrclient frame format
}

// apiResponse is the OpenAI-compatible JSON response shape. Only fields
// the Backend currently surfaces are decoded; everything else is dropped.
type apiResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language"`
	Duration float64 `json:"duration"`
}

// PostTranscription posts a single audio buffer to an OpenAI-compatible
// /v1/audio/transcriptions endpoint and returns the resulting Transcript.
// PCMAudio is wrapped into a WAV container before upload so the server
// has a recognizable container; this is required by both OpenAI and
// whisper-server.
func PostTranscription(ctx context.Context, hc *http.Client, req Request) (asrclient.Transcript, error) {
	body, contentType, err := buildMultipart(req)
	if err != nil {
		return asrclient.Transcript{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.Endpoint, body)
	if err != nil {
		return asrclient.Transcript{}, fmt.Errorf("asrclient/httpcore: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", contentType)
	if req.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	}

	resp, err := hc.Do(httpReq)
	if err != nil {
		return asrclient.Transcript{}, fmt.Errorf("asrclient/httpcore: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return asrclient.Transcript{}, fmt.Errorf("asrclient/httpcore: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return asrclient.Transcript{}, fmt.Errorf("asrclient/httpcore: read response: %w", err)
	}
	var ar apiResponse
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return asrclient.Transcript{}, fmt.Errorf("asrclient/httpcore: decode response: %w", err)
	}
	return asrclient.Transcript{
		Text:     ar.Text,
		Language: ar.Language,
		Duration: time.Duration(ar.Duration * float64(time.Second)),
	}, nil
}

// HealthyHEAD issues a HEAD to endpoint. Any response (including non-2xx)
// counts as healthy: the goal is to confirm we can talk to the server,
// not to assert it is willing to serve a particular request.
func HealthyHEAD(ctx context.Context, hc *http.Client, endpoint string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		return fmt.Errorf("asrclient/httpcore: build HEAD: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("asrclient/httpcore: HEAD: %w", err)
	}
	_ = resp.Body.Close()
	return nil
}

func buildMultipart(req Request) (io.Reader, string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	if err := mw.WriteField("model", req.Model); err != nil {
		return nil, "", err
	}
	if req.Language != "" {
		if err := mw.WriteField("language", req.Language); err != nil {
			return nil, "", err
		}
	}
	if req.Prompt != "" {
		if err := mw.WriteField("prompt", req.Prompt); err != nil {
			return nil, "", err
		}
	}

	fw, err := mw.CreateFormFile("file", "audio.wav")
	if err != nil {
		return nil, "", err
	}
	if _, err := fw.Write(PCMToWav(req.PCMAudio)); err != nil {
		return nil, "", err
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return &buf, mw.FormDataContentType(), nil
}
