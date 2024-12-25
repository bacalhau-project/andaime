// Package types provides type definitions for SSH utilities
package types

// Duration represents a time duration
type Duration int64

// Time represents a point in time
type Time struct {
	UnixTime int64
}

const (
	Nanosecond  Duration = 1
	Microsecond          = 1000 * Nanosecond
	Millisecond          = 1000 * Microsecond
	Second              = 1000 * Millisecond
	Minute              = 60 * Second
	Hour                = 60 * Minute
)
