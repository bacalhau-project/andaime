package interfaces

import (
	"context"
	// Import any other necessary packages, but avoid importing from azure package
)

type AzureClienter interface {
	// Add all the methods that AzureClienter should have
	// For example:
	ListResourceGroups(ctx context.Context) ([]ResourceGroup, error)
	// Add other methods...
}

// If needed, define any structs used in the interface methods here
// For example:
type ResourceGroup struct {
	// Define the struct fields
}
