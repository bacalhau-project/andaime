package aws

import (
	"sync"
)

// Status represents the status of an instance
type Status struct {
	ID string
	// Add other fields as needed
}

var (
	allStatuses      = make(map[string]*Status)
	allStatusesMutex sync.Mutex
)

// UpdateAllStatuses updates the global allStatuses map in a thread-safe manner
func UpdateAllStatuses(status *Status) {
	allStatusesMutex.Lock()
	defer allStatusesMutex.Unlock()
	allStatuses[status.ID] = status
}
