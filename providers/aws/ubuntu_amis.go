package aws

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/utils"
)

func GetAndPrintUbuntuAMIs(ctx context.Context) error {
	regions, err := utils.GetAWSRegions()
	if err != nil {
		return fmt.Errorf("failed to get AWS regions: %w", err)
	}

	provider := NewAWSProvider()
	ubuntuAMIs := make(map[string]string)
	for _, region := range regions {
		amiID, err := provider.GetLatestUbuntuImage(ctx, region)
		if err != nil {
			return fmt.Errorf("failed to get AMI for region %s: %w", region, err)
		}
		ubuntuAMIs[region] = amiID
	}

	PrintUbuntuAMIs(ubuntuAMIs)
	return nil
}

func (p *AWSProvider) GetUbuntuAMIs(ctx context.Context) (map[string]string, error) {
	regions, err := utils.GetAWSRegions()
	if err != nil {
		return nil, fmt.Errorf("failed to get AWS regions: %w", err)
	}

	ubuntuAMIs := make(map[string]string)
	for _, region := range regions {
		amiID, err := p.GetLatestUbuntuImage(ctx, region)
		if err != nil {
			return nil, fmt.Errorf("failed to get AMI for region %s: %w", region, err)
		}
		ubuntuAMIs[region] = amiID
	}

	return ubuntuAMIs, nil
}

func PrintUbuntuAMIs(amis map[string]string) {
	fmt.Println("UBUNTU_AMIS = {")
	for region, amiID := range amis {
		fmt.Printf("    \"%s\": \"%s\",\n", region, amiID)
	}
	fmt.Println("}")
}
