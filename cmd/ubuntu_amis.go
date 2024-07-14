package cmd

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/providers/aws"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ubuntuAMIsCmd represents the ubuntuAMIs command
var ubuntuAMIsCmd = &cobra.Command{
	Use:   "ubuntu-amis",
	Short: "Retrieve latest Ubuntu AMIs for specified AWS regions",
	Long: `This command retrieves the latest Ubuntu AMIs for the AWS regions
specified in the configuration file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create AWS provider
		provider := aws.NewAWSProvider() // We'll need to implement this

		// Get regions from Viper config
		regions := viper.GetStringSlice("aws.regions")
		if len(regions) == 0 {
			return fmt.Errorf("no AWS regions specified in config")
		}

		// Get Ubuntu AMIs
		ubuntuAMIs := make(map[string]string)
		for _, region := range regions {
			amiID, err := provider.GetLatestUbuntuImage(context.Background(), region)
			if err != nil {
				return fmt.Errorf("failed to get AMI for region %s: %w", region, err)
			}
			ubuntuAMIs[region] = amiID
		}

		// Print results
		aws.PrintUbuntuAMIs(ubuntuAMIs)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(ubuntuAMIsCmd)
}
