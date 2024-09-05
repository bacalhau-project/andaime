//nolint:unused,gomnd
package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bacalhau-project/andaime/pkg/logger"
	awsprovider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"sigs.k8s.io/yaml"
)

const DefaultInstancesPerRegion = 5

type Config struct {
	TagPrefix string                    `yaml:"tag_prefix"`
	Providers map[string]ProviderConfig `yaml:"providers"`
}

type ProviderConfig struct {
	AWS *awsprovider.SpotInstanceConfig `yaml:"aws,omitempty"`
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
	// Constants
	const (
		MinimumArgCount        = 2
		MinimumArgCountForTest = 3
		MinArgsForDelete       = 3
	)

	if len(os.Args) < MinimumArgCount {
		fmt.Println("Usage:")
		fmt.Println("  andaime deploy")
		fmt.Println("  andaime delete <deployment-tag>")
		return
	}

	if len(os.Args) < MinimumArgCountForTest {
		fmt.Println("Usage:")
		fmt.Println("  andaime deploy")
		fmt.Println("  andaime delete <deployment-tag>")
		return
	}

	cfg, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Check for GCP credentials
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		fmt.Println("GOOGLE_APPLICATION_CREDENTIALS is not set. Please set up your credentials using the following steps:")
		fmt.Println("1. Install the gcloud CLI if you haven't already: https://cloud.google.com/sdk/docs/install")
		fmt.Println("2. Run the following commands in your terminal:")
		fmt.Println("   gcloud auth login")
		fmt.Println("   gcloud auth application-default login")
		fmt.Println("3. The above command will set up your user credentials.")
		fmt.Println("After completing these steps, run your application again.")
		return
	}

	switch os.Args[1] {
	case "deploy":
		if err := deploy(cfg); err != nil {
			log.Fatalf("Error deploying: %v", err)
		}
	case "delete":
		if len(os.Args) < MinArgsForDelete {
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
	l := logger.Get()
	for providerName, providerConfig := range cfg.Providers {
		switch providerName {
		case "aws":
			if providerConfig.AWS != nil {
				// You'll need to implement this function in the aws package
				instances, err := awsprovider.CreateSpotInstancesInRegion(
					context.TODO(),
					"us-west-2",               // You might want to make this configurable
					[]string{},                // orchestrators
					"",                        // token
					DefaultInstancesPerRegion, // instancesPerRegion
					*providerConfig.AWS,
				)
				if err != nil {
					return fmt.Errorf("failed to create AWS instances: %w", err)
				}
				l.Debugf("Created %d AWS instances", len(instances))
			}
		// Add cases for other providers here
		default:
			l.Errorf("Unsupported provider: %s", providerName)
		}
	}
	return nil
}

func deleteDeployment(_ Config, _ string) error {
	// Implement deletion logic here
	return fmt.Errorf("deletion not implemented yet")
}
