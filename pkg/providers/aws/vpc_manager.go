package aws

import (
	"github.com/bacalhau-project/andaime/pkg/models"
	aws_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
)

// RegionalVPCManager handles VPC operations for a specific region
type RegionalVPCManager struct {
	deployment *models.Deployment
	ec2Client  aws_interfaces.EC2Clienter
}

// NewRegionalVPCManager creates a new VPC manager for a region
func NewRegionalVPCManager(
	deployment *models.Deployment,
	ec2Client aws_interfaces.EC2Clienter,
) *RegionalVPCManager {
	return &RegionalVPCManager{
		deployment: deployment,
		ec2Client:  ec2Client,
	}
}
