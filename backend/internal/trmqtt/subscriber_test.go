package trmqtt

import (
	"sync"
	"testing"
)

// recordingEmitter is a thread-safe slice-backed eventEmitter for tests.
type recordingEmitter struct {
	mu     sync.Mutex
	events []Event
}

func (r *recordingEmitter) emit(ev Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
}

func (r *recordingEmitter) snapshot() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Event, len(r.events))
	copy(out, r.events)
	return out
}

func newSubscriber(t *testing.T, emitter eventEmitter) (*subscriber, *Snapshot) {
	t.Helper()
	snap := NewSnapshot(7, "test")
	return &subscriber{
		instanceID:     7,
		label:          "test",
		expectedPlugin: "site-a",
		baseTopic:      "tr",
		unitTopic:      "tr/units",
		messageTopic:   "tr/messages",
		snapshot:       snap,
		out:            emitter,
		enrich:         noopEnricher{},
	}, snap
}

func TestSubscriber_RoutesRatesAndUpdatesSnapshot(t *testing.T) {
	emitter := &recordingEmitter{}
	sub, snap := newSubscriber(t, emitter)

	payload := []byte(`{"type":"rates","instance_id":"site-a","rates":[{"sys_num":0,"decoderate":35.5}]}`)
	sub.handle(t.Context(), "tr/rates", payload)

	evs := emitter.snapshot()
	if len(evs) != 1 || evs[0].Type != EventRates {
		t.Fatalf("events: %+v", evs)
	}
	if got := snap.Get().Rates.InstanceID; got != "site-a" {
		t.Fatalf("snapshot rates instance_id: %q", got)
	}
}

func TestSubscriber_RoutesUnitCall(t *testing.T) {
	emitter := &recordingEmitter{}
	sub, snap := newSubscriber(t, emitter)

	payload := []byte(`{"type":"call","instance_id":"site-a","shortname":"hc","unit_id":42,"unit_alpha":"X1","talkgroup":500}`)
	sub.handle(t.Context(), "tr/units/hc/call", payload)

	evs := emitter.snapshot()
	if len(evs) != 1 || evs[0].Type != EventUnitCall {
		t.Fatalf("events: %+v", evs)
	}
	view := snap.Get()
	if len(view.UnitEvents) != 1 {
		t.Fatalf("ring stored %d entries", len(view.UnitEvents))
	}
	if view.UnitEvents[0].Frame.Kind != UnitEventCall {
		t.Fatalf("kind: %v", view.UnitEvents[0].Frame.Kind)
	}
}

func TestSubscriber_DropsMismatchedInstanceID(t *testing.T) {
	emitter := &recordingEmitter{}
	sub, _ := newSubscriber(t, emitter)

	before := pkgMetrics.dropMismatch.Load()
	payload := []byte(`{"type":"rates","instance_id":"OTHER","rates":[]}`)
	sub.handle(t.Context(), "tr/rates", payload)

	if got := emitter.snapshot(); len(got) != 0 {
		t.Fatalf("expected zero events, got %+v", got)
	}
	if pkgMetrics.dropMismatch.Load() <= before {
		t.Fatalf("dropMismatch counter not incremented")
	}
}

func TestSubscriber_AllowsEmptyInstanceID(t *testing.T) {
	emitter := &recordingEmitter{}
	sub, _ := newSubscriber(t, emitter)

	payload := []byte(`{"type":"rates","rates":[]}`)
	sub.handle(t.Context(), "tr/rates", payload)

	if got := emitter.snapshot(); len(got) != 1 {
		t.Fatalf("expected 1 event, got %+v", got)
	}
}

func TestSubscriber_BadJSONIncrementsParseErrors(t *testing.T) {
	emitter := &recordingEmitter{}
	sub, _ := newSubscriber(t, emitter)

	before := pkgMetrics.parseErrors.Load()
	sub.handle(t.Context(), "tr/rates", []byte(`{not json`))

	if got := emitter.snapshot(); len(got) != 0 {
		t.Fatalf("expected zero events, got %+v", got)
	}
	if pkgMetrics.parseErrors.Load() <= before {
		t.Fatalf("parseErrors counter not incremented")
	}
}

func TestTopicSpecs_NoAudio(t *testing.T) {
	emitter := &recordingEmitter{}
	sub, _ := newSubscriber(t, emitter)

	for _, spec := range sub.topicSpecs(0) {
		if spec.filter == "tr/audio" || spec.filter == sub.baseTopic+"/audio" {
			t.Fatalf("audio topic must never be subscribed; got %q", spec.filter)
		}
	}
}
