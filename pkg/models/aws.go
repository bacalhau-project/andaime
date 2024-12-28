package models

import (
	aws_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
)

// AWSDeployment represents AWS-specific deployment configuration
type AWSDeployment struct {
	DefaultMachineType    string             `json:"default_machine_type,omitempty"`
	DefaultDiskSizeGB     int32              `json:"default_disk_size_gb,omitempty"`
	DefaultCountPerRegion int32              `json:"default_count_per_zone,omitempty"`
	AccountID             string             `json:"account_id,omitempty"`
	RegionalResources     *RegionalResources `json:"regional_resources,omitempty"`
}

// Initialize AWS Deployment
func NewAWSDeployment() *AWSDeployment {
	return &AWSDeployment{
		RegionalResources: &RegionalResources{
			VPCs:    make(map[string]*AWSVPC),
			clients: make(map[string]aws_interfaces.EC2Clienter),
		},
	}
}

// GetRegionalClient returns the EC2 client for the given region
func (d *AWSDeployment) GetRegionalClient(region string) aws_interfaces.EC2Clienter {
	if d.RegionalResources.clients == nil {
		return nil
	}
	client, ok := d.RegionalResources.clients[region]
	if !ok {
		return nil
	}
	return client
}

func (d *AWSDeployment) SetRegionalClient(region string, client aws_interfaces.EC2Clienter) {
	d.RegionalResources.Lock()
	defer d.RegionalResources.Unlock()
	if d.RegionalResources == nil {
		d.RegionalResources = &RegionalResources{}
	}

	// Initialize clients map if it's nil
	if d.RegionalResources.clients == nil {
		d.RegionalResources.clients = make(map[string]aws_interfaces.EC2Clienter)
	}
	d.RegionalResources.clients[region] = client
}

func (d *AWSDeployment) GetAllRegionalClients() map[string]aws_interfaces.EC2Clienter {
	d.RegionalResources.Lock()
	defer d.RegionalResources.Unlock()
	return d.RegionalResources.clients
}

// AWSVPC represents AWS VPC and related resources
type AWSVPC struct {
	VPCID              string   `json:"vpc_id"`
	SecurityGroupID    string   `json:"security_group_id"`
	SubnetID           string   `json:"subnet_id"`
	PublicSubnetIDs    []string `json:"public_subnet_ids,omitempty"`
	PrivateSubnetIDs   []string `json:"private_subnet_ids,omitempty"`
	InternetGatewayID  string   `json:"internet_gateway_id,omitempty"`
	PublicRouteTableID string   `json:"public_route_table_id,omitempty"`
}
