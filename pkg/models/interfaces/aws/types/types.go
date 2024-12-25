// Package types provides AWS-specific type definitions
package types

import (
	"context"
)

// Common AWS types and interfaces
type AWSError interface {
	Error() string
}

type AWSOutput interface {
	ResultMetadata() interface{}
}

type AWSInput interface {
	Validate() error
}

// EC2Client represents the core EC2 client functionality
type EC2Client interface {
	GetClient() interface{}
	GetIMDSConfig() IMDSClient
}

// STSClient represents the core STS client functionality
type STSClient interface {
	GetClient() interface{}
}

// STSOperations defines the interface for STS operations
type STSOperations interface {
	GetAccountID(ctx context.Context) (string, error)
	GetCallerIdentity(ctx context.Context) (*STSIdentity, error)
}

// STSIdentity represents AWS STS identity information
type STSIdentity struct {
	Account string
	Arn     string
	UserID  string
}

// EC2Instance represents an EC2 instance
type EC2Instance struct {
	ID            string
	PublicIP      string
	PrivateIP     string
	State         string
	SpotRequestID string
}

// EC2SecurityGroup represents an EC2 security group
type EC2SecurityGroup struct {
	ID    string
	Name  string
	VPCID string
}

// EC2VPC represents an EC2 VPC
type EC2VPC struct {
	ID   string
	CIDR string
}

// EC2Subnet represents an EC2 subnet
type EC2Subnet struct {
	ID    string
	VPCID string
	CIDR  string
	Zone  string
}

// EC2Image represents an EC2 AMI
type EC2Image struct {
	ID           string
	CreationDate string
	Name         string
}

// SpotInstanceRequest represents a spot instance request
type SpotInstanceRequest struct {
	ID           string
	State        string
	InstanceID   string
	FaultMessage string
	MaxPrice     string
	InstanceType string
	ImageID      string
}

// EC2Operations defines the interface for EC2 operations
type EC2Operations interface {
	GetIMDSConfig() IMDSClient
	CreateSpotInstance(ctx context.Context, input *SpotInstanceRequest) (*EC2Instance, error)
	CreateOnDemandInstance(ctx context.Context, input *EC2Instance) (*EC2Instance, error)
	DescribeInstance(ctx context.Context, instanceID string) (*EC2Instance, error)
	TerminateInstance(ctx context.Context, instanceID string) error
}

// VPCOperations defines the interface for VPC operations
type VPCOperations interface {
	CreateVPC(ctx context.Context, cidrBlock string) (*EC2VPC, error)
	DeleteVPC(ctx context.Context, vpcID string) error
}

// NetworkOperations defines the interface for network operations
type NetworkOperations interface {
	CreateSubnet(ctx context.Context, vpcID, cidrBlock, zone string) (*EC2Subnet, error)
	CreateSecurityGroup(ctx context.Context, vpcID, name, description string) (*EC2SecurityGroup, error)
}

// GatewayOperations defines the interface for gateway operations
type GatewayOperations interface {
	CreateInternetGateway(ctx context.Context) (*InternetGateway, error)
	AttachInternetGateway(ctx context.Context, vpcID, gatewayID string) error
}

// RouteOperations defines the interface for route operations
type RouteOperations interface {
	CreateRouteTable(ctx context.Context, vpcID string) (*RouteTable, error)
	CreateRoute(ctx context.Context, routeTableID, destinationCIDR, gatewayID string) error
}

// RegionOperations defines the interface for region operations
type RegionOperations interface {
	GetRegion() string
	SetRegion(region string)
}

// IMDSClient represents an EC2 Instance Metadata Service client
type IMDSClient interface {
	GetInstanceIdentityDocument() (*InstanceIdentityDocument, error)
}

// InstanceIdentityDocument represents EC2 instance identity information
type InstanceIdentityDocument struct {
	AccountID    string
	Region       string
	InstanceID   string
	InstanceType string
}

// InternetGateway represents an AWS internet gateway
type InternetGateway struct {
	ID string
}

// RouteTable represents an AWS route table
type RouteTable struct {
	ID    string
	VPCID string
}

// AWSOperations combines all AWS operations interfaces
type AWSOperations interface {
	EC2Operations
	VPCOperations
	NetworkOperations
	GatewayOperations
	RouteOperations
	RegionOperations
}

// RunInstancesAPI defines the interface for running EC2 instances
type RunInstancesAPI interface {
	EC2Operations
}

// DescribeInstancesAPI defines the interface for describing EC2 instances
type DescribeInstancesAPI interface {
	EC2Operations
}

// TerminateInstancesAPI defines the interface for terminating EC2 instances
type TerminateInstancesAPI interface {
	EC2Operations
}

// ImageAPI defines the interface for EC2 image operations
type ImageAPI interface {
	EC2Operations
}

// VPCAPI defines the interface for VPC operations
type VPCAPI interface {
	VPCOperations
}

// SubnetAPI defines the interface for subnet operations
type SubnetAPI interface {
	NetworkOperations
}

// SecurityGroupAPI defines the interface for security group operations
type SecurityGroupAPI interface {
	NetworkOperations
}

// InternetGatewayAPI defines the interface for internet gateway operations
type InternetGatewayAPI interface {
	GatewayOperations
}

// RouteTableAPI defines the interface for route table operations
type RouteTableAPI interface {
	RouteOperations
}

// RegionAPI defines the interface for region operations
type RegionAPI interface {
	RegionOperations
}

// EC2ContextOperations combines all context-dependent EC2 operations
type EC2ContextOperations interface {
	RunInstancesAPI
	DescribeInstancesAPI
	TerminateInstancesAPI
	ImageAPI
	VPCAPI
	SubnetAPI
	SecurityGroupAPI
	InternetGatewayAPI
	RouteTableAPI
	RegionAPI
}
