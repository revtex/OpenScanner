package audio

import (
	"context"
	"log/slog"
	"sync"
)

// Transcriber is the interface used by call handlers to submit transcription jobs.
// It allows the underlying pool to be swapped at runtime without restarting.
type Transcriber interface {
	Submit(ctx context.Context, job TranscriptionJob) error
}

// TranscriberManager wraps a TranscriberPool with thread-safe hot-reload.
// It implements Transcriber so callers can submit jobs without caring about
// the underlying pool lifecycle.
type TranscriberManager struct {
	mu      sync.RWMutex
	pool    *TranscriberPool
	cancel  context.CancelFunc // cancels the current pool's workers
	results chan TranscriptionJobResult
	appCtx  context.Context // root context (server lifetime)
}

// NewTranscriberManager creates a manager. If pool is nil, transcription starts disabled.
// results is a shared channel that all pools write to; the consumer goroutine reads from it.
func NewTranscriberManager(appCtx context.Context, pool *TranscriberPool, cancel context.CancelFunc) *TranscriberManager {
	m := &TranscriberManager{
		pool:    pool,
		cancel:  cancel,
		results: make(chan TranscriptionJobResult, 16),
		appCtx:  appCtx,
	}
	// Pump results from the initial pool (if any) into the shared channel.
	if pool != nil {
		go m.pumpResults(pool)
	}
	return m
}

// Submit enqueues a transcription job on the current pool.
// Returns nil (no-op) if transcription is disabled.
func (m *TranscriberManager) Submit(ctx context.Context, job TranscriptionJob) error {
	m.mu.RLock()
	p := m.pool
	m.mu.RUnlock()
	if p == nil {
		return nil
	}
	return p.Submit(ctx, job)
}

// Results returns the shared results channel that the consumer goroutine reads from.
func (m *TranscriberManager) Results() <-chan TranscriptionJobResult {
	return m.results
}

// Model returns the current pool's model name, or empty string if disabled.
func (m *TranscriberManager) Model() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.pool == nil {
		return ""
	}
	return m.pool.Model()
}

// Enabled returns whether transcription is currently active.
func (m *TranscriberManager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pool != nil
}

// BaseURL returns the current pool's base URL, or empty string if disabled.
func (m *TranscriberManager) BaseURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.pool == nil {
		return ""
	}
	return m.pool.baseURL
}

// QueueDepth returns the number of jobs currently buffered, or 0 if disabled.
func (m *TranscriberManager) QueueDepth() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.pool == nil {
		return 0
	}
	return m.pool.QueueDepth()
}

// Reload stops the current pool (if any) and starts a new one with the given config.
// If enabled is false, the pool is stopped and transcription is disabled.
func (m *TranscriberManager) Reload(enabled bool, baseURL, model, language string, diarize bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop existing pool.
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.pool = nil

	if !enabled {
		slog.Info("transcription disabled")
		return true
	}

	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}
	if model == "" {
		model = "ggml-base"
	}

	poolCtx, poolCancel := context.WithCancel(m.appCtx)

	tp, err := NewTranscriberPool(poolCtx, 2, baseURL, model, language, diarize)
	if err != nil {
		poolCancel()
		slog.Warn("transcription pool creation failed", "error", err)
		return false
	}
	if err := tp.Ping(poolCtx); err != nil {
		poolCancel()
		slog.Warn("go-whisper unreachable", "url", baseURL, "error", err)
		return false
	}

	m.pool = tp
	m.cancel = poolCancel
	slog.Info("transcription enabled", "url", baseURL, "model", model, "diarize", diarize)

	// Pump results from the new pool into the shared channel.
	go m.pumpResults(tp)

	return true
}

// pumpResults forwards results from a specific pool into the shared results channel.
// It stops when the pool's results channel is closed (pool context cancelled).
func (m *TranscriberManager) pumpResults(pool *TranscriberPool) {
	for res := range pool.Results() {
		select {
		case m.results <- res:
		case <-m.appCtx.Done():
			return
		}
	}
}
