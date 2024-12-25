package aws

import (
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/bacalhau-project/andaime/pkg/models/interfaces/aws/types"
)

// EC2ClientWrapper wraps ec2.Client to implement EC2Clienter interface
type EC2ClientWrapper struct {
	*ec2.Client
	imdsClient *imds.Client
}

// NewEC2ClientWrapper creates a new EC2ClientWrapper
func NewEC2ClientWrapper(client *ec2.Client, imdsClient *imds.Client) types.EC2Clienter {
	return &EC2ClientWrapper{
		Client:     client,
		imdsClient: imdsClient,
	}
}

// GetIMDSConfig implements EC2Clienter interface
func (w *EC2ClientWrapper) GetIMDSConfig() *imds.Client {
	return w.imdsClient
}
