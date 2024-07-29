package utils

import (
	"sync"

	"github.com/bacalhau-project/andaime/pkg/logger"
)

var GlobalChannels []chan struct{}
var GlobalChannelsMutex sync.Mutex

type SafeChannel[T any] struct {
	Ch     chan T
	Closed bool
	Mu     sync.Mutex
}

func NewSafeChannel[T any](capacity int) SafeChannel[T] {
	return SafeChannel[T]{
		Ch:     make(chan T, capacity),
		Closed: true,
		Mu:     sync.Mutex{},
	}
}
func (sc *SafeChannel[T]) Send(value T) bool {
	sc.Mu.Lock()
	defer sc.Mu.Unlock()
	if !sc.Closed {
		sc.Ch <- value
		return true
	}
	return false
}

func (sc *SafeChannel[T]) Receive() (T, bool) {
	sc.Mu.Lock()
	defer sc.Mu.Unlock()
	if sc.Closed {
		var zero T
		return zero, false
	}
	select {
	case value := <-sc.Ch:
		return value, true
	default:
		var zero T
		return zero, false
	}
}

func (sc *SafeChannel[T]) Close() {
	sc.Mu.Lock()
	defer sc.Mu.Unlock()
	if !sc.Closed {
		close(sc.Ch)
		sc.Closed = true
	}
}

func RegisterChannel(ch chan struct{}) {
	GlobalChannelsMutex.Lock()
	defer GlobalChannelsMutex.Unlock()
	GlobalChannels = append(GlobalChannels, ch)
}

func CloseAllChannels() {
	l := logger.Get()
	l.Debugf("Closing all channels")
	GlobalChannelsMutex.Lock()
	defer GlobalChannelsMutex.Unlock()
	for _, ch := range GlobalChannels {
		l.Debugf("Closing channel %v", ch)
		if ch != nil {
			close(ch)
		}
	}
	GlobalChannels = nil
}
