package aws

import (
	"sync"
)

var (
	allRegionsMocked bool
	mockStateMutex   sync.RWMutex
)

// IsAllRegionsMocked returns whether all regions have been mocked
func IsAllRegionsMocked() bool {
	mockStateMutex.RLock()
	defer mockStateMutex.RUnlock()
	return allRegionsMocked
}

// SetAllRegionsMocked sets the state of region mocking
func SetAllRegionsMocked(mocked bool) {
	mockStateMutex.Lock()
	defer mockStateMutex.Unlock()
	allRegionsMocked = mocked
}

func init() {
	// Initialize to false before any mocking operations begin
	SetAllRegionsMocked(false)
}
