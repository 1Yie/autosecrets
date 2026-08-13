package fleet

import "time"

// NodeState is the derived primary state of a Managed Node (ADR-0015). The
// projection is pure so it can be unit-tested without a database.
type NodeState string

const (
	StateNeverOnline NodeState = "never_online"
	StateHealthy     NodeState = "healthy"
	StateConverging  NodeState = "converging"
	StateFailed      NodeState = "failed"
	StateOffline     NodeState = "offline"
)

// deriveNodeState ranks facts with the documented priority: failed >
// offline > converging > healthy > never_online. A connected node without
// any Assignment is healthy and unassigned, never failed.
func deriveNodeState(lastSeen *time.Time, now time.Time, offlineAfter time.Duration,
	hasFailed, hasAssignment, allConverged bool) (NodeState, bool) {
	if hasFailed {
		return StateFailed, false
	}
	if lastSeen != nil && now.Sub(*lastSeen) > offlineAfter {
		return StateOffline, false
	}
	if hasAssignment && !allConverged {
		return StateConverging, false
	}
	if lastSeen != nil {
		return StateHealthy, !hasAssignment
	}
	return StateNeverOnline, false
}
