// Package supervisor governs appliance container lifecycle management and Compose v2 orchestration.
//
// Invariants & Boundaries:
// - Privileged Boundary: Only sovereign-daemon mounts /var/run/docker.sock to supervise container spokes.
// - Compose Declarations: Managed services are declared in versioned compose specifications (/data/state/compose/rev-:rev.yaml).
// - Health Monitoring & Watchdog: 180-second verification window with debounced health checks.
// - Atomic Rollback: In the event of persistent service failure, automatically rolls back to previous revision and restores pre-upgrade database snapshot.
package supervisor

import (
	"context"
)

// ServiceState represents the operational status of a managed container spoke.
type ServiceState string

const (
	ServiceStateRunning   ServiceState = "running"
	ServiceStateDegraded  ServiceState = "degraded"
	ServiceStateStopped   ServiceState = "stopped"
	ServiceStateStarting  ServiceState = "starting"
	ServiceStateRollback  ServiceState = "rollback"
)

// Supervisor defines the container orchestration contract for sovereign-daemon.
type Supervisor interface {
	StartService(ctx context.Context, name string) error
	StopService(ctx context.Context, name string) error
	RestartService(ctx context.Context, name string) error
	GetServiceState(ctx context.Context, name string) (ServiceState, error)
}
