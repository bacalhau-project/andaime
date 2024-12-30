package aws

import (
	"sync"
)

var (
	verificationPassed bool
	verificationMutex sync.RWMutex
)

// IsVerificationPassed returns whether the AWS verification has passed
func IsVerificationPassed() bool {
	verificationMutex.RLock()
	defer verificationMutex.RUnlock()
	return verificationPassed
}

// SetVerificationPassed sets the state of AWS verification
func SetVerificationPassed(passed bool) {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	verificationPassed = passed
}

func init() {
	// Initialize to false before verification begins
	SetVerificationPassed(false)
}
