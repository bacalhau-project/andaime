package sshutils

import (
	"time"
)

// DefaultTimeProvider implements the TimeProvider interface using the standard time package
type DefaultTimeProvider struct{}

func NewDefaultTimeProvider() TimeProvider {
	return &DefaultTimeProvider{}
}

func (p *DefaultTimeProvider) Now() Time {
	return Time{UnixTime: time.Now().Unix()}
}

func (p *DefaultTimeProvider) Sleep(d time.Duration) {
	time.Sleep(d)
}

type TimeProvider interface {
	Now() Time
	Sleep(d time.Duration)
}

type Time struct {
	UnixTime int64
}
