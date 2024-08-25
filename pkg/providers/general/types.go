package general

import (
	"context"

	"github.com/spf13/viper"
)

// AzureProvider wraps the Azure deployment functionality
type Providerer interface {
	GetClient() interface{}
	SetClient(client interface{})
	GetConfig() *viper.Viper
	SetConfig(config *viper.Viper)

	DeployBacalhauOrchestrator(ctx context.Context) error
	DeployBacalhauWorker(ctx context.Context,
		machineName string,
		workerErrChan chan error) error
}
