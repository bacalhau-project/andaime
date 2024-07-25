package aws_provider

import (
	"sync"
)

// Status represents the status of an instance
type ResourceInfo struct {
	ID         string
	Type       string
	Region     string
	Zone       string
	Status     string
	InstanceID string
	PublicIP   string
	PrivateIP  string
}

type Status struct {
	ID string
	ResourceInfo
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
