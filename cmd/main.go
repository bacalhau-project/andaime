//nolint:unused,gomnd
package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bacalhau-project/andaime/providers/aws"
	"sigs.k8s.io/yaml"
)

type Config struct {
	TagPrefix string                    `yaml:"tag_prefix"`
	Providers map[string]ProviderConfig `yaml:"providers"`
}

type ProviderConfig struct {
	AWS *aws.SpotInstanceConfig `yaml:"aws,omitempty"`
	// Add other providers here as needed
}

func loadConfig(filename string) (Config, error) {
	var cfg Config
	file, err := os.ReadFile(filename)
	if err != nil {
		return cfg, err
	}
	err = yaml.Unmarshal(file, &cfg)
	return cfg, err
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  andaime deploy")
		fmt.Println("  andaime delete <deployment-tag>")
		os.Exit(1)
	}

	cfg, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	switch os.Args[1] {
	case "deploy":
		if err := deploy(cfg); err != nil {
			log.Fatalf("Error deploying: %v", err)
		}
	case "delete":
		if len(os.Args) < 3 {
			log.Fatal("Please provide the deployment tag to delete")
		}
		if err := deleteDeployment(cfg, os.Args[2]); err != nil {
			log.Fatalf("Error deleting deployment: %v", err)
		}
	default:
		log.Fatalf("Unknown command: %s", os.Args[1])
	}
}

func deploy(cfg Config) error {
	for providerName, providerConfig := range cfg.Providers {
		switch providerName {
		case "aws":
			if providerConfig.AWS != nil {
				// You'll need to implement this function in the aws package
				instances, err := aws.CreateSpotInstancesInRegion(
					context.TODO(),
					"us-west-2", // You might want to make this configurable
					[]string{},  // orchestrators
					"",          // token
					5,           // instancesPerRegion
					*providerConfig.AWS,
				)
				if err != nil {
					return fmt.Errorf("failed to create AWS instances: %w", err)
				}
				fmt.Printf("Created %d AWS instances\n", len(instances))
			}
		// Add cases for other providers here
		default:
			log.Printf("Unsupported provider: %s", providerName)
		}
	}
	return nil
}

func deleteDeployment(_ Config, _ string) error {
	// Implement deletion logic here
	return fmt.Errorf("deletion not implemented yet")
}
