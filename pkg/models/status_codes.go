package models

// StatusCode represents the possible status codes
type StatusCode string

// Constants for each status code
const (
	StatusSucceeded StatusCode = "✅"
	StatusFailed    StatusCode = "❌"
	StatusCreating  StatusCode = "🕕"
	StatusUpdating  StatusCode = "🆙"
	StatusDeleting  StatusCode = "␡"
	StatusUnknown   StatusCode = "❓"
)
