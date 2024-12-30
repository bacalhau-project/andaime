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
	testCases := []struct {
		name            string
		networkErrors   []error
		vmError         error
		expectedError   string
		expectedRetry   int
		expectedPublic  string
		expectedPrivate string
		setupContext    func(context.Context) (context.Context, context.CancelFunc)
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
			setupContext: func(baseCtx context.Context) (context.Context, context.CancelFunc) {
				return context.WithTimeout(baseCtx, 10*time.Second)
			},
			expectedLogs: []string{
				"Getting IP addresses for VM test-vm in resource group test-resource-group (attempt 1/3)",
				"Getting network interface test-network-interface",
				"Network interface not found (retriable error, attempt 1/3)",
				"Getting IP addresses for VM test-vm in resource group test-resource-group (attempt 2/3)",
				"Successfully retrieved network interface",
				"Found private IP: 10.0.0.4",
				"Getting public IP address test-public-ip",
				"Found public IP: 1.2.3.4",
			},
		},
		{
			name: "Fail after max retries",
			networkErrors: []error{
				&AzureError{Code: "ResourceNotFound", Message: "Network interface not found"},
				&AzureError{Code: "ResourceNotFound", Message: "Network interface not found"},
				&AzureError{Code: "ResourceNotFound", Message: "Network interface not found"},
			},
			vmError:       nil,
			expectedError: "exceeded maximum retries",
			expectedRetry: 2,
			setupContext: func(baseCtx context.Context) (context.Context, context.CancelFunc) {
				return context.WithTimeout(baseCtx, 10*time.Second)
			},
			expectedLogs: []string{
				"Getting IP addresses for VM test-vm in resource group test-resource-group (attempt 1/3)",
				"Getting network interface test-network-interface",
				"Network interface not found (retriable error, attempt 1/3)",
				"Getting IP addresses for VM test-vm in resource group test-resource-group (attempt 2/3)",
				"Network interface not found (retriable error, attempt 2/3)",
				"Getting IP addresses for VM test-vm in resource group test-resource-group (attempt 3/3)",
				"Network interface not found (retriable error, attempt 3/3)",
				"Exceeded maximum retries (3) while getting network interface",
			},
		},
		{
			name: "Non-retriable error",
			networkErrors: []error{
				&AzureError{Code: "AuthorizationFailed", Message: "Not authorized"},
			},
			vmError:       nil,
			expectedError: "Not authorized",
			expectedRetry: 0,
			setupContext: func(baseCtx context.Context) (context.Context, context.CancelFunc) {
				return context.WithTimeout(baseCtx, 10*time.Second)
			},
			expectedLogs: []string{
				"Getting IP addresses for VM test-vm in resource group test-resource-group (attempt 1/3)",
				"Getting network interface test-network-interface",
				"Non-retriable error getting network interface: Not authorized",
			},
		},
		{
			name:          "Context cancellation",
			networkErrors: []error{context.Canceled},
			vmError:       nil,
			expectedError: "context canceled",
			expectedRetry: 0,
			setupContext: func(baseCtx context.Context) (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(baseCtx)
				go func() {
					time.Sleep(100 * time.Millisecond)
					cancel()
				}()
				return ctx, cancel
			},
			expectedLogs: []string{
				"Getting IP addresses for VM test-vm in resource group test-resource-group (attempt 1/3)",
				"Getting network interface test-network-interface",
				"Context cancelled while waiting for IP addresses",
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			testCtx := s.Ctx
			if tc.setupContext != nil {
				var cancel context.CancelFunc
				testCtx, cancel = tc.setupContext(testCtx)
				defer cancel()
			} else {
				var cancel context.CancelFunc
				testCtx, cancel = context.WithTimeout(testCtx, 10*time.Second)
				defer cancel()
			}

			if tc.vmError != nil {
				s.MockAzureClient.On("GetVirtualMachine",
					mock.Anything, mock.Anything, mock.Anything).
					Return(nil, tc.vmError).Once()
			} else {
				s.MockAzureClient.On("GetVirtualMachine",
					mock.Anything, mock.Anything, mock.Anything).
					Return(&armcompute.VirtualMachine{
						Properties: &armcompute.VirtualMachineProperties{
							NetworkProfile: &armcompute.NetworkProfile{
								NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
									{
										ID: to.Ptr("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test-resource-group/providers/Microsoft.Network/networkInterfaces/test-network-interface"),
									},
								},
							},
						},
					}, nil).Times(tc.expectedRetry + 1)
			}

			currentError := 0
			s.MockAzureClient.On("GetNetworkInterface",
				mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					if currentError >= len(tc.networkErrors) {
						s.Fail("Too many network client calls")
					}
				}).
				Return(func(ctx context.Context, rg, name string) (*armnetwork.Interface, error) {
					if currentError >= len(tc.networkErrors) {
						return nil, fmt.Errorf("unexpected call")
					}
					err := tc.networkErrors[currentError]
					currentError++
					if err != nil {
						return nil, err
					}
					return testdata.FakeNetworkInterface(), nil
				}).Times(len(tc.networkErrors))

			if tc.expectedPublic != "" {
				s.MockAzureClient.On("GetPublicIPAddress",
					mock.Anything, mock.Anything, mock.Anything).
					Return(&armnetwork.PublicIPAddress{
						Properties: &armnetwork.PublicIPAddressPropertiesFormat{
							IPAddress: &tc.expectedPublic,
						},
					}, nil).Once()
			}

			publicIP, privateIP, err := s.provider.GetVMIPAddresses(testCtx, "test-resource-group", "test-vm")

			if tc.expectedError != "" {
				s.Error(err)
				s.Contains(err.Error(), tc.expectedError)
			} else {
				s.NoError(err)
				s.Equal(tc.expectedPublic, publicIP)
				s.Equal(tc.expectedPrivate, privateIP)
			}

			s.Equal(tc.expectedRetry, currentError-1)

			logs := s.GetLogs()
			s.T().Logf("Available logs: %v", logs)
			for _, expectedLog := range tc.expectedLogs {
				found := false
				for _, log := range logs {
					if strings.Contains(log, expectedLog) {
						found = true
						break
					}
				}
				s.True(found, "Expected log message not found: %s", expectedLog)
			}
		})
	}
}
