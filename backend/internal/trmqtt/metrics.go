package trmqtt

import "sync/atomic"

// metrics holds package-private counters. Plain atomics; no Prometheus dep.
type metrics struct {
	connectAttempts atomic.Int64
	connected       atomic.Int64
	disconnects     atomic.Int64
	parseErrors     atomic.Int64
	dropOversize    atomic.Int64
	dropMismatch    atomic.Int64
	lagWarnings     atomic.Int64
}

// Snapshot returns a copy of the current counter values for tests/inspection.
func (m *metrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		ConnectAttempts: m.connectAttempts.Load(),
		Connected:       m.connected.Load(),
		Disconnects:     m.disconnects.Load(),
		ParseErrors:     m.parseErrors.Load(),
		DropOversize:    m.dropOversize.Load(),
		DropMismatch:    m.dropMismatch.Load(),
		LagWarnings:     m.lagWarnings.Load(),
	}
}

// MetricsSnapshot is the exported view of the package counters.
type MetricsSnapshot struct {
	ConnectAttempts int64
	Connected       int64
	Disconnects     int64
	ParseErrors     int64
	DropOversize    int64
	DropMismatch    int64
	LagWarnings     int64
}

// pkgMetrics is the shared instance; Manager and tests read through it.
var pkgMetrics metrics

// Metrics returns the current package-level counter snapshot.
func Metrics() MetricsSnapshot { return pkgMetrics.Snapshot() }
