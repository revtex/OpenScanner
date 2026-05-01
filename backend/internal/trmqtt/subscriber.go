package trmqtt

import (
	"context"
	"log/slog"
	"strings"
)

// subscriber owns topic-to-handler routing for a single Client. It updates
// the per-instance Snapshot and emits Events on `out`.
//
// Routing is purely string-prefix based: each Client only ever subscribes to
// the topics owned by its own row, so the subscriber doesn't need a regex
// engine.
type subscriber struct {
	instanceID     int64
	label          string
	expectedPlugin string // matches tr_instances.instance_id; mismatched frames are dropped

	baseTopic    string
	unitTopic    string // optional ("" when disabled)
	messageTopic string // optional

	snapshot *Snapshot
	out      eventEmitter
	enrich   Enricher
}

// eventEmitter is the narrow interface a subscriber needs to fan events out.
// Manager's emit() satisfies it. Tests wire a slice-based recorder.
type eventEmitter interface {
	emit(Event)
}

// topicSpecs returns the (filter, qos) pairs this subscriber wants the broker
// to deliver. Audio is never present.
func (s *subscriber) topicSpecs(qos byte) []topicSpec {
	specs := []topicSpec{
		{filter: s.baseTopic + "/rates", qos: qos},
		{filter: s.baseTopic + "/recorders", qos: qos},
		{filter: s.baseTopic + "/calls_active", qos: qos},
		{filter: s.baseTopic + "/call_start", qos: qos},
		{filter: s.baseTopic + "/call_end", qos: qos},
		{filter: s.baseTopic + "/system", qos: qos},
		{filter: s.baseTopic + "/systems", qos: qos},
		{filter: s.baseTopic + "/config", qos: qos},
		{filter: s.baseTopic + "/trunk_recorder/status", qos: qos},
	}
	if s.unitTopic != "" {
		for _, k := range []string{"on", "off", "join", "location", "call", "end", "data", "ackresp", "ans_req"} {
			specs = append(specs, topicSpec{filter: s.unitTopic + "/+/" + k, qos: qos})
		}
	}
	if s.messageTopic != "" {
		specs = append(specs, topicSpec{filter: s.messageTopic + "/+/messages", qos: qos})
	}
	return specs
}

type topicSpec struct {
	filter string
	qos    byte
}

