package gcp

import (
	"github.com/bacalhau-project/andaime/pkg/providers/common"
)

// GenerateTags creates a map of tags for GCP resources
func GenerateTags(projectID, uniqueID string) map[string]string {
	return common.GenerateTags(projectID, uniqueID)
}

// Add any GCP-specific utility functions here if needed
