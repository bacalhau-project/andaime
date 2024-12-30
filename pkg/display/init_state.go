package display

import (
	"sync"
)

var (
	displayModelInitialized bool
	displayModelMutex      sync.RWMutex
)

// IsDisplayModelInitialized returns the current initialization state of the display model
func IsDisplayModelInitialized() bool {
	displayModelMutex.RLock()
	defer displayModelMutex.RUnlock()
	return displayModelInitialized
}

// SetDisplayModelInitialized sets the initialization state of the display model
func SetDisplayModelInitialized(initialized bool) {
	displayModelMutex.Lock()
	defer displayModelMutex.Unlock()
	displayModelInitialized = initialized
}

func init() {
	// Initialize to false before any AWS operations or display model initialization
	SetDisplayModelInitialized(false)
}
