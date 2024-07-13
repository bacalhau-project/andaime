package providers

import (
	"context"
)

// CloudProvider defines the interface for cloud-specific operations
type CloudProvider interface {
	// GetLatestUbuntuImage retrieves the latest Ubuntu image ID for a given region
	GetLatestUbuntuImage(ctx context.Context, region string) (string, error)

	// Add other common cloud operations here as needed
}

// ImageInfo represents details about a cloud image
type ImageInfo struct {
	ID           string
	Name         string
	CreationDate string
}

// ErrNoImagesFound is returned when no images are found matching the criteria
var ErrNoImagesFound = errors.New("no images found")