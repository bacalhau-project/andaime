package utils

import (
	"fmt"
	"os"
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
}

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
	for i := len(parts) - 2; i >= 0; i-- {
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

	if err := os.WriteFile(file, content, 0644); err != nil {
		return err
	}

	viper.SetConfigFile(file)
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	return nil
}
