package models

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bacalhau-project/andaime/internal"
	"github.com/spf13/viper"
)

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
	var badSettings []string

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

			if !internal.IsValidBacalhauKey(key) {
				badSettings = append(badSettings, key)
				continue
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
			if !internal.IsValidBacalhauKey(key) {
				badSettings = append(badSettings, key)
				continue
			}

			result = append(result, BacalhauSettings{Key: key, Value: value})
		}

	case map[string]string:
		// Handle the map[string]string structure (another possible format)
		for key, value := range settings {
			if !internal.IsValidBacalhauKey(key) {
				badSettings = append(badSettings, key)
				continue
			}

			result = append(result, BacalhauSettings{Key: key, Value: value})
		}

	default:
		return nil, fmt.Errorf(
			"invalid bacalhau_settings type: expected []interface{} or map[string]interface{}, got %T",
			bacalhauSettings,
		)
	}

	if len(badSettings) > 0 {
		return nil, fmt.Errorf(
			"invalid bacalhau_settings keys: %s",
			strings.Join(badSettings, ", "),
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
