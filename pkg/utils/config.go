package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

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

func DeleteKeyFromConfig(key string) error {
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

	// Split the key into parts
	parts := strings.Split(key, ".")

	// Navigate through the nested maps
	current := configMap
	for _, part := range parts[:len(parts)-1] {
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			// If the path doesn't exist, we're done
			return nil
		}
	}

	// Delete the final key
	delete(current, parts[len(parts)-1])

	// Clean up empty maps
	const parentDirIndex = 2
	for i := len(parts) - parentDirIndex; i >= 0; i-- {
		parent := configMap
		for _, part := range parts[:i] {
			parent = parent[part].(map[string]interface{})
		}
		if len(parent[parts[i]].(map[string]interface{})) == 0 {
			delete(parent, parts[i])
		} else {
			break
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

type BacalhauSettings struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

func ReadBacalhauSettingsFromViper() ([]BacalhauSettings, error) {
	bacalhauSettings := viper.Get("general.bacalhau_settings")

	if bacalhauSettings == nil {
		return nil, nil
	}

	var result []BacalhauSettings

	switch settings := bacalhauSettings.(type) {
	case []interface{}:
		// Handle the slice of maps structure (as in the YAML example)
		for _, setting := range settings {
			settingMap, ok := setting.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid setting type: expected map[string]interface{}, got %T", setting)
			}

			key, ok := settingMap["key"].(string)
			if !ok {
				return nil, fmt.Errorf("invalid key type: expected string, got %T", settingMap["Key"])
			}

			value, ok := settingMap["value"]
			if !ok {
				return nil, fmt.Errorf("missing Value for key: %s", key)
			}

			result = append(result, BacalhauSettings{Key: key, Value: value})
		}

	case map[string]interface{}:
		// Handle the map structure (as it appears in your tests)
		for key, value := range settings {
			result = append(result, BacalhauSettings{Key: key, Value: value})
		}

	case map[string]string:
		// Handle the map[string]string structure (another possible format)
		for key, value := range settings {
			result = append(result, BacalhauSettings{Key: key, Value: value})
		}

	default:
		return nil, fmt.Errorf(
			"invalid bacalhau_settings type: expected []interface{} or map[string]interface{}, got %T",
			bacalhauSettings,
		)
	}

	// Process the values to ensure consistent types
	for i, setting := range result {
		switch v := setting.Value.(type) {
		case string:
			// Keep as is
		case []interface{}:
			stringSlice := make([]string, 0, len(v))
			for _, item := range v {
				strItem, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("invalid value type in slice for key %s: expected string, got %T", setting.Key, item)
				}
				stringSlice = append(stringSlice, strItem)
			}
			result[i].Value = stringSlice
		case bool:
			result[i].Value = strconv.FormatBool(v)
		default:
			return nil, fmt.Errorf("invalid value type for key %s: expected string, []string, bool or []interface{}, got %T", setting.Key, setting.Value)
		}
	}

	return result, nil
}
