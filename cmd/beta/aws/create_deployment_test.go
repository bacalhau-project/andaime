package aws

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	aws_mock "github.com/bacalhau-project/andaime/mocks/aws"
	sshutils_mock "github.com/bacalhau-project/andaime/mocks/sshutils"
	"github.com/bacalhau-project/andaime/pkg/models"
	aws_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
	sshutils_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	awsprovider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWriteVPCIDToConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	// Create test configuration
	config := []byte(`
deployments:
  test-deployment:
    provider: "aws"
    aws:
      account_id: "test-account-id"
      regions:
        us-west-2:
          vpc_id: ""
          security_group_id: ""
`)

	err := os.WriteFile(configFile, config, 0644)
	require.NoError(t, err)

	// Initialize viper with the test config
	viper.Reset()
	viper.SetConfigFile(configFile)
	err = viper.ReadInConfig()
	require.NoError(t, err)

	// Create test deployment
	deployment := &models.Deployment{
		UniqueID:       "test-deployment",
		DeploymentType: models.DeploymentTypeAWS,
		AWS: &models.AWSDeployment{
			AccountID: "test-account-id",
			RegionalResources: &models.RegionalResources{
				VPCs: map[string]*models.AWSVPC{
					"us-west-2": {
						VPCID:           "vpc-12345",
						SecurityGroupID: "sg-12345",
					},
				},
			},
		},
	}

	// Write deployment to config
	err = deployment.UpdateViperConfig()
	require.NoError(t, err)

	// Verify the configuration was written correctly
	viper.ReadInConfig() // Reload config

	// Check AWS account ID
	assert.Equal(t,
		"test-account-id",
		viper.GetString("deployments.test-deployment.aws.account_id"),
		"AWS account ID should be written to config",
	)

	// Check VPC ID
	assert.Equal(t,
		"vpc-12345",
		viper.GetString("deployments.test-deployment.aws.regions.us-west-2.vpc_id"),
		"VPC ID should be written to config",
	)

	// Check security group ID
	assert.Equal(t,
		"sg-12345",
		viper.GetString("deployments.test-deployment.aws.regions.us-west-2.security_group_id"),
		"Security group ID should be written to config",
	)
}

