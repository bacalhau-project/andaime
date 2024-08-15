package models

import "strings"

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

// StatusString represents the full status strings
type StatusString string

// Constants for each full status string
const (
	StatusStringSucceeded StatusString = "Succeeded"
	StatusStringFailed    StatusString = "Failed"
	StatusStringCreating  StatusString = "Creating"
	StatusStringUpdating  StatusString = "Updating"
	StatusStringDeleting  StatusString = "Deleting"
)

func (s *StatusString) GetLowered() StatusString {
	return StatusString(strings.ToLower(string(*s)))
}

// GetStatusCode converts a StatusString to a StatusCode
func GetStatusCode(status StatusString) StatusCode {
	switch status.GetLowered() {
	case StatusStringSucceeded:
		return StatusSucceeded
	case StatusStringFailed:
		return StatusFailed
	case StatusStringCreating:
		return StatusCreating
	case StatusStringUpdating:
		return StatusUpdating
	case StatusStringDeleting:
		return StatusDeleting
	default:
		return StatusUnknown
	}
}

// GetStatusString converts a StatusCode to a StatusString
func GetStatusString(code StatusCode) StatusString {
	switch code {
	case StatusSucceeded:
		return StatusStringSucceeded
	case StatusFailed:
		return StatusStringFailed
	case StatusCreating:
		return StatusStringCreating
	case StatusUpdating:
		return StatusStringUpdating
	case StatusDeleting:
		return StatusStringDeleting
	default:
		return ""
	}
}