// handle dispatches one inbound message. payload must be the raw broker bytes;
// caller is responsible for not invoking handle on payloads larger than
// MaxPayloadBytes (the Client wrapper enforces the cap and counts oversize).
func (s *subscriber) handle(ctx context.Context, topic string, payload []byte) {
	switch {
	case topic == s.baseTopic+"/rates":
		var f RatesFrame
		if !s.decodeAndCheck(topic, payload, &f, &f.frameCommon) {
			return
		}
		s.snapshot.setRates(f)
		s.out.emit(Event{Type: EventRates, InstanceID: s.instanceID, Label: s.label, Payload: f})

	case topic == s.baseTopic+"/recorders":
		var f RecordersFrame
		if !s.decodeAndCheck(topic, payload, &f, &f.frameCommon) {
			return
		}
		s.snapshot.setRecorders(f)
		s.out.emit(Event{Type: EventRecorders, InstanceID: s.instanceID, Label: s.label, Payload: f})

	case topic == s.baseTopic+"/calls_active":
		var f CallsActiveFrame
		if !s.decodeAndCheck(topic, payload, &f, &f.frameCommon) {
			return
		}
		s.snapshot.setCallsActive(f)
		s.out.emit(Event{Type: EventCallsActive, InstanceID: s.instanceID, Label: s.label, Payload: f})

	case topic == s.baseTopic+"/call_start":
		var f CallStartFrame
		if !s.decodeAndCheck(topic, payload, &f, &f.frameCommon) {
			return
		}
		s.out.emit(Event{Type: EventCallStart, InstanceID: s.instanceID, Label: s.label, Payload: f})

	case topic == s.baseTopic+"/call_end":
		var f CallEndFrame
		if !s.decodeAndCheck(topic, payload, &f, &f.frameCommon) {
			return
		}
		s.out.emit(Event{Type: EventCallEnd, InstanceID: s.instanceID, Label: s.label, Payload: f})

	case topic == s.baseTopic+"/system":
		var f SystemFrame
		if !s.decodeAndCheck(topic, payload, &f, &f.frameCommon) {
			return
		}
		s.snapshot.mergeSystem(f)
		s.out.emit(Event{Type: EventSystem, InstanceID: s.instanceID, Label: s.label, Payload: f})

	case topic == s.baseTopic+"/systems":
		var f SystemsFrame
		if !s.decodeAndCheck(topic, payload, &f, &f.frameCommon) {
			return
		}
		s.snapshot.setSystems(f)
		s.out.emit(Event{Type: EventSystems, InstanceID: s.instanceID, Label: s.label, Payload: f})

	case topic == s.baseTopic+"/config":
		var f ConfigFrame
		if !s.decodeAndCheck(topic, payload, &f, &f.frameCommon) {
			return
		}
		s.snapshot.setConfig(f)
		s.out.emit(Event{Type: EventConfig, InstanceID: s.instanceID, Label: s.label, Payload: f})

	case topic == s.baseTopic+"/trunk_recorder/status":
		var f PluginStatusFrame
		if !s.decodeAndCheck(topic, payload, &f, &f.frameCommon) {
			return
		}
		s.snapshot.setPluginStatus(f)
		s.out.emit(Event{Type: EventPluginStatus, InstanceID: s.instanceID, Label: s.label, Payload: f})

	default:
		// Unit and message topics use shortname wildcards.
		if s.unitTopic != "" && strings.HasPrefix(topic, s.unitTopic+"/") {
			kind := unitKindFromTopic(topic)
			if kind == "" {
				return
			}
			var f UnitFrame
			if !s.decodeAndCheck(topic, payload, &f, &f.frameCommon) {
				return
			}
			f.Kind = kind
			s.snapshot.appendUnit(topic, f)
			ev := unitKindToEvent(kind)
			if ev == "" {
				return
			}
			s.out.emit(Event{Type: ev, InstanceID: s.instanceID, Label: s.label, Payload: f})
			return
		}
		if s.messageTopic != "" && strings.HasPrefix(topic, s.messageTopic+"/") && strings.HasSuffix(topic, "/messages") {
			var f MessageFrame
			if !s.decodeAndCheck(topic, payload, &f, &f.frameCommon) {
				return
			}
			s.snapshot.appendMessage(topic, f)
			s.out.emit(Event{Type: EventMessage, InstanceID: s.instanceID, Label: s.label, Payload: f})
			return
		}
		slog.DebugContext(ctx, "trmqtt: unrouted topic",
			"instance_id", s.instanceID, "topic", topic)
	}
}

// decodeAndCheck wraps decode + plugin-instance-ID guard. Returns true when
// the frame is valid and matches the expected instance.
func (s *subscriber) decodeAndCheck(topic string, payload []byte, v any, common *frameCommon) bool {
	if err := decode(payload, v); err != nil {
		pkgMetrics.parseErrors.Add(1)
		slog.Warn("trmqtt: decode failed",
			"instance_id", s.instanceID, "topic", topic, "size", len(payload), "error", err)
		return false
	}
	// Defense against misconfiguration: drop frames whose instance_id doesn't
	// match the configured row. Empty instance_id (e.g. retained legacy
	// frames) is allowed through.
	if s.expectedPlugin != "" && common.InstanceID != "" && common.InstanceID != s.expectedPlugin {
		pkgMetrics.dropMismatch.Add(1)
		slog.Warn("trmqtt: dropped frame with mismatched instance_id",
			"instance_id", s.instanceID, "topic", topic,
			"expected", s.expectedPlugin, "got", common.InstanceID)
		return false
	}
	return true
}

// unitKindFromTopic extracts the trailing event kind from a unit topic of the
// shape `<unit_topic>/<shortname>/<kind>`. Returns "" if the suffix is not a
// known unit kind.
func unitKindFromTopic(topic string) UnitEventKind {
	idx := strings.LastIndex(topic, "/")
	if idx < 0 {
		return ""
	}
	switch UnitEventKind(topic[idx+1:]) {
	case UnitEventOn:
		return UnitEventOn
	case UnitEventOff:
		return UnitEventOff
	case UnitEventJoin:
		return UnitEventJoin
	case UnitEventLocation:
		return UnitEventLocation
	case UnitEventCall:
		return UnitEventCall
	case UnitEventEnd:
		return UnitEventEnd
	case UnitEventData:
		return UnitEventData
	case UnitEventAckResp:
		return UnitEventAckResp
	case UnitEventAnsReq:
		return UnitEventAnsReq
	}
	return ""
}
