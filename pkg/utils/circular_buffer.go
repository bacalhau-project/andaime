package utils

import "sync"

type CircularBuffer struct {
	lines []string
	cur   int
	full  bool
	mu    sync.Mutex
}

func NewCircularBuffer(size int) *CircularBuffer {
	return &CircularBuffer{
		lines: make([]string, size),
	}
}

func (cb *CircularBuffer) Add(line string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lines[cb.cur] = line
	cb.cur = (cb.cur + 1) % len(cb.lines)
	if !cb.full && cb.cur == 0 {
		cb.full = true
	}
}

func (cb *CircularBuffer) GetLines() []string {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !cb.full {
		return cb.lines[:cb.cur]
	}

	result := make([]string, len(cb.lines))
	copy(result, cb.lines[cb.cur:])
	copy(result[len(cb.lines)-cb.cur:], cb.lines[:cb.cur])
	return result
}
