package azure

import (
	"context"
	"testing"

	"github.com/bacalhau-project/andaime/pkg/display"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/suite"
)

type PkgProvidersAzureTestSuite struct {
	suite.Suite
	ctx                    context.Context
	testPrivateKeyPath     string
	cleanup                func()
	originalGetGlobalModel func() *display.DisplayModel
}

func TestAzure(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pkg Azure Suite")
}

var _ = BeforeSuite(func() {
	// Any additional setup specific to this suite
})

var _ = AfterSuite(func() {
	// Any additional teardown specific to this suite
})
