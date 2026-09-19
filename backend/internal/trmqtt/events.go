package trmqtt

// EventType enumerates events fanned out from Manager to subscribers
// (admin WS hub in step 4).
type EventType string

const (
	EventInstanceConnected    EventType = "tr.instance.connected"
	EventInstanceDisconnected EventType = "tr.instance.disconnected"
	EventSnapshot             EventType = "tr.snapshot"
	EventRates                EventType = "tr.rates"
	EventRecorders            EventType = "tr.recorders"
	EventCallsActive          EventType = "tr.callsActive"
	EventCallStart            EventType = "tr.callStart"
	EventCallEnd              EventType = "tr.callEnd"
	EventSystem               EventType = "tr.system"
	EventSystems              EventType = "tr.systems"
	EventConfig               EventType = "tr.config"
	EventPluginStatus         EventType = "tr.pluginStatus"
	EventUnitOn               EventType = "tr.unit.on"
	EventUnitOff              EventType = "tr.unit.off"
	EventUnitJoin             EventType = "tr.unit.join"
	EventUnitLocation         EventType = "tr.unit.location"
	EventUnitCall             EventType = "tr.unit.call"
	EventUnitEnd              EventType = "tr.unit.end"
	EventUnitData             EventType = "tr.unit.data"
	EventUnitAckResp          EventType = "tr.unit.ackresp"
	EventUnitAnsReq           EventType = "tr.unit.ans_req"
	EventMessage              EventType = "tr.message"
	EventWarnLag              EventType = "tr.warn.lag"
)

// Event is the unit of fan-out from Manager to consumers. InstanceID is the
// tr_instances row ID; Label is the human-friendly label from the same row.
// Payload's concrete type depends on Type and is documented per case.
type Event struct {
	Type       EventType
	InstanceID int64
	Label      string
	Payload    any
	// Err is populated on EventInstanceDisconnected.
	Err error
}

// unitKindToEvent maps a UnitEventKind to its Event type.
func unitKindToEvent(k UnitEventKind) EventType {
	switch k {
	case UnitEventOn:
		return EventUnitOn
	case UnitEventOff:
		return EventUnitOff
	case UnitEventJoin:
		return EventUnitJoin
	case UnitEventLocation:
		return EventUnitLocation
	case UnitEventCall:
		return EventUnitCall
	case UnitEventEnd:
		return EventUnitEnd
	case UnitEventData:
		return EventUnitData
	case UnitEventAckResp:
		return EventUnitAckResp
	case UnitEventAnsReq:
		return EventUnitAnsReq
	}
	return EventType("")
}
