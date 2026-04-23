package model

// HealthStatus represents the health state of a resource.
// Lower values = higher severity = sorted first.
type HealthStatus int

const (
	HealthUnknown     HealthStatus = 0
	HealthFailed      HealthStatus = 1
	HealthProgressing HealthStatus = 2
	HealthSuspended   HealthStatus = 3
	HealthReady       HealthStatus = 4
)

func (h HealthStatus) String() string {
	switch h {
	case HealthFailed:
		return "Failed"
	case HealthProgressing:
		return "Sync"
	case HealthSuspended:
		return "Suspended"
	case HealthReady:
		return "Ready"
	default:
		return "Unknown"
	}
}

func (h HealthStatus) Symbol() string {
	switch h {
	case HealthFailed:
		return "✗"
	case HealthProgressing:
		return "●"
	case HealthSuspended:
		return "⏸"
	case HealthReady:
		return "✓"
	default:
		return "?"
	}
}

func (h HealthStatus) IsFailed() bool {
	return h == HealthFailed
}
