package gcp

import (
	"context"
	"testing"
	"time"

	gcp_mocks "github.com/bacalhau-project/andaime/mocks/gcp"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	gcp_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/gcp"
	gcp_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/gcp"
	gcp_provider "github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/stretchr/testify/suite"
)

type GCPProgressTestSuite struct {
	suite.Suite
	ctx                    context.Context
	mockGCPClient          *gcp_mocks.MockGCPClienter
	gcpProvider            *gcp_provider.GCPProvider
	origGetGlobalModelFunc func() *display.DisplayModel
	origGetClientFunc      func(ctx context.Context, organizationID string) (gcp_interfaces.GCPClienter, func(), error)
}

func (suite *GCPProgressTestSuite) SetupTest() {
	suite.ctx = context.Background()
	suite.mockGCPClient = new(gcp_mocks.MockGCPClienter)

	// Store original functions
	suite.origGetGlobalModelFunc = display.GetGlobalModelFunc
	suite.origGetClientFunc = gcp_provider.NewGCPClientFunc

	// Setup test deployment
	deployment, err := models.NewDeployment()
	suite.Require().NoError(err)
	deployment.DeploymentType = models.DeploymentTypeGCP

	// Create test machine
	machine := &models.Machine{
		ID:            "test-machine",
		Name:          "test-machine",
		Region:        "us-central1",
		Zone:          "us-central1-a",
		VMSize:        "n1-standard-2",
		SSHPort:       22,
		SSHUser:       "test-user",
		StartTime:     time.Now(),
		PublicIP:      "1.2.3.4",
		PrivateIP:     "10.0.0.2",
		CloudProvider: models.DeploymentTypeGCP,
	}
	deployment.Machines = map[string]models.Machiner{
		machine.Name: machine,
	}

	// Setup global model
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return &display.DisplayModel{
			Deployment: deployment,
		}
	}

	// Setup mock client
	gcp_provider.NewGCPClientFunc = func(ctx context.Context, organizationID string) (gcp_interface.GCPClienter, func(), error) {
		return suite.mockGCPClient, func() {}, nil
	}

	// Create provider
	var err2 error
	suite.gcpProvider, err2 = gcp_provider.NewGCPProviderFunc(suite.ctx, "test-org", "test-billing")
	suite.Require().NoError(err2)
	suite.gcpProvider.SetGCPClient(suite.mockGCPClient)
}

func (suite *GCPProgressTestSuite) TearDownTest() {
	display.GetGlobalModelFunc = suite.origGetGlobalModelFunc
	gcp_provider.NewGCPClientFunc = suite.origGetClientFunc
}

func (suite *GCPProgressTestSuite) TestProgressBarAndServiceCompletion() {
	m := display.GetGlobalModelFunc()
	machine := m.Deployment.GetMachine("test-machine")
	suite.Require().NotNil(machine)

	// Initialize RequiredGCPResources with all resource types we want to track
	requiredResources := []models.ResourceType{
		{
			ResourceString:    "cloudresourcemanager.googleapis.com/Project",
			ShortResourceName: "Project",
		},
		{ResourceString: "compute.googleapis.com/Network", ShortResourceName: "VPC"},
		{ResourceString: "compute.googleapis.com/Firewall", ShortResourceName: "Firewall"},
		{ResourceString: "compute.googleapis.com/Instance", ShortResourceName: "Instance"},
		{ResourceString: "compute.googleapis.com/Disk", ShortResourceName: "Disk"},
		{ResourceString: "iam.googleapis.com/ServiceAccount", ShortResourceName: "ServiceAccount"},
		{
			ResourceString:    "cloudresourcemanager.googleapis.com/IAMPolicy",
			ShortResourceName: "IAMPolicy",
		},
	}
	models.RequiredGCPResources = requiredResources

	// Initialize resource states
	for _, resource := range requiredResources {
		machine.SetMachineResourceState(resource.ResourceString, models.ResourceStateNotStarted)
	}

	// Test initial state
	progress, total := machine.ResourcesComplete()
	suite.Equal(0, progress)
	suite.Equal(len(models.RequiredGCPResources), total)

	// Simulate resource updates
	resourceStates := map[string]models.MachineResourceState{
		"cloudresourcemanager.googleapis.com/Project":   models.ResourceStateSucceeded,
		"compute.googleapis.com/Network":                models.ResourceStateSucceeded,
		"compute.googleapis.com/Firewall":               models.ResourceStateSucceeded,
		"compute.googleapis.com/Instance":               models.ResourceStateSucceeded,
		"compute.googleapis.com/Disk":                   models.ResourceStateSucceeded,
		"iam.googleapis.com/ServiceAccount":             models.ResourceStateSucceeded,
		"cloudresourcemanager.googleapis.com/IAMPolicy": models.ResourceStateSucceeded,
	}

	// Update each resource and verify progress
	for resourceType, state := range resourceStates {
		machine.SetMachineResourceState(resourceType, state)
		progress, total = machine.ResourcesComplete()
		suite.Greater(progress, 0)
		suite.Equal(len(models.RequiredGCPResources), total)
	}

	// Verify final progress
	progress, total = machine.ResourcesComplete()
	suite.Equal(total, progress)

	// Test service states
	serviceStates := map[string]models.ServiceState{
		models.ServiceTypeSSH.Name:          models.ServiceStateSucceeded,
		models.ServiceTypeCorePackages.Name: models.ServiceStateSucceeded,
		models.ServiceTypeDocker.Name:       models.ServiceStateSucceeded,
		models.ServiceTypeBacalhau.Name:     models.ServiceStateSucceeded,
		models.ServiceTypeScript.Name:       models.ServiceStateSucceeded,
	}

	// Update each service and verify state
	for service, state := range serviceStates {
		machine.SetServiceState(service, state)
		suite.Equal(state, machine.GetServiceState(service))
	}

	// Verify machine completion
	suite.True(machine.IsComplete())
}

func TestGCPProgressTestSuite(t *testing.T) {
	suite.Run(t, new(GCPProgressTestSuite))
}
