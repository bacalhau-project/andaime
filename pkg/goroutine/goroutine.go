package goroutine

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var (
	goroutineCounter uint64
	goroutineMap     sync.Map
)

// RegisterGoroutine registers a new goroutine with a given name and returns a unique ID
func RegisterGoroutine(name string) uint64 {
	id := atomic.AddUint64(&goroutineCounter, 1)
	goroutineMap.Store(id, name)
	return id
}

// DeregisterGoroutine removes a goroutine from the map using its ID
func DeregisterGoroutine(id uint64) {
	goroutineMap.Delete(id)
}

// GetActiveGoroutines returns a map of active goroutine IDs and their names
func GetActiveGoroutines() map[uint64]string {
	result := make(map[uint64]string)
	goroutineMap.Range(func(key, value interface{}) bool {
		id, ok := key.(uint64)
		if !ok {
			fmt.Printf("Warning: invalid goroutine ID type: %T\n", key)
			return true
		}
		name, ok := value.(string)
		if !ok {
			fmt.Printf("Warning: invalid goroutine name type: %T\n", value)
			return true
		}
		result[id] = name
		return true
	})
	return result
}
