package provision

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/models"
)

// SettingsParser handles the parsing of Bacalhau settings files
type SettingsParser struct{}

// NewSettingsParser creates a new settings parser
func NewSettingsParser() *SettingsParser {
	return &SettingsParser{}
}

// ParseFile reads and parses a settings file, returning the settings or an error
func (p *SettingsParser) ParseFile(filePath string) ([]models.BacalhauSettings, error) {
	if filePath == "" {
		return nil, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open settings file: %w", err)
	}
	defer file.Close()

	return p.parse(file)
}

// parse reads from a reader and returns the parsed settings
func (p *SettingsParser) parse(file *os.File) ([]models.BacalhauSettings, error) {
	var settings []models.BacalhauSettings
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		setting, err := p.parseLine(line, lineNum)
		if err != nil {
			return nil, err
		}

		settings = append(settings, setting)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading settings file: %w", err)
	}

	return settings, nil
}

// parseLine parses a single line of the settings file
func (p *SettingsParser) parseLine(line string, lineNum int) (models.BacalhauSettings, error) {
	// Split on first colon
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return models.BacalhauSettings{}, fmt.Errorf(
			"invalid format at line %d: missing key-value separator ':', line content: %s",
			lineNum,
			line,
		)
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Validate key
	if key == "" {
		return models.BacalhauSettings{}, fmt.Errorf("empty key at line %d: %s", lineNum, line)
	}

	// Validate value and remove quotes if present
	if value == "" {
		return models.BacalhauSettings{}, fmt.Errorf("empty value at line %d: %s", lineNum, line)
	}
	value = strings.Trim(value, `"`)
	if value == "" {
		return models.BacalhauSettings{}, fmt.Errorf(
			"value contains only quotes at line %d: %s",
			lineNum,
			line,
		)
	}

	return models.BacalhauSettings{
		Key:   key,
		Value: value,
	}, nil
}
