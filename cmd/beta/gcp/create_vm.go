package gcp

import (
	"fmt"
	"os"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func GetCreateVMCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-vm <project-id>",
		Short: "Create a new VM in a GCP project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := createVM(cmd, args)
			if err != nil {
				fmt.Println(err)
				return nil
			}
			return nil
		},
	}

	cmd.Flags().StringP("zone", "z", "", "The zone where the VM will be created")
	cmd.Flags().StringP("machine-type", "m", "", "The machine type for the VM")
	_ = cmd.MarkFlagRequired("zone")
	_ = cmd.MarkFlagRequired("machine-type")

	return cmd
}

// This is only for doing one offs - the create-deployment will not use this function.
func createVM(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("project ID is required")
	}
	projectID := args[0]

	zone, err := cmd.Flags().GetString("zone")
	if err != nil {
		return err
	}

	machineType, err := cmd.Flags().GetString("machine-type")
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	p, err := gcp.NewGCPProviderFunc(ctx)
	if err != nil {
		return err
	}

	sshUser := viper.GetString("general.ssh_user")
	if sshUser == "" {
		return fmt.Errorf("ssh user is not set")
	}

	publicKeyPath := viper.GetString("general.ssh_public_key_path")
	publicKeyMaterial, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return err
	}

	if publicKeyMaterial == nil {
		return fmt.Errorf("public key material is nil, read from file %s", publicKeyPath)
	}

	vmName := fmt.Sprintf("vm-%s", projectID)

	organizationID := viper.GetString("gcp.organization_id")
	if organizationID == "" {
		return fmt.Errorf("organization ID is not set")
	}

	billingAccountID := viper.GetString("gcp.billing_account_id")
	if billingAccountID == "" {
		return fmt.Errorf("billing account ID is not set")
	}

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}
	m.Deployment.ProjectID = projectID
	m.Deployment.GCP.ProjectID = projectID
	m.Deployment.GCP.OrganizationID = organizationID
	m.Deployment.GCP.BillingAccountID = billingAccountID
	m.Deployment.GCP.Region = getRegionFromZone(zone)
	m.Deployment.GCP.Zone = zone
	m.Deployment.Machines = map[string]*models.Machine{
		vmName: {
			Name:                 vmName,
			VMSize:               machineType,
			SSHUser:              sshUser,
			SSHPublicKeyMaterial: publicKeyMaterial,
			CloudSpecific: models.CloudSpecificInfo{
				Zone:   zone,
				Region: getRegionFromZone(zone),
			},
		},
	}

	vm, err := p.CreateComputeInstance(ctx, vmName)
	if err != nil {
		if strings.Contains(err.Error(), "Unknown zone") {
			return fmt.Errorf(
				"invalid zone '%s'. Please use a valid zone. You can list all available zones using the following command:\n\tgcloud compute zones list",
				zone,
			)
		}
		return err
	}

	// Get the external IP address of the VM
	externalIP, err := p.GetVMExternalIP(ctx, projectID, zone, *vm.Name)
	if err != nil {
		return err
	}

	fmt.Printf("VM created successfully: %s (External IP: %s)\n", *vm.Name, externalIP)
	return nil
}

func getRegionFromZone(zone string) string {
	// GCP zones are typically in the format of <region>-<zone>, e.g., us-central1-a
	parts := strings.Split(zone, "-")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], "-")
}
