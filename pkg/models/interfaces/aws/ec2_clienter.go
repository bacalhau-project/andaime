// Package aws provides interfaces for AWS clients
package aws

import (
	"github.com/bacalhau-project/andaime/pkg/models/interfaces/aws/types"
)

// This file is kept for backward compatibility
// All interfaces have been moved to the types package
type EC2Clienter = types.EC2Clienter
type STSClienter = types.STSClienter
