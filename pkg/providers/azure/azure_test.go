package azure

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type AzureTestSuite struct {
	BaseAzureTestSuite
}

func TestAzure(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Azure Suite")
}

var _ = BeforeSuite(func() {
	// Any additional setup specific to this suite
})

var _ = AfterSuite(func() {
	// Any additional teardown specific to this suite
})