func TestExecuteCreateDeployment(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	sshPrivateKeyPath := filepath.Join(tempDir, "test-ssh-key")
	sshPublicKeyPath := filepath.Join(tempDir, "test-ssh-key.pub")

	// Create SSH key pair for testing
	privateKey := []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAv8QrHMQxn6M0kF6UQN0dGZrqKGdQJYJ4jqRfxlbAOQzTqEfQ
jQGwON/c+nMoOBpxVqIFWLwM3p4ZwNrZU4LG8kJJcyWwfRGWgEXHtOgYgGQmODIh
Ij5TQDWgYB6CewHR1AO+L+VQqPPf8JHXlJMoUG3ZNwKFbA0YtLlxzwqtj8YGWZ8l
1gHHtlNxF4sGLQXUJQGpxFBJfNQyWHRxKGXlFvJfxVgQKrKrkPkD3GQnnb1YkQGM
qQABXYDX8cXD4SnP8XWbQQTBHqmxNXdfAEhL1JWb9uXXgWV6tF6JIQsdKGn9dcKW
7YZbGbQEyJ/qXEwRs0Ky4dZ+LLu4QgFcuuWxkQIDAQABAoIBADnFJpRQ3DnFwwEq
F5BT6IQoXrKmYYQQqsL86UDgVnGGZYkUBCVQT+qF6HkVGrhxgV3fZGYHd3dTWLdF
3p4Z5CQl4JYhoV5CKqEoQAUVK6GmOtYv6U8DX9EcPUZGqQwGPqL4QxlgHEPs3gHO
K+w6wqZGQcYFGHJc8LwREeXH4RNUQXrTGGYRmXTFvgj+Qr3Cn3Z+CFVFYwZX9Zs/
vJ1MgHHQkMqp7Vf7qYKhGy3ePlXxHGVcxuKDNqiBG0KlERAZxF8yzZHCvS5Kljc8
RxhQ6bvOJPXNj4Nk4EYDsKQ6QgkZYzH6vE3kHn9HXz8AqK5XGvJFJlcn+Vc/IxgC
f4OxrAECgYEA4w5T1vNOjmKKYVxq6Ks5Wk+NSvIWTYkX3KzERjvHwkHj5LuRlHiC
kyIPUcuUWqYF5WkZp5/sVf7rdUxp8UVFPhbl7gMwwqrwDt/zN9+0LnUJGGqNcXXP
YeVRIZY8q5J2aHzYo5bHaMqWDUoW5a6BHwUuLgrLHB7EeeHhuvbhp4ECgYEA2Fq3
RGmZXqDYzF0hRb6pItNv3lTuWIwWwwHh5UQx6dI7YMNI3Vc7IGTqEfnhmk8aSHyq
TQhPV3dqdJrHl6yKOPF1yVx3RHAtWnhQh9hjGJzGGxLN1xGxXJQxzUZfQEJGHn5K
fMvO7RxYIAWUxzOQbedfxUSHU0r25yZjjZvFG5ECgYBNHwMzWX6GpBuZ5SuvZO7F
6YPL9qVZDhH8vhDZD4djmLyCKzZk1zRqBX/0YJ7uUXKh8EQWZ9gWTxXn5PKKbS5y
HaH4+jUEWQUgGcx7zYq7DVl8/XRJ2wJZX4Vz5mJJBPUDqJXvMqxAKXU8c3Lr4DGz
SFhRHwEy0k3Kkp7qQQKBgQCvlZnes0F1t8F+Qhv3eKkH+gQqYH3YGUAVl54h5i8X
kHZVxYhSIqzSb5fRyHTjQqEf8xZnD0PXm8/DBnYqh5m9rRQNwbV8OqLt4NRgxYj5
E0/jddcV7oKEYd/tKZ7kBx5HvN5zWqr8I4SJhWkE8ZgGGnVuRKA8yEZvfHzugQKB
gBM4YBnWzQgB2LCz5B8Bv4+qEBOJHQKvLM0EcFQeQQYqL83GrGTGoYh5LKuHfHfE
s5QJjVQYXr0Uw1oJ5GF4dE1XVxZWUcIxh7z0H5IZY5xrWVDzqHwShQqwZ1cQ/pQx
JRHnTGHIxKWYGR0O5mQGvDGwIdNyd5UZHdJhyXj5sg==
-----END RSA PRIVATE KEY-----`)
	publicKey := []byte(`ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC/xCscxDGfozSQXpRA3R0ZmuooZ1AlgniOpF/GVsA5DNOoR9CNAbA439z6cyg4GnFWogVYvAzenhHA2tlTgsbyQklzJbB9EZaARce06BiAZCY4MiEiPlNANaBgHoJ7AdHUA74v5VCo89/wkdeUkyhQbdk3AoVsDRi0uXHPCq2PxgZZnyXWAce2U3EXiwYtBdQlAanEUEl81DJYdHEoZeUW8l/FWBAqsquQ+QPcZCedvViRAYypAAFdgNfxxcPhKc/xdZtBBMEeqbE1d18ASEvUlZv25deBZXq0XokhCx0oaf11wpbthlsZtATIn+pcTBGzQrLh1n4su7hCAVy65bGR test@andaime`)

	// Write SSH key files
	err := os.WriteFile(sshPrivateKeyPath, privateKey, 0600)
	require.NoError(t, err)
	err = os.WriteFile(sshPublicKeyPath, publicKey, 0644)
	require.NoError(t, err)

	config := []byte(`
general:
  ssh_private_key_path: "` + sshPrivateKeyPath + `"
  ssh_public_key_path: "` + sshPublicKeyPath + `"
  ssh_port: 22
aws:
  account_id: "123456789012"
  default_count_per_zone: 1
  default_machine_type: "t3.medium"
  default_disk_size_gb: 30
deployments:
  test-deployment:
    provider: "aws"
    aws:
      machines:
        - location: "us-east-1a"
          parameters:
            count: 1
            orchestrator: true
      regions:
        us-east-1:
          vpc_id: ""
          security_group_id: ""
`)
	err = os.WriteFile(configFile, config, 0644)
	require.NoError(t, err)

	// Initialize viper with the test config
	viper.Reset()
	viper.SetConfigFile(configFile)
	err = viper.ReadInConfig()
	require.NoError(t, err)

	// Set required viper keys
	viper.Set("general.log_level", "debug")
	viper.Set("general.log_path", "/tmp/andaime-test.log")

	// Create mock EC2 client
	mockEC2Client := new(aws_mock.MockEC2Clienter)

	// Mock DescribeAvailabilityZones response
	mockEC2Client.On("DescribeAvailabilityZones", mock.Anything, &ec2.DescribeAvailabilityZonesInput{}).
		Return(
			&ec2.DescribeAvailabilityZonesOutput{
				AvailabilityZones: []types.AvailabilityZone{
					{
						ZoneName:   aws.String("us-east-1a"),
						ZoneType:   aws.String("availability-zone"),
						RegionName: aws.String("us-east-1"),
						State:      types.AvailabilityZoneStateAvailable,
					},
					{
						ZoneName:   aws.String("us-east-1b"),
						ZoneType:   aws.String("availability-zone"),
						RegionName: aws.String("us-east-1"),
						State:      types.AvailabilityZoneStateAvailable,
					},
					{
						ZoneName:   aws.String("us-east-1c"),
						ZoneType:   aws.String("availability-zone"),
						RegionName: aws.String("us-east-1"),
						State:      types.AvailabilityZoneStateAvailable,
					},
				},
			}, nil)

	// Mock VPC creation
	mockEC2Client.On("CreateVpc", mock.Anything, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
	}).Return(&ec2.CreateVpcOutput{
		Vpc: &types.Vpc{
			VpcId: aws.String("vpc-12345"),
		},
	}, nil)

	mockEC2Client.On("ModifyVpcAttribute", mock.Anything, mock.Anything).
		Return(&ec2.ModifyVpcAttributeOutput{}, nil)

	mockEC2Client.On("CreateSecurityGroup", mock.Anything, &ec2.CreateSecurityGroupInput{
		Description: aws.String("Security group for Bacalhau deployment"),
		GroupName:   aws.String("bacalhau-sg"),
		VpcId:      aws.String("vpc-12345"),
	}).Return(&ec2.CreateSecurityGroupOutput{
		GroupId: aws.String("sg-12345"),
	}, nil)

	mockEC2Client.On("AuthorizeSecurityGroupIngress", mock.Anything, mock.Anything).
		Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil)

	// Mock instance creation
	mockEC2Client.On("RunInstances", mock.Anything, mock.Anything).
		Return(&ec2.RunInstancesOutput{
			Instances: []types.Instance{
				{
					InstanceId: aws.String("i-12345"),
					State: &types.InstanceState{
						Name: types.InstanceStateNameRunning,
					},
					PublicIpAddress: aws.String("1.2.3.4"),
				},
			},
		}, nil)

	mockEC2Client.On("DescribeInstances", mock.Anything, mock.Anything).
		Return(&ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{
				{
					Instances: []types.Instance{
						{
							InstanceId: aws.String("i-12345"),
							State: &types.InstanceState{
								Name: types.InstanceStateNameRunning,
							},
							PublicIpAddress: aws.String("1.2.3.4"),
						},
					},
				},
			},
		}, nil)

	// Create SSH config mock
	mockSSHConfig := new(sshutils_mock.MockSSHConfiger)

	// Set up expectations for SSH operations
	mockSSHConfig.On("Connect").Return(nil)
	mockSSHConfig.On("RunCommand", mock.Anything).Return([]byte("success"), nil)
	mockSSHConfig.On("Close").Return(nil)

	// Override SSH config factory
	originalSSHConfigFunc := sshutils.NewSSHConfigFunc
	defer func() { sshutils.NewSSHConfigFunc = originalSSHConfigFunc }()
	sshutils.NewSSHConfigFunc = func(host string, port int, username string, privateKeyPath string) (sshutils_interfaces.SSHConfiger, error) {
		return mockSSHConfig, nil
	}

	// Initialize display model
	deployment, err := models.NewDeployment()
	require.NoError(t, err)
	deployment.AWS = &models.AWSDeployment{
		AccountID: "123456789012",
		RegionalResources: &models.RegionalResources{
			VPCs:    make(map[string]*models.AWSVPC),
			Clients: make(map[string]aws_interface.EC2Clienter),
		},
	}
	deployment.AWS.RegionalResources.Clients["us-east-1"] = mockEC2Client

	// Set up display model
	displayModel := &display.DisplayModel{
		Deployment: deployment,
	}
	originalGetModelFunc := display.GetGlobalModelFunc
	defer func() { display.GetGlobalModelFunc = originalGetModelFunc }()
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return displayModel
	}

	// Override AWS provider factory
	originalProviderFunc := awsprovider.NewAWSProviderFunc
	defer func() { awsprovider.NewAWSProviderFunc = originalProviderFunc }()
	awsprovider.NewAWSProviderFunc = func(accountID string) (*awsprovider.AWSProvider, error) {
		provider, err := originalProviderFunc(accountID)
		if err != nil {
			return nil, err
		}
		provider.EC2Client = mockEC2Client
		return provider, nil
	}

	// Create cobra command with config flag
	cmd := &cobra.Command{}
	cmd.Flags().String("config", configFile, "Path to the configuration file")

	// Execute deployment
	err = ExecuteCreateDeployment(cmd, nil)
	require.NoError(t, err)

	// Verify VPC and security group IDs in config
	assert.Equal(t,
		"vpc-12345",
		viper.GetString("deployments.test-deployment.aws.regions.us-east-1.vpc_id"),
		"VPC ID should be written to config",
	)
	assert.Equal(t,
		"sg-12345",
		viper.GetString("deployments.test-deployment.aws.regions.us-east-1.security_group_id"),
		"Security group ID should be written to config",
	)
}
