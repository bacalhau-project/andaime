package provision

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
)

// SettingsParser handles the parsing of Bacalhau settings files
type SettingsParser struct {
	// Configuration constants
	maxKeyLength   int
	maxValueLength int
	keyPattern     *regexp.Regexp
}

// NewSettingsParser creates a new settings parser with default configuration
func NewSettingsParser() *SettingsParser {
	return &SettingsParser{
		maxKeyLength:   256,  //nolint:mnd
		maxValueLength: 4096, //nolint:mnd
		keyPattern:     regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`),
	}
}

// WithMaxKeyLength sets the maximum allowed key length
func (p *SettingsParser) WithMaxKeyLength(length int) *SettingsParser {
	p.maxKeyLength = length
	return p
}

// WithMaxValueLength sets the maximum allowed value length
func (p *SettingsParser) WithMaxValueLength(length int) *SettingsParser {
	p.maxValueLength = length
	return p
}

// ParseFile reads and parses a settings file, returning the settings or an error
func (p *SettingsParser) ParseFile(filePath string) ([]models.BacalhauSettings, error) {
	l := logger.Get()
	if filePath == "" {
		l.Debug("No settings file provided. Skipping settings parsing")
		return []models.BacalhauSettings{}, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open settings file: %w", err)
	}
	defer file.Close()

	return p.parse(file)
}

// ParseString parses settings from a string
func (p *SettingsParser) ParseString(content string) ([]models.BacalhauSettings, error) {
	return p.parseReader(strings.NewReader(content))
}

// parse reads from a file and returns the parsed settings
func (p *SettingsParser) parse(file *os.File) ([]models.BacalhauSettings, error) {
	return p.parseReader(file)
}

// parseReader reads from any io.Reader and returns the parsed settings
func (p *SettingsParser) parseReader(reader interface{}) ([]models.BacalhauSettings, error) {
	var settings []models.BacalhauSettings
	scanner := bufio.NewScanner(reader.(interface{ Read([]byte) (int, error) }))
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
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		settings = append(settings, setting)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading settings: %w", err)
	}

	return settings, nil
}

// parseLine parses a single line of the settings file
func (p *SettingsParser) parseLine(line string, _ int) (models.BacalhauSettings, error) {
	// Split on first colon
	parts := strings.SplitN(line, ":", 2) //nolint:mnd
	if len(parts) != 2 {                  //nolint:mnd
		return models.BacalhauSettings{}, fmt.Errorf("invalid format: missing key-value separator ':', content: %s", line)
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Validate key
	if err := p.validateKey(key); err != nil {
		return models.BacalhauSettings{}, fmt.Errorf("invalid key %q: %w", key, err)
	}

	// Validate and clean value
	value, err := p.validateValue(value)
	if err != nil {
		return models.BacalhauSettings{}, fmt.Errorf("invalid value for key %q: %w", key, err)
	}

	return models.BacalhauSettings{
		Key:   key,
		Value: value,
	}, nil
}

// validateKey validates the key format and length
func (p *SettingsParser) validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("empty key")
	}
	if len(key) > p.maxKeyLength {
		return fmt.Errorf("exceeds maximum length of %d", p.maxKeyLength)
	}
	if !p.keyPattern.MatchString(key) {
		return fmt.Errorf("must contain only alphanumeric characters, underscores, dashes, and periods")
	}
	return nil
}

// validateValue validates and cleans the value
func (p *SettingsParser) validateValue(value string) (string, error) {
	// Remove surrounding quotes if present
	value = strings.Trim(value, `"`)

	if value == "" {
		return "", fmt.Errorf("empty value")
	}
	if len(value) > p.maxValueLength {
		return "", fmt.Errorf("exceeds maximum length of %d", p.maxValueLength)
	}
	return value, nil
}
