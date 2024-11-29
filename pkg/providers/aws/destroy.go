package awsprovider

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/spf13/viper"
)

// DestroyResources deletes the specified AWS VPC and associated resources
func (p *AWSProvider) DestroyResources(ctx context.Context, vpcID string) error {
	l := logger.Get()
	l.Infof("Starting destruction of AWS deployment (VPC ID: %s)", vpcID)

	// If VPC ID is empty, just remove it from the config
	if vpcID == "" {
		l.Info("Empty VPC ID provided, cleaning up configuration")
		// Find the deployment in config that has an empty vpc_id
		deployments := viper.GetStringMap("deployments.aws")
		for uniqueID, deployment := range deployments {
			if d, ok := deployment.(map[string]interface{}); ok {
				if vpcID, exists := d["vpc_id"]; exists && (vpcID == "" || vpcID == nil) {
					// Just remove the vpc_id field
					delete(d, "vpc_id")
					viper.Set(fmt.Sprintf("deployments.aws.%s", uniqueID), d)
					if err := viper.WriteConfig(); err != nil {
						return fmt.Errorf("failed to update config file: %w", err)
					}
					l.Infof("Successfully removed vpc_id from deployment %s", uniqueID)
				}
			}
		}
		return nil
	}

	// Call the Destroy method we implemented
	err := p.Destroy(ctx)
	if err != nil {
		return fmt.Errorf("failed to destroy AWS resources: %w", err)
	}

	l.Info("AWS resources successfully destroyed")
	return nil
}
