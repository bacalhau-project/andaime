package gcp

import (
	"fmt"

	"github.com/spf13/cobra"
)

func getCreateDeploymentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-deployment",
		Short: "Create a new deployment in GCP",
		Long:  `Create a new deployment in Google Cloud Platform (GCP).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreateDeployment()
		},
	}

	return cmd
}

func runCreateDeployment() error {
	fmt.Println("Creating deployment in GCP...")
	// TODO: Implement the actual deployment creation logic
	return nil
}
package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCreateDeploymentCmd(t *testing.T) {
	cmd := getCreateDeploymentCmd()
	assert.NotNil(t, cmd)
	assert.Equal(t, "create-deployment", cmd.Use)
	assert.Equal(t, "Create a new deployment in GCP", cmd.Short)
}
