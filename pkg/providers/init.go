// pkg/providers/init.go
package providers

import (
	// Import providers for their init functions
	_ "github.com/bacalhau-project/andaime/pkg/providers/azure"
	_ "github.com/bacalhau-project/andaime/pkg/providers/gcp"
)
