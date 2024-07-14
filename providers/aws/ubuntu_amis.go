package aws

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/utils"
func main() {
	ctx := context.Background()
	if err := aws.GetAndPrintUbuntuAMIs(ctx); err != nil {
		panic(err)
	}
}

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

func PrintUbuntuAMIs(ubuntuAMIs map[string]string) {
	fmt.Println("Ubuntu AMIs:")
	for region, amiID := range ubuntuAMIs {
		fmt.Printf("  %s: %s\n", region, amiID)
	}
}
