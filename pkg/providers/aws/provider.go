package awsprovider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsinterfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/spf13/viper"
)

// ConfigInterface defines the interface for configuration operations
type ConfigInterfacer interface {
	GetString(key string) string
}

type AWSProvider struct {
	Config    *aws.Config
	EC2Client EC2Clienter
	Region    string
}

var ubuntuAMICache = make(map[string]string)
var cacheLock sync.RWMutex

// NewAWSProvider creates a new AWSProvider instance
func NewAWSProvider(v *viper.Viper) (awsinterfaces.AWSProviderer, error) {
	ctx := context.Background()
	region := v.GetString("aws.region")
	awsConfig, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}
	ec2Client := ec2.NewFromConfig(awsConfig)

	awsProvider := &AWSProvider{
		Config:    &awsConfig,
		EC2Client: ec2Client,
		Region:    region,
	}

	return awsProvider, nil
}

// ConfigWrapper wraps the AWS config to implement ConfigInterface
type ConfigWrapper struct {
	config aws.Config
}

func NewConfigWrapper(config aws.Config) *ConfigWrapper {
	return &ConfigWrapper{config: config}
}

func (cw *ConfigWrapper) GetString(key string) string {
	// Implement this method based on how you're storing/retrieving config values
	// This is just a placeholder
	return ""
}

// GetEC2Client returns the current EC2 client
func (p *AWSProvider) GetEC2Client() (EC2Clienter, error) {
	return p.EC2Client, nil
}

// SetEC2Client sets a new EC2 client
func (p *AWSProvider) SetEC2Client(client EC2Clienter) {
	p.EC2Client = client
}

// CreateDeployment performs the AWS deployment
func (p *AWSProvider) CreateDeployment(ctx context.Context, instanceType InstanceType) error {
	l := logger.Get()

	image, err := p.GetLatestUbuntuImage(ctx, p.Region)
	if err != nil {
		return fmt.Errorf("failed to get latest Ubuntu image: %w", err)
	}

	l.Infof("Latest Ubuntu AMI ID for region %s: %s\n", p.Region, *image.ImageId)

	var runInstancesInput *ec2.RunInstancesInput

	switch instanceType {
	case EC2Instance:
		runInstancesInput = p.createEC2InstanceInput(image.ImageId)
	case SpotInstance:
		runInstancesInput = p.createSpotInstanceInput(image.ImageId)
	default:
		return fmt.Errorf("invalid instance type: %s", instanceType)
	}

	result, err := p.EC2Client.RunInstances(ctx, runInstancesInput)
	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	l.Infof("Created instance: %s\n", *result.Instances[0].InstanceId)
	return nil
}

func (p *AWSProvider) validateRegion(region string) error {
	if region == "" {
		return fmt.Errorf("AWS region is not specified in the configuration")
	}
	return nil
}

func (p *AWSProvider) CreateDeployment(ctx context.Context, instanceType InstanceType) error {
	l := logger.Get()

	image, err := p.GetLatestUbuntuImage(ctx, p.Region)
	if err != nil {
		return fmt.Errorf("failed to get latest Ubuntu image: %w", err)
	}

	l.Infof("Latest Ubuntu AMI ID for region %s: %s\n", p.Region, *image.ImageId)

	var runInstancesInput *ec2.RunInstancesInput

	switch instanceType {
	case EC2Instance:
		runInstancesInput = p.createEC2InstanceInput(image.ImageId)
	case SpotInstance:
		runInstancesInput = p.createSpotInstanceInput(image.ImageId)
	default:
		return fmt.Errorf("invalid instance type: %s", instanceType)
	}

	result, err := p.EC2Client.RunInstances(ctx, runInstancesInput)
	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	l.Infof("Created instance: %s\n", *result.Instances[0].InstanceId)
	return nil
}

func (p *AWSProvider) createEC2InstanceInput(imageId *string) *ec2.RunInstancesInput {
	return &ec2.RunInstancesInput{
		ImageId:      imageId,
		InstanceType: types.InstanceTypeT3Micro,
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
	}
}

func (p *AWSProvider) createSpotInstanceInput(imageId *string) *ec2.RunInstancesInput {
	return &ec2.RunInstancesInput{
		ImageId:      imageId,
		InstanceType: types.InstanceTypeT3Micro,
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		InstanceMarketOptions: &types.InstanceMarketOptionsRequest{
			MarketType: types.MarketTypeSpot,
			SpotOptions: &types.SpotMarketOptions{
				MaxPrice: aws.String("0.05"), // Set your maximum spot price
			},
		},
	}
}

func (p *AWSProvider) describeInstances(ctx context.Context) ([]*types.Instance, error) {
	input := &ec2.DescribeInstancesInput{}
	result, err := p.EC2Client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var instances []*types.Instance
	for _, reservation := range result.Reservations {
		for i := range reservation.Instances {
			instances = append(instances, &reservation.Instances[i])
		}
	}

	return instances, nil
}

func (p *AWSProvider) ListDeployments(ctx context.Context) ([]*types.Instance, error) {
	instances, err := p.describeInstances(ctx)
	if err != nil {
		return nil, err
	}

	// Add any additional filtering or processing of instances here if needed

	return instances, nil
}

func (p *AWSProvider) terminateInstances(ctx context.Context, instanceIDs []string) error {
	input := &ec2.TerminateInstancesInput{
		InstanceIds: instanceIDs,
	}
	_, err := p.EC2Client.TerminateInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to terminate instances: %w", err)
	}
	return nil
}

func (p *AWSProvider) TerminateDeployment(ctx context.Context) error {
	instances, err := p.describeInstances(ctx)
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	var instanceIDs []string
	for _, instance := range instances {
		instanceIDs = append(instanceIDs, *instance.InstanceId)
	}

	if len(instanceIDs) > 0 {
		err = p.terminateInstances(ctx, instanceIDs)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetLatestUbuntuImage gets the latest Ubuntu AMI for the specified region
func (p *AWSProvider) GetLatestUbuntuImage(
	ctx context.Context,
	region string,
) (*types.Image, error) {
	cacheLock.RLock()
	cachedAMI, found := ubuntuAMICache[p.Region]
	cacheLock.RUnlock()

	if found {
		return &types.Image{ImageId: aws.String(cachedAMI)}, nil
	}

	input := &ec2.DescribeImagesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{"ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"},
			},
			{
				Name:   aws.String("architecture"),
				Values: []string{"x86_64"},
			},
			{
				Name:   aws.String("root-device-type"),
				Values: []string{"ebs"},
			},
			{
				Name:   aws.String("virtualization-type"),
				Values: []string{"hvm"},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
			{
				Name:   aws.String("owner-id"),
				Values: []string{"099720109477"}, // Canonical's AWS account ID
			},
		},
	}

	result, err := p.EC2Client.DescribeImages(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe images: %w", err)
	}

	if len(result.Images) == 0 {
		return nil, fmt.Errorf("no Ubuntu images found")
	}

	var latestImage *types.Image
	var latestTime time.Time

	for _, image := range result.Images {
		internalImage := image
		creationTime, err := time.Parse(time.RFC3339, *image.CreationDate)
		if err != nil {
			continue
		}

		if latestImage == nil || creationTime.After(latestTime) {
			latestImage = &internalImage
			latestTime = creationTime
		}
	}

	if latestImage == nil {
		return nil, fmt.Errorf("failed to find the latest Ubuntu image")
	}

	cacheLock.Lock()
	ubuntuAMICache[region] = *latestImage.ImageId
	cacheLock.Unlock()

	return latestImage, nil
}
