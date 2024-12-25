package interfaces

import (
	"github.com/bacalhau-project/andaime/pkg/sshutils/interfaces/types"
)

// TimeProvider defines an interface for time-related operations
type TimeProvider interface {
	// Now returns the current time
	Now() types.Time
	// Sleep pauses the current goroutine for the specified duration
	Sleep(d types.Duration)
}
