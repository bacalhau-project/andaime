// pkg/providers/factory/factory.go
package factory

import (
	"context"
	"fmt"
	"sync"

	"github.com/bacalhau-project/andaime/pkg/models"
	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
)

// FactoryFunc defines the signature for provider factory functions.
type FactoryFunc func(ctx context.Context) (common_interface.Providerer, error)

// registry holds the mapping from DeploymentType to FactoryFunc.
var (
	registry   = make(map[models.DeploymentType]FactoryFunc)
	registryMu sync.RWMutex
)

// RegisterProvider allows provider packages to register their factory functions.
func RegisterProvider(providerType models.DeploymentType,
	factoryFunc FactoryFunc) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[providerType]; exists {
		panic(fmt.Sprintf("factory: provider type %s is already registered", providerType))
	}
	registry[providerType] = factoryFunc
}

// GetProvider returns the appropriate Providerer based on the DeploymentType.
func GetProvider(
	ctx context.Context,
	providerType models.DeploymentType,
) (common_interface.Providerer, error) {
	registryMu.RLock()
	factoryFunc, exists := registry[providerType]
	registryMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("factory: no provider registered for type %s", providerType)
	}

	return factoryFunc(ctx)
}
