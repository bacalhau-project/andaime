package utils

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

var (
	NumberOfSSHRetries      = 3
	TimeInBetweenSSHRetries = 2 * time.Second
	SSHTimeOut              = 30 * time.Second
)

type Config struct {
	AWS struct {
		Regions []string `yaml:"regions"`
	} `yaml:"aws"`
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}
