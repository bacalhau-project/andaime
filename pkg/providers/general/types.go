package general

import (
	"github.com/spf13/viper"
)

// AzureProvider wraps the Azure deployment functionality
type Providerer interface {
	GetClient() interface{}
	SetClient(client interface{})
	GetConfig() *viper.Viper
	SetConfig(config *viper.Viper)
}
