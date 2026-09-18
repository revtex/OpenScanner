package stream

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"
)

// Tuning constants.
const (
	// silenceSeconds is how much filler is generated up front and then
	// cycled. Any value works; a couple of seconds keeps the slice small
	// while still being long enough that cycling is not hot.
	silenceSeconds = 2

	// primeSeconds is the burst of silence written before real-time pacing
	// begins. Browsers will not start playing until they hold a little
	// audio, so without a prime the first call is delayed by however long
	// the player decides to buffer.
	primeSeconds = 2

	// maxQueueSeconds bounds a listener's backlog. A listener whose socket
	// cannot keep up, or who is hit by a burst of simultaneous calls, drops
	// the *oldest* queued audio rather than growing without limit — a
	// scanner backlog is stale by definition and the newest traffic is what
	// the listener actually wants. Without this the queue is unbounded.
	maxQueueSeconds = 120
)

// Call is the subset of a stored call the stream needs.
type Call struct {
	ID          int64
	SystemID    int64 // database system id, matching Client grants
	TalkgroupID int64 // database talkgroup id, matching the saved selection
	AudioPath   string
}

// CallSource resolves a call id to its stream-relevant fields.
type CallSource func(ctx context.Context, callID int64) (Call, error)

// Filter reports whether the given listener should hear a call. It is
// consulted once per listener per call, deliberately: listeners retune
// their talkgroup selection constantly, and a cached selection would keep
// playing talkgroups they just switched off.
type Filter func(ctx context.Context, userID int64, call Call) bool

// Manager owns the connected stream listeners and the shared silence
// filler. The zero value is not usable; call New.
type Manager struct {
	source CallSource
	filter Filter
	// encode is swappable so tests do not need FFmpeg on PATH.
	encode func(ctx context.Context, path string) ([][]byte, error)

	mu        sync.Mutex
	listeners map[*listener]struct{}
	silence   [][]byte
	period    time.Duration
}

// New builds a Manager. Start must be called before Serve.
func New(source CallSource, filter Filter) *Manager {
	return &Manager{
		source:    source,
		filter:    filter,
		encode:    encodeFile,
		listeners: make(map[*listener]struct{}),
	}
}

// Start generates the silence filler and derives the frame pacing from it.
// It shells out to FFmpeg once, so it can fail on a host without a usable
// encoder — in which case the stream endpoint should stay disabled rather
// than serve a broken stream.
func (m *Manager) Start(ctx context.Context) error {
	frames, err := encodeSilence(ctx, silenceSeconds)
	if err != nil {
		return err
	}
	samples, rate := frameDuration(frames[0])
	if samples == 0 || rate == 0 {
		return errors.New("stream: silence frames have an unreadable header")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.silence = frames
	m.period = time.Duration(samples) * time.Second / time.Duration(rate)
	slog.Info("stream: ready",
		"silence_frames", len(frames),
		"frame_ms", m.period.Milliseconds(),
		"sample_rate", rate,
	)
	return nil
}

// Ready reports whether Start succeeded. Nil-safe: a server built without
// a stream manager reports "not ready" rather than panicking.
func (m *Manager) Ready() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.silence) > 0
}

// ListenerCount returns the number of connected stream listeners.
func (m *Manager) ListenerCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.listeners)
}

// listener is one connected stream client.
type listener struct {
	userID int64

	mu         sync.Mutex
	queue      [][]byte
	silenceIdx int
}

// next pops the next frame to send, falling back to cycled silence when no
// call audio is pending.
func (l *listener) next(silence [][]byte) []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.queue) > 0 {
		f := l.queue[0]
		l.queue = l.queue[1:]
		return f
	}
	f := silence[l.silenceIdx%len(silence)]
	l.silenceIdx++
	return f
}

// enqueue appends a call's frames, dropping the oldest audio if the
// listener has fallen too far behind. See maxQueueSeconds.
func (l *listener) enqueue(frames [][]byte, maxFrames int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.queue = append(l.queue, frames...)
	if maxFrames > 0 && len(l.queue) > maxFrames {
		dropped := len(l.queue) - maxFrames
		l.queue = l.queue[dropped:]
		slog.Warn("stream: listener backlog exceeded, dropped oldest audio",
			"user_id", l.userID, "dropped_frames", dropped)
	}
}

func (l *listener) queued() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.queue)
}

// Notify offers a newly ingested call to every connected listener. Callers
// pass only the call id; the manager resolves and transcodes it once and
// shares the resulting frames, so cost does not scale with listener count.
// It returns without doing any work when nobody is listening, which is the
// common case.
func (m *Manager) Notify(ctx context.Context, callID int64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	if len(m.listeners) == 0 || len(m.silence) == 0 {
		m.mu.Unlock()
		return
	}
	targets := make([]*listener, 0, len(m.listeners))
	for l := range m.listeners {
		targets = append(targets, l)
	}
	maxFrames := m.maxFramesLocked()
	m.mu.Unlock()

	call, err := m.source(ctx, callID)
	if err != nil {
		slog.Warn("stream: could not resolve call", "call_id", callID, "error", err)
		return
	}

	wanted := targets[:0:0]
	for _, l := range targets {
		if m.filter == nil || m.filter(ctx, l.userID, call) {
			wanted = append(wanted, l)
		}
	}
	if len(wanted) == 0 {
		return
	}

	frames, err := m.encode(ctx, call.AudioPath)
	if err != nil {
		slog.Warn("stream: could not encode call",
			"call_id", callID, "path", call.AudioPath, "error", err)
		return
	}

	for _, l := range wanted {
		l.enqueue(frames, maxFrames)
	}
	slog.Debug("stream: queued call",
		"call_id", callID, "listeners", len(wanted), "frames", len(frames))
}

func (m *Manager) maxFramesLocked() int {
	if m.period <= 0 {
		return 0
	}
	return int(maxQueueSeconds * time.Second / m.period)
}

// Serve streams to one listener until the context is cancelled or the
// client disconnects. It never returns normally: a live stream ends only
// when one side goes away.
func (m *Manager) Serve(ctx context.Context, userID int64, w io.Writer, flush func()) error {
	m.mu.Lock()
	silence := m.silence
	period := m.period
	m.mu.Unlock()
	if len(silence) == 0 || period <= 0 {
		return errors.New("stream: manager not started")
	}

	l := &listener{userID: userID}
	m.add(l)
	defer m.remove(l)

	if flush == nil {
		flush = func() {}
	}

	// Prime the player's buffer before pacing, otherwise the browser sits
	// waiting for enough data and the first call arrives late.
	prime := int(primeSeconds * time.Second / period)
	for i := 0; i < prime; i++ {
		if _, err := w.Write(l.next(silence)); err != nil {
			return err
		}
	}
	flush()

	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := w.Write(l.next(silence)); err != nil {
				// Client hung up; this is the normal way a stream ends.
				return err
			}
			flush()
		}
	}
}

func (m *Manager) add(l *listener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners[l] = struct{}{}
	slog.Info("stream: listener connected", "user_id", l.userID, "listeners", len(m.listeners))
}

func (m *Manager) remove(l *listener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.listeners, l)
	slog.Info("stream: listener disconnected", "user_id", l.userID, "listeners", len(m.listeners))
}
