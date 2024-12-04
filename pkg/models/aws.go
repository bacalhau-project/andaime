package models

import (
	"sync"
)

// AWSDeployment represents AWS-specific deployment configuration
type AWSDeployment struct {
	DefaultMachineType    string             `json:"default_machine_type,omitempty"`
	DefaultDiskSizeGB     int32              `json:"default_disk_size_gb,omitempty"`
	DefaultCountPerRegion int32              `json:"default_count_per_zone,omitempty"`
	AccountID             string             `json:"account_id,omitempty"`
	RegionalResources     *RegionalResources `json:"regional_resources,omitempty"`
	mu                    sync.RWMutex
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
