package utils

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"
)

var mu sync.Mutex

type Config struct {
	AWS struct {
		Regions []string `yaml:"regions"`
	} `yaml:"aws"`
	GCP struct {
		ProjectID string `yaml:"project_id"`
	} `yaml:"gcp"`
}

const (
	ConfigFilePermissions = 0600
)

func DeleteUniqueIDFromConfig(uniqueID string) error {
	mu.Lock()
	defer mu.Unlock()

	file := viper.ConfigFileUsed()
	if file == "" {
		return fmt.Errorf("no config file found")
	}

	tempViper := viper.New()
	tempViper.SetConfigFile(file)
	if err := tempViper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %v", err)
	}

	configMap := tempViper.AllSettings()

	// Get the deployments map
	if deploymentsMap, ok := configMap["deployments"].(map[string]interface{}); ok {
		// Delete the specific deployment
		delete(deploymentsMap, uniqueID)

		// If deployments is empty, remove it entirely
		if len(deploymentsMap) == 0 {
			delete(configMap, "deployments")
		}
	}

	content, err := yaml.Marshal(configMap)
	if err != nil {
		return err
	}

	if err := os.WriteFile(file, content, ConfigFilePermissions); err != nil {
		return err
	}

	viper.SetConfigFile(file)
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	return nil
}

func StripAndParseJSON(input string) ([]map[string]interface{}, error) {
	// Find the start of the JSON array
	start := strings.Index(input, "[")
	if start == -1 {
		return nil, fmt.Errorf("no JSON array found in input")
	}

	// Extract the JSON part
	jsonStr := input[start:]

	// Parse the JSON
	var result []map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	return result, nil
}

func IsValidGUID(guid string) bool {
	r := regexp.MustCompile(
		"^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$",
	)
	return r.MatchString(guid)
}

// GenerateUniqueID generates a unique ID of length 8
func GenerateUniqueID() string {
	return generateID(8) //nolint:mnd
}

// CreateShortID generates a short ID of length 6
func CreateShortID() string {
	return generateID(6) //nolint:mnd
}

func generateID(length int) string {
	l := logger.Get()

	var letters = []rune("bcdfghjklmnpqrstvwxz")
	b := make([]rune, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			l.Fatalf("Failed to generate unique ID: %v", err)
		}
		b[i] = letters[n.Int64()]
	}
	return string(b)
}

func InitConfig(configFile string) (string, error) {
	l := logger.Get()
	l.Debug("Starting initConfig")

	viper.SetConfigType("yaml")

	if configFile != "" {
		// Use config file from the flag.
		absPath, err := filepath.Abs(configFile)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path for config file: %w", err)
		}
		viper.SetConfigFile(absPath)
		l.Debugf("Using config file specified by flag: %s", absPath)
	} else {
		// Search for config in the working directory
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		l.Debug("No config file specified, using default: ./config.yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if configFile != "" {
				// Only return an error if a specific config file was requested
				return "", fmt.Errorf("config file not found: %s", configFile)
			}
			l.Debug("No config file found, using defaults and environment variables")
		} else {
			return "", fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		l.Infof("Using config file: %s", viper.ConfigFileUsed())
	}

	l.Info("Configuration initialization complete")
	return configFile, nil
}
