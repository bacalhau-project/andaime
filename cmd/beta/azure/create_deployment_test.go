package azure

import (
	"bytes"
	"context"
	"errors"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	armresources "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"
)

func TestCreateDeploymentCmd(t *testing.T) {
	_, err := testutil.InitializeTestViper()
	if err != nil {
		t.Fatalf("Failed to initialize test Viper: %v", err)
	}

	testSSHPublicKeyFile, cleanup_public_key, err := testutil.WriteStringToTempFile(testdata.TestPublicSSHKeyMaterial)
	defer cleanup_public_key()

	testSSHPrivateKeyFile, cleanup_private_key, err := testutil.WriteStringToTempFile(testdata.TestPrivateSSHKeyMaterial)
	defer cleanup_private_key()

	if err != nil {
		t.Fatalf("Failed to write test SSH public key file: %v", err)
	}

	tests := []struct {
		name           string
		configSetup    func() (*viper.Viper, error)
		clientSetup    func(subscriptionID string) (azure.AzureClient, error)
		expectedOutput string
		expectedError  string
	}{
		{
			name: "Successful deployment",
			configSetup: func() (*viper.Viper, error) {
				viper.Set("general.ssh_public_key_path", testSSHPublicKeyFile)
				viper.Set("general.ssh_private_key_path", testSSHPrivateKeyFile)
				viper.Set("general.ssh_key_path", strings.TrimSuffix(testSSHPublicKeyFile, ".pub"))
				return viper.GetViper(), nil
			},
			clientSetup: func(subscriptionID string) (azure.AzureClient, error) {
				client := azure.GetMockAzureClient().(*azure.MockAzureClient)
				client.GetOrCreateResourceGroupFunc = func(ctx context.Context, location string, name string) (*armresources.ResourceGroup, error) {
					return &armresources.ResourceGroup{
						Name:     to.Ptr(name),
						Location: &location,
					}, nil
				}

				client.CreateVirtualNetworkFunc = func(ctx context.Context, resourceGroupName string, vnetName string, parameters armnetwork.VirtualNetwork) (armnetwork.VirtualNetwork, error) {
					return testdata.TestVirtualNetwork, nil
				}
				client.CreatePublicIPFunc = func(ctx context.Context, resourceGroupName string, ipName string, parameters armnetwork.PublicIPAddress) (armnetwork.PublicIPAddress, error) {
					return testdata.TestPublicIPAddress, nil
				}

				client.CreateNetworkSecurityGroupFunc = func(ctx context.Context, resourceGroupName string, nsgName string, parameters armnetwork.SecurityGroup) (armnetwork.SecurityGroup, error) {
					return testdata.TestNSG, nil
				}

				client.CreateNetworkInterfaceFunc = func(ctx context.Context, resourceGroupName string, networkInterfaceName string, parameters armnetwork.Interface) (armnetwork.Interface, error) {
					return testdata.TestInterface, nil
				}

				client.CreateVirtualMachineFunc = func(ctx context.Context, resourceGroupName string, vmName string, parameters armcompute.VirtualMachine) (armcompute.VirtualMachine, error) {
					return testdata.TestVirtualMachine, nil
				}
				return client, nil
			},
			expectedOutput: "Azure deployment created successfully\n",
		},
		{
			name: "Provider creation error",
			configSetup: func() (*viper.Viper, error) {
				testConfig, err := testutil.InitializeTestViper()
				if err != nil {
					return nil, err
				}
				return testConfig, nil
			},
			clientSetup: func(subscriptionID string) (azure.AzureClient, error) {
				return nil, assert.AnError
			},
			expectedError: "failed to initialize Azure provider",
		},
		{
			name: "Deployment error",
			configSetup: func() (*viper.Viper, error) {
				viper.Set("general.ssh_public_key_path", testSSHPublicKeyFile)
				viper.Set("general.ssh_private_key_path", testSSHPrivateKeyFile)
				viper.Set("general.ssh_key_path", strings.TrimSuffix(testSSHPublicKeyFile, ".pub"))
				return viper.GetViper(), nil
			},
			clientSetup: func(subscriptionID string) (azure.AzureClient, error) {
				mockClient := &azure.MockAzureClient{
					GetOrCreateResourceGroupFunc: func(ctx context.Context, location string, name string) (*armresources.ResourceGroup, error) {
						return &armresources.ResourceGroup{
							Name:     to.Ptr(name),
							Location: &location,
						}, nil
					},
					CreateVirtualNetworkFunc: func(ctx context.Context, resourceGroupName string, vnetName string, parameters armnetwork.VirtualNetwork) (armnetwork.VirtualNetwork, error) {
						return armnetwork.VirtualNetwork{}, errors.New("UNIQUE_VIRTUAL_NETWORK_ERROR")
					},
				}
				return mockClient, nil
			},
			expectedError: "UNIQUE_VIRTUAL_NETWORK_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic in test: %v", r)
					debug.PrintStack()
				}
			}()

			testConfig, err := tt.configSetup()
			assert.NoError(t, err)

			oldAzureProviderFunc := azure.AzureProviderFunc
			azure.AzureProviderFunc = func(_ *viper.Viper) (azure.AzureProviderer, error) {
				client, err := tt.clientSetup(testConfig.GetString("azure.subscription_id"))
				if err != nil {
					return nil, err
				}
				return &azure.AzureProvider{
					Client: client,
					Config: testConfig,
				}, nil
			}
			defer func() {
				azure.AzureProviderFunc = oldAzureProviderFunc
			}()

			oldSSHWaiter := sshutils.NewSSHClientFunc
			sshutils.NewSSHClientFunc = func(sshClientConfig *ssh.ClientConfig,
				dialer sshutils.SSHDialer) sshutils.SSHClienter {
				client, _ := sshutils.NewMockSSHClient(sshutils.NewMockSSHDialer())
				mockSession := sshutils.NewMockSSHSession()
				mockSession.On("Close", mock.Anything).Return(nil)
				client.On("NewSession", mock.Anything).Return(mockSession, nil)
				client.On("Close", mock.Anything).Return(nil)
				return client
			}
			defer func() {
				sshutils.NewSSHClientFunc = oldSSHWaiter
			}()

			cmd := &cobra.Command{}
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetContext(context.Background())

			// Set Viper instance for the command
			cmd.SetContext(context.WithValue(cmd.Context(), "viper", testConfig))

			err = executeCreateDeployment(cmd, nil)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, buf.String(), tt.expectedOutput)
			}
		})
	}
}
