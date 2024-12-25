// Package aws provides interfaces for AWS STS operations
package aws

// STSClienter defines the interface for STS client operations
type STSClienter interface {
	GetCallerIdentity(ctx Context) (*GetCallerIdentityOutput, error)
}
