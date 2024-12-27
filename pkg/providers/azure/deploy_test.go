package azure

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/pkg/logger"
	azure_mocks "github.com/bacalhau-project/andaime/mocks/azure"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type AzureDeployTestSuite struct {
	BaseAzureTestSuite
	provider *AzureProvider
}

func TestAzureDeploySuite(t *testing.T) {
	suite.Run(t, new(AzureDeployTestSuite))
}

func (s *AzureDeployTestSuite) SetupTest() {
	s.BaseAzureTestSuite.SetupTest()
	provider, err := NewAzureProvider(s.Ctx, "test-subscription-id")
	s.Require().NoError(err)
	provider.SetAzureClient(s.MockAzureClient)
	s.provider = provider
}

func (s *AzureDeployTestSuite) TestGetVMIPAddressesWithRetries() {
	// Initialize test context
	ctx := context.Background()

	// Create test resources
	resourceGroup := "test-resource-group"
	vmName := "test-vm"
	networkInterfaceID := "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test-resource-group/providers/Microsoft.Network/networkInterfaces/test-network-interface"

	// Test cases
	testCases := []struct {
		name            string
		networkErrors   []error
		vmError         error
		expectedError   string
		expectedRetry   int
		expectedPublic  string
		expectedPrivate string
		setupContext    func() (context.Context, context.CancelFunc)
		expectedLogs    []string
	}{
		{
			name: "Success after one retry",
			networkErrors: []error{
				&AzureError{Code: "ResourceNotFound", Message: "Network interface not found"},
				nil,
			},
			vmError:         nil,
			expectedRetry:   1,
			expectedPublic:  "1.2.3.4",
			expectedPrivate: "10.0.0.4",
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(ctx, 10*time.Second)
			},
			expectedLogs: []string{
				"Getting IP addresses for VM test-vm in resource group test-resource-group (attempt 1/3)",
				"Getting network interface test-network-interface",
				"Network interface not found (retriable error)",
				"Getting IP addresses for VM test-vm in resource group test-resource-group (attempt 2/3)",
				"Successfully retrieved network interface",
			},
		},
		{
			name: "Fail after max retries",
			networkErrors: []error{
				&AzureError{Code: "ResourceNotFound", Message: "Network interface not found"},
				&AzureError{Code: "ResourceNotFound", Message: "Network interface not found"},
				&AzureError{Code: "ResourceNotFound", Message: "Network interface not found"},
				&AzureError{Code: "ResourceNotFound", Message: "Network interface not found"},
			},
			vmError:       nil,
			expectedError: "exceeded maximum retries",
			expectedRetry: 3,
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(ctx, 10*time.Second)
			},
			expectedLogs: []string{
				"Getting IP addresses for VM test-vm in resource group test-resource-group (attempt 1/3)",
				"Getting network interface test-network-interface",
				"Network interface not found (retriable error)",
				"Getting IP addresses for VM test-vm in resource group test-resource-group (attempt 2/3)",
				"Exceeded maximum retries (3) while getting IP addresses",
			},
		},
		{
			name: "Non-retriable error",
			networkErrors: []error{
				&AzureError{Code: "AuthorizationFailed", Message: "Not authorized"},
			},
			vmError:       &AzureError{Code: "AuthorizationFailed", Message: "Not authorized"},
			expectedError: "Not authorized",
			expectedRetry: 0,
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(ctx, 10*time.Second)
			},
			expectedLogs: []string{
				"Getting IP addresses for VM test-vm in resource group test-resource-group (attempt 1/3)",
				"Getting network interface test-network-interface",
				"Non-retriable error getting network interface",
			},
		},
		{
			name: "Context cancellation",
			networkErrors: []error{
				&AzureError{Code: "ResourceNotFound", Message: "Network interface not found"},
			},
			vmError:       context.Canceled,
			expectedError: "context canceled",
			expectedRetry: 0,
			setupContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(ctx)
				go func() {
					time.Sleep(100 * time.Millisecond)
					cancel()
				}()
				return ctx, cancel
			},
			expectedLogs: []string{
				"Getting IP addresses for VM test-vm in resource group test-resource-group (attempt 1/3)",
				"Context cancelled while waiting",
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Set up test context and logger first
			s.SetupTest()
			testCtx, cancel := tc.setupContext()
			defer cancel()

			// Reset mock expectations and provider
			s.MockAzureClient = new(azure_mocks.MockAzureClienter)
			provider, err := NewAzureProvider(s.Ctx, "test-subscription-id")
			s.Require().NoError(err)
			provider.SetAzureClient(s.MockAzureClient)
			s.provider = provider

			// Mock GetVirtualMachine to return a VM with a network interface
			if tc.vmError != nil {
				s.MockAzureClient.On("GetVirtualMachine",
					mock.Anything, resourceGroup, vmName).
					Return(nil, tc.vmError).Once()
			} else {
				s.MockAzureClient.On("GetVirtualMachine",
					mock.Anything, resourceGroup, vmName).
					Return(&armcompute.VirtualMachine{
						Properties: &armcompute.VirtualMachineProperties{
							NetworkProfile: &armcompute.NetworkProfile{
								NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
									{
										ID: to.Ptr(networkInterfaceID),
									},
								},
							},
						},
					}, nil).Times(tc.expectedRetry + 1)
			}

			// Mock network interface responses
			currentError := 0
			s.MockAzureClient.On("GetNetworkInterface",
				mock.Anything, mock.Anything, mock.Anything).
				Return(func(ctx context.Context, rg, name string) (*armnetwork.Interface, error) {
					if currentError >= len(tc.networkErrors) {
						s.Fail("Too many network client calls")
						return nil, fmt.Errorf("unexpected call")
					}
					err := tc.networkErrors[currentError]
					currentError++
					if err != nil {
						return nil, err
					}
					// Use testdata.FakeNetworkInterface for consistent test data
					fakeInterface := testdata.FakeNetworkInterface()
					// Update IP addresses to match test case expectations
					if len(fakeInterface.Properties.IPConfigurations) > 0 {
						fakeInterface.Properties.IPConfigurations[0].Properties.PrivateIPAddress = &tc.expectedPrivate
						if fakeInterface.Properties.IPConfigurations[0].Properties.PublicIPAddress != nil {
							fakeInterface.Properties.IPConfigurations[0].Properties.PublicIPAddress.Properties = &armnetwork.PublicIPAddressPropertiesFormat{
								IPAddress: &tc.expectedPublic,
							}
						}
					}
					return fakeInterface, nil
				}).Maybe()

			// Mock GetPublicIPAddress responses
			s.MockAzureClient.On("GetPublicIPAddress",
				mock.Anything, mock.Anything, mock.Anything).
				Return(tc.expectedPublic, nil).Maybe()

			// Execute test
			publicIP, privateIP, err := s.provider.GetVMIPAddresses(testCtx, resourceGroup, vmName)

			// Verify results
			if tc.expectedError != "" {
				s.Error(err)
				s.Contains(err.Error(), tc.expectedError)
			} else {
				s.NoError(err)
				s.Equal(tc.expectedPublic, publicIP)
				s.Equal(tc.expectedPrivate, privateIP)
			}

			// Verify retry count
			s.Equal(tc.expectedRetry, currentError)

			// Verify logs
			logs := s.Logger.GetLogs()
			for _, expectedLog := range tc.expectedLogs {
				found := false
				for _, log := range logs {
					// Compare raw messages directly
					if strings.Contains(log, expectedLog) {
						found = true
						break
					}
				}
				s.True(found, "Expected log not found: %s", expectedLog)
			}
		})
	}
}
