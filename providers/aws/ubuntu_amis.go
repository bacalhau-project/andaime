package aws

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
)

func (p *AWSProvider) GetUbuntuAMIs(ctx context.Context) (map[string]string, error) {
	regions := viper.GetStringSlice("aws.regions")
	if len(regions) == 0 {
		return nil, fmt.Errorf("no AWS regions specified in config")
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