package provision

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bacalhau-project/andaime/pkg/models"
)

// readBacalhauSettingsFromFile reads and parses Bacalhau settings from a JSON file
func readBacalhauSettingsFromFile(path string) ([]models.BacalhauSettings, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	var settings []models.BacalhauSettings
	if err := json.Unmarshal(content, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings JSON: %w", err)
	}

	return settings, nil
}
