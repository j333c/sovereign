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

// Supervisor defines the container orchestration contract for sovereign-daemon.
type Supervisor interface {
	ListServices(ctx context.Context) ([]ServiceStatus, error)
	GetService(ctx context.Context, id string) (*ServiceStatus, error)
	ActivateService(ctx context.Context, id string) (*ServiceStatus, error)
	DeactivateService(ctx context.Context, id string) (*ServiceStatus, error)
	RestartService(ctx context.Context, id string) (*ServiceStatus, error)
}
