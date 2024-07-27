package awsprovider

import (
	"sync"

	"github.com/bacalhau-project/andaime/pkg/models"
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
func UpdateAllStatuses(status *models.Status) {
	allStatuses.Store(status.ID, status)
}

// GetAllStatuses retrieves all statuses from the allStatuses map
func GetAllStatuses() map[string]*models.Status {
	result := make(map[string]*models.Status)
	allStatuses.Range(func(key, value interface{}) bool {
		result[key.(string)] = value.(*models.Status)
		return true
	})
	return result
}
