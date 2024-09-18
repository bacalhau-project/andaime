// pkg/providers/common/interfaces.go
package common

import (
	"context"
)

// ProviderFactoryer defines an interface to create providers.
type ProviderFactoryer interface {
	CreateProvider(ctx context.Context) (Providerer, error)
}
