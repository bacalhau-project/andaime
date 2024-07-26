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
	"github.com/spf13/viper"
)

// ConfigInterface defines the interface for configuration operations
type ConfigInterfacer interface {
	GetString(key string) string
}

// AWSProvider wraps the AWS deployment functionality
type AWSProvider struct {
	Config    *aws.Config
	EC2Client EC2Clienter
}

var ubuntuAMICache = make(map[string]string)
var cacheLock sync.RWMutex

// NewAWSProvider creates a new AWSProvider instance
func NewAWSProvider(viper *viper.Viper) (*AWSProvider, error) {
	ctx := context.Background()
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}
	ec2Client, err := NewEC2Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create EC2 client: %w", err)
	}

	return &AWSProvider{
		Config:    &awsConfig,
		EC2Client: ec2Client,
	}, nil
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

// GetConfig returns the current configuration
func (p *AWSProvider) GetConfig() *aws.Config {
	return p.Config
}

// SetConfig sets a new configuration
func (p *AWSProvider) SetConfig(config *aws.Config) {
	p.Config = config
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
func (p *AWSProvider) getRegion() (string, error) {
	region := p.Config.Region
	if err := p.validateRegion(region); err != nil {
		return "", err
	}
	return region, nil
}

func (p *AWSProvider) validateRegion(region string) error {
	if region == "" {
		return fmt.Errorf("AWS region is not specified in the configuration")
	}
	return nil
}

func (p *AWSProvider) CreateDeployment(ctx context.Context) error {
	region, err := p.getRegion()
	if err != nil {
		return fmt.Errorf("failed to get region: %w", err)
	}

	image, err := p.GetLatestUbuntuImage(ctx, region)
	if err != nil {
		return fmt.Errorf("failed to get latest Ubuntu image: %w", err)
	}

	fmt.Printf("Latest Ubuntu AMI ID for region %s: %s\n", region, *image.ImageId)
	return nil
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

func (p *AWSProvider) DestroyDeployment(ctx context.Context) error {
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
func (p *AWSProvider) GetLatestUbuntuImage(ctx context.Context, region string) (*types.Image, error) {
	if err := p.validateRegion(region); err != nil {
		return nil, err
	}

	cacheLock.RLock()
	cachedAMI, found := ubuntuAMICache[region]
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
		creationTime, err := time.Parse(time.RFC3339, *image.CreationDate)
		if err != nil {
			continue
		}

		if latestImage == nil || creationTime.After(latestTime) {
			latestImage = &image
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
