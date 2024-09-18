package azure

import (
	"context"
	"sync"

	"github.com/bacalhau-project/andaime/internal/testutil"
	azure_mocks "github.com/bacalhau-project/andaime/mocks/azure"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/interfaces"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
)

var cleanupFunctions []func()
var cleanupMutex sync.Mutex

type BaseAzureTestSuite struct {
	suite.Suite
	MockAzureClient    *azure_mocks.MockAzureClienter
	MockResourceGroups []interfaces.ResourceGroup
	Ctx                context.Context
	TestDisplayModel   *display.DisplayModel
}

func (suite *BaseAzureTestSuite) SetupSuite() {
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
	suite.MockAzureClient = nil
	suite.MockResourceGroups = []interfaces.ResourceGroup{}
}
