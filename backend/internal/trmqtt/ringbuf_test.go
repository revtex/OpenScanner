package trmqtt

import (
	"testing"
)

func TestRing_PushSnapshotOrder(t *testing.T) {
	r := NewRing[int](3)
	r.Push(1)
	r.Push(2)
	got := r.Snapshot()
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("snapshot: %v", got)
	}
}

func TestRing_OverflowDropsOldest(t *testing.T) {
	r := NewRing[int](3)
	for i := 1; i <= 5; i++ {
		r.Push(i)
	}
	got := r.Snapshot()
	if len(got) != 3 || got[0] != 3 || got[1] != 4 || got[2] != 5 {
		t.Fatalf("snapshot: %v", got)
	}
	if r.Len() != 3 {
		t.Fatalf("len: %d", r.Len())
	}
}

func TestRing_Empty(t *testing.T) {
	r := NewRing[string](2)
	if got := r.Snapshot(); len(got) != 0 {
		t.Fatalf("snapshot non-empty: %v", got)
	}
}

func TestRing_ZeroCapClamped(t *testing.T) {
	r := NewRing[int](0)
	r.Push(1)
	r.Push(2)
	got := r.Snapshot()
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("snapshot: %v", got)
	}
}
