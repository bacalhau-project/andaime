// Package aws provides interfaces for AWS clients
package aws

import (
	"github.com/bacalhau-project/andaime/pkg/models/interfaces/aws/types"
)

// EC2Clienter defines the interface for EC2 client operations
type EC2Clienter interface {
	types.EC2Operations
	EC2ContextOperations
}

// STSClienter defines the interface for STS client operations
type STSClienter interface {
	types.STSOperations
	types.STSClient
}
