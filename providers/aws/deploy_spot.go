package aws

import (
	"sync"
)

// Status represents the status of an instance
type Status struct {
	ID string
	// Add other fields as needed
}

// allStatuses is a thread-safe map to store instance statuses
var allStatuses sync.Map

// UpdateAllStatuses updates the global allStatuses map in a thread-safe manner
func UpdateAllStatuses(status *Status) {
	allStatuses.Store(status.ID, status)
}

// GetAllStatuses retrieves all statuses from the allStatuses map
func GetAllStatuses() map[string]*Status {
	result := make(map[string]*Status)
	allStatuses.Range(func(key, value interface{}) bool {
		result[key.(string)] = value.(*Status)
		return true
	})
	return result
}
