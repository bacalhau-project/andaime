package utils

import "sync"

type CircularBuffer[T any] struct {
	lines []T
	cur   int
	full  bool
	mu    sync.Mutex
}

func NewCircularBuffer[T any](size int) *CircularBuffer[T] {
	return &CircularBuffer[T]{
		lines: make([]T, size),
	}
}

func (cb *CircularBuffer[T]) Add(line T) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lines[cb.cur] = line
	cb.cur = (cb.cur + 1) % len(cb.lines)
	if !cb.full && cb.cur == 0 {
		cb.full = true
	}
}

func (cb *CircularBuffer[T]) GetLines() []T {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !cb.full {
		return cb.lines[:cb.cur]
	}

	result := make([]T, len(cb.lines))
	copy(result, cb.lines[cb.cur:])
	copy(result[len(cb.lines)-cb.cur:], cb.lines[:cb.cur])
	return result
}
