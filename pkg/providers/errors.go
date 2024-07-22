package providers

import (
	"fmt"
)

// Provide a list of errors that can be used by any provider
var (
	ErrNoRegionsSpecified = fmt.Errorf("no regions specified")
	ErrNoImagesFound      = fmt.Errorf("no images found")
)
