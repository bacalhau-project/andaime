package gcpip

import (
	"fmt"
	"time"
)

// QuotaDetails contains information about IP address quota
type QuotaDetails struct {
	Used      int
	Limit     int
	Region    string
	Timestamp time.Time
}

// IPAllocationError wraps errors from IP allocation attempts
type IPAllocationError struct {
	Operation   string
	Region      string
	Attempt     int
	StatusCode  int
	QuotaInfo   *QuotaDetails
	Err         error
	Timestamp   time.Time
}

func (e *IPAllocationError) Error() string {
	base := fmt.Sprintf("IP allocation failed in %s (attempt %d): %s", e.Region, e.Attempt, e.Operation)
	if e.QuotaInfo != nil {
		base += fmt.Sprintf(" [Quota: %d/%d]", e.QuotaInfo.Used, e.QuotaInfo.Limit)
	}
	if e.Err != nil {
		base += fmt.Sprintf(" - %v", e.Err)
	}
	return base
}

// NewIPAllocationError creates a new IP allocation error
func NewIPAllocationError(op, region string, attempt int, err error) *IPAllocationError {
	return &IPAllocationError{
		Operation: op,
		Region:    region,
		Attempt:   attempt,
		Err:       err,
		Timestamp: time.Now(),
	}
}
