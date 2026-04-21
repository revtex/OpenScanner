package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TranscriptionSegment represents one timestamped segment from go-whisper.
type TranscriptionSegment struct {
	Speaker string  `json:"speaker,omitempty"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Text    string  `json:"text"`
}

// transcribeResponse is the raw JSON response from go-whisper.
type transcribeResponse struct {
	Object   string                 `json:"object"`
	Language string                 `json:"language"`
	Segments []TranscriptionSegment `json:"segments"`
}

// TranscriptionResult holds the parsed response from go-whisper.
type TranscriptionResult struct {
	Text     string                 // Flat concatenated text (all segments joined by newline)
	Segments []TranscriptionSegment // Raw segments from go-whisper
	Language string                 // Detected language
}

// TranscriptionJob represents a single transcription task.
type TranscriptionJob struct {
	CallID    int64
	AudioPath string // Absolute path to the converted audio file
}

// TranscriptionJobResult is the outcome of a transcription attempt.
type TranscriptionJobResult struct {
	CallID int64
	Result *TranscriptionResult
	Err    error
}

// TranscriberPool runs go-whisper transcription jobs with bounded concurrency.
type TranscriberPool struct {
	jobs     chan TranscriptionJob
	results  chan TranscriptionJobResult
	client   *http.Client
	baseURL  string
	model    string
	language string
	diarize  bool
}

// maxResponseSize caps go-whisper response bodies at 10 MB.
const maxResponseSize = 10 << 20

// NewTranscriberPool creates a pool of numWorkers goroutines that consume
// transcription jobs and post results. Workers stop when ctx is cancelled.
// baseURL must use http or https scheme.
func NewTranscriberPool(ctx context.Context, numWorkers int, baseURL, model, language string, diarize bool) (*TranscriberPool, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid go-whisper base URL: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("go-whisper base URL must use http or https scheme, got %q", parsed.Scheme)
	}

	if numWorkers < 1 {
		numWorkers = 1
	}

	p := &TranscriberPool{
		jobs:    make(chan TranscriptionJob, numWorkers*4),
		results: make(chan TranscriptionJobResult, numWorkers*4),
		client: &http.Client{
			Timeout: 120 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		baseURL:  strings.TrimRight(baseURL, "/"),
		model:    model,
		language: language,
		diarize:  diarize,
	}

	for i := 0; i < numWorkers; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-p.jobs:
					if !ok {
						return
					}
					p.transcribe(ctx, job)
				}
			}
		}()
	}

	slog.Info("transcriber pool started", "workers", numWorkers, "baseURL", p.baseURL, "model", model, "diarize", diarize)
	return p, nil
}

// Submit enqueues a transcription job. Returns ctx.Err() if the context is
// cancelled before the job can be queued.
func (p *TranscriberPool) Submit(ctx context.Context, job TranscriptionJob) error {
	slog.Debug("transcriber: submitting job", "callID", job.CallID, "audioPath", job.AudioPath)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.jobs <- job:
		return nil
	}
}

// Results returns the channel consumers read transcription outcomes from.
func (p *TranscriberPool) Results() <-chan TranscriptionJobResult {
	return p.results
}

// QueueDepth returns the number of jobs currently buffered.
func (p *TranscriberPool) QueueDepth() int {
	return len(p.jobs)
}

// Model returns the whisper model name configured for this pool.
func (p *TranscriberPool) Model() string {
	return p.model
}

// Ping checks that go-whisper is reachable by hitting its model endpoint.
func (p *TranscriberPool) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/whisper/model", nil)
	if err != nil {
		return fmt.Errorf("transcriber ping: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("transcriber ping: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("transcriber ping: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// transcribe performs a single transcription HTTP call and sends the result.
func (p *TranscriberPool) transcribe(ctx context.Context, job TranscriptionJob) {
	result := TranscriptionJobResult{CallID: job.CallID}

	tr, err := p.doTranscribe(ctx, job)
	if err != nil {
		result.Err = err
		slog.Error("transcription failed", "callID", job.CallID, "error", err)
	} else {
		result.Result = tr
		slog.Debug("transcription completed", "callID", job.CallID, "language", tr.Language, "segments", len(tr.Segments))
	}

	select {
	case p.results <- result:
	case <-ctx.Done():
	}
}

// doTranscribe builds the multipart request, sends it, and parses the response.
func (p *TranscriberPool) doTranscribe(ctx context.Context, job TranscriptionJob) (*TranscriptionResult, error) {
	f, err := os.Open(job.AudioPath)
	if err != nil {
		return nil, fmt.Errorf("open audio file: %w", err)
	}
	defer f.Close()

	// Build multipart body.
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Write form data in a goroutine so the pipe streams to the HTTP request.
	go func() {
		defer pw.Close()
		defer writer.Close()

		filename := filepath.Base(job.AudioPath)
		part, err := writer.CreateFormFile("audio", filename)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("create form file: %w", err))
			return
		}
		if _, err := io.Copy(part, f); err != nil {
			pw.CloseWithError(fmt.Errorf("copy audio data: %w", err))
			return
		}
		if err := writer.WriteField("model", p.model); err != nil {
			pw.CloseWithError(err)
			return
		}
		if p.language != "" {
			if err := writer.WriteField("language", p.language); err != nil {
				pw.CloseWithError(err)
				return
			}
		}
		if p.diarize {
			if err := writer.WriteField("diarize", "true"); err != nil {
				pw.CloseWithError(err)
				return
			}
		}
		if err := writer.WriteField("filename", filename); err != nil {
			pw.CloseWithError(err)
			return
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/whisper/transcribe", pr)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("go-whisper request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Truncate body for logging to avoid huge error messages.
		truncated := string(body)
		if len(truncated) > 512 {
			truncated = truncated[:512] + "..."
		}
		return nil, fmt.Errorf("go-whisper returned status %d: %s", resp.StatusCode, truncated)
	}

	var parsed transcribeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse response JSON: %w", err)
	}

	// Concatenate segment texts into a flat transcript.
	texts := make([]string, 0, len(parsed.Segments))
	for _, seg := range parsed.Segments {
		t := strings.TrimSpace(seg.Text)
		if t != "" {
			texts = append(texts, t)
		}
	}

	return &TranscriptionResult{
		Text:     strings.Join(texts, "\n"),
		Segments: parsed.Segments,
		Language: parsed.Language,
	}, nil
}
