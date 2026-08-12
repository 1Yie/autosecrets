package app

import (
	"testing"
	"time"
)

func TestDeriveNodeState(t *testing.T) {
	now := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-10 * time.Second)
	stale := now.Add(-90 * time.Second)
	const offlineAfter = 75 * time.Second

	tests := []struct {
		name           string
		lastSeen       *time.Time
		hasFailed      bool
		hasAssignment  bool
		allConverged   bool
		wantState      NodeState
		wantUnassigned bool
	}{
		{"never seen", nil, false, false, false, StateNeverOnline, false},
		{"healthy assigned", &recent, false, true, true, StateHealthy, false},
		{"healthy unassigned", &recent, false, false, false, StateHealthy, true},
		{"converging", &recent, false, true, false, StateConverging, false},
		{"offline", &stale, false, true, true, StateOffline, false},
		{"failed outranks offline", &stale, true, true, false, StateFailed, false},
		{"failed with fresh heartbeat", &recent, true, true, true, StateFailed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, unassigned := deriveNodeState(tt.lastSeen, now, offlineAfter,
				tt.hasFailed, tt.hasAssignment, tt.allConverged)
			if state != tt.wantState || unassigned != tt.wantUnassigned {
				t.Fatalf("deriveNodeState = %s unassigned=%v, want %s unassigned=%v",
					state, unassigned, tt.wantState, tt.wantUnassigned)
			}
		})
	}
}
