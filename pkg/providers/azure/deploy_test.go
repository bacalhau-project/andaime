package azure

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
)

type AzureDeployTestSuite struct {
	suite.Suite
	provider *AzureProvider
	client   *LiveAzureClient
}

func (s *AzureDeployTestSuite) SetupTest() {
	s.client = &LiveAzureClient{}
	s.provider = &AzureProvider{
		client: s.client,
	}
}

func TestAzureDeploySuite(t *testing.T) {
	suite.Run(t, new(AzureDeployTestSuite))
}

func (s *AzureDeployTestSuite) TestGetVMIPAddressesWithRetries() {
	// Initialize test context and logger
	ctx := context.Background()
	l := logger.Get()

	// Create test resources
	resourceGroup := "test-resource-group"
	vmName := "test-vm"

	// Test cases
	testCases := []struct {
		name            string
		networkErrors   []error
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
			expectedRetry:   1,
			expectedPublic:  "1.2.3.4",
			expectedPrivate: "10.0.0.4",
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(ctx, 10*time.Second)
			},
			expectedLogs: []string{
				"Getting network interface",
				"Network interface not found (retriable error)",
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
			expectedError: "exceeded maximum retries",
			expectedRetry: 3,
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(ctx, 10*time.Second)
			},
			expectedLogs: []string{
				"Getting network interface",
				"Network interface not found (retriable error)",
				"Exceeded maximum retries",
			},
		},
		{
			name: "Non-retriable error",
			networkErrors: []error{
				&AzureError{Code: "AuthorizationFailed", Message: "Not authorized"},
			},
			expectedError: "Not authorized",
			expectedRetry: 0,
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(ctx, 10*time.Second)
			},
			expectedLogs: []string{
				"Getting network interface",
				"Non-retriable error getting network interface",
			},
		},
		{
			name: "Context cancellation",
			networkErrors: []error{
				&AzureError{Code: "ResourceNotFound", Message: "Network interface not found"},
			},
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
				"Getting network interface",
				"Context cancelled while waiting",
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Set up test context
			testCtx, cancel := tc.setupContext()
			defer cancel()

			// Create log observer
			logObserver := logger.NewTestLogObserver()
			logger.SetLogger(logObserver)
			defer logger.SetLogger(l)

			// Mock network interface responses
			currentError := 0
			s.client.networkClient = &mockNetworkClient{
				getFunc: func(ctx context.Context, resourceGroup, name string) (armnetwork.InterfacesClientGetResponse, error) {
					if currentError >= len(tc.networkErrors) {
						s.Fail("Too many network client calls")
						return armnetwork.InterfacesClientGetResponse{}, fmt.Errorf("unexpected call")
					}
					err := tc.networkErrors[currentError]
					currentError++
					if err != nil {
						return armnetwork.InterfacesClientGetResponse{}, err
					}
					return armnetwork.InterfacesClientGetResponse{
						Interface: armnetwork.Interface{
							Properties: &armnetwork.InterfacePropertiesFormat{
								IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
									{
										Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
											PrivateIPAddress: &tc.expectedPrivate,
											PublicIPAddress: &armnetwork.PublicIPAddress{
												ID: &tc.expectedPublic,
											},
										},
									},
								},
							},
						},
					}, nil
				},
			}

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
			s.Equal(tc.expectedRetry, currentError-1)

			// Verify logs
			logs := logObserver.GetLogs()
			for _, expectedLog := range tc.expectedLogs {
				found := false
				for _, log := range logs {
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

// Mock network client for testing
type mockNetworkClient struct {
	getFunc func(ctx context.Context, resourceGroup, name string) (armnetwork.InterfacesClientGetResponse, error)
}

func (m *mockNetworkClient) Get(ctx context.Context, resourceGroup, name string, options *armnetwork.InterfacesClientGetOptions) (armnetwork.InterfacesClientGetResponse, error) {
	return m.getFunc(ctx, resourceGroup, name)
}
