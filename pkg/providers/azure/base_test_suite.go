package azure

import (
	"context"
	"os"
	"sync"

	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	pkg_testutil "github.com/bacalhau-project/andaime/pkg/testutil"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
)

var cleanupFunctions []func()
var cleanupMutex sync.Mutex

type BaseAzureTestSuite struct {
	suite.Suite
	MockAzureClient *azure_mocks.MockAzureClienter
	Ctx             context.Context
	Deployment      *models.Deployment
}

func (suite *BaseAzureTestSuite) SetupSuite() {
	tempConfigFile, err := os.CreateTemp("", "azure_test_config_*.yaml")
	suite.Require().NoError(err, "Failed to create temporary config file")

	cleanupFunctions = append(cleanupFunctions, func() {
		_ = os.Remove(tempConfigFile.Name())
	})

	// Copy the content of the internal/testdata/config/gcp.yaml file to the temporary file
	content, err := testdata.ReadTestGCPConfig()
	suite.Require().NoError(err)
	err = os.WriteFile(tempConfigFile.Name(), []byte(content), 0o600) //nolint:mnd
	suite.Require().NoError(err)

	viper.SetConfigFile(tempConfigFile.Name())

	testSSHPublicKeyPath,
		testCleanupPublicKey,
		testSSHPrivateKeyPath,
		testCleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()

	cleanupMutex.Lock()
	cleanupFunctions = append(cleanupFunctions, testCleanupPublicKey, testCleanupPrivateKey)
	cleanupMutex.Unlock()

	viper.Set("general.project_prefix", "test-project")
	viper.Set("general.unique_id", "test-unique-id")
	viper.Set("general.ssh_private_key_path", testSSHPrivateKeyPath)
	viper.Set("general.ssh_public_key_path", testSSHPublicKeyPath)
	viper.Set("azure.subscription_id", "test-subscription-id")

	pkg_testutil.SetupViper(models.DeploymentTypeAzure, testSSHPublicKeyPath, testSSHPrivateKeyPath)

	m := display.GetGlobalModelFunc()
	dep, err := common.PrepareDeployment(suite.Ctx, models.DeploymentTypeAzure)
	suite.Require().NoError(err)
	suite.Deployment = dep
	m.Deployment = suite.Deployment
}

func (suite *BaseAzureTestSuite) TearDownSuite() {
	cleanupMutex.Lock()
	defer cleanupMutex.Unlock()
	for _, cleanup := range cleanupFunctions {
		cleanup()
	}
	cleanupFunctions = []func(){}
}

func (suite *BaseAzureTestSuite) SetupTest() {
	suite.Ctx = context.Background()
	suite.MockAzureClient = (*azure_mocks.MockAzureClienter)(NewMockClient())

	m := display.GetGlobalModelFunc()
	m.Deployment = suite.Deployment
}
