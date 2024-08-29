package cmd

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

var VersionNumber string = "v0.0.1-alpha"

const (
	DefaultNumberOfOrchestratorNodes = 1
	DefaultNumberOfComputeNodes      = 2
	RetryTimeout                     = 2 * time.Second
)

//go:embed startup_scripts/*
var startupScriptsFS embed.FS

// Struct to hold instance information
type InstanceInfo struct {
	InstanceID string
	Region     string
	PublicIP   string
}

// Struct to hold template data
type TemplateData struct {
	ProjectName               string
	TargetPlatform            string
	NumberOfOrchestratorNodes int
	NumberOfComputeNodes      int
	TargetRegions             string
	AwsProfile                string
	OrchestratorIPs           string
	NodeType                  string
}

var (
	VerboseModeFlag bool = false
	ProjectSettings      = map[string]interface{}{
		"ProjectName":               "bacalhau-by-andaime",
		"TargetPlatform":            "aws",
		"NumberOfOrchestratorNodes": 1, //nolint:mnd
		"NumberOfComputeNodes":      2, //nolint:mnd
	}

	SetBy = map[string]string{
		"ProjectName":               "default",
		"TargetPlatform":            "default",
		"NumberOfOrchestratorNodes": "default",
		"NumberOfComputeNodes":      "default",
	}
	ProjectNameFlag               string
	TargetPlatformFlag            string
	NumberOfOrchestratorNodesFlag int
	NumberOfComputeNodesFlag      int
	TargetRegionsFlag             string
	OrchestratorIPFlag            string
	command                       string
	helpFlag                      bool //nolint:unused
	AWSProfileFlag                string
	InstanceTypeFlag              string
	ComputeInstanceTypeFlag       string
	OrchestratorInstanceTypeFlag  string
	ValidArchitectures            = []string{"arm64", "x86_64"}
	BootVolumeSizeFlag            int
	SessionGuidanceLogged         = false
)

func GetSession(region string) *session.Session {
	var sess *session.Session
	var err error

	awsAccessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

	if AWSProfileFlag != "" {
		if VerboseModeFlag && !SessionGuidanceLogged {
			SessionGuidanceLogged = true
			fmt.Printf("\tUsing -aws-profile flag \"%s\"\n\n", AWSProfileFlag)
		}

		sess, err = session.NewSessionWithOptions(session.Options{
			Profile: AWSProfileFlag,
			Config:  aws.Config{Region: aws.String(region)},
		})

		if err != nil {
			fmt.Printf("Error creating session for region %s: %v\n", region, err)
			os.Exit(1)
		}

		return sess
	}

	if awsAccessKeyID != "" && awsSecretAccessKey != "" {
		if VerboseModeFlag && !SessionGuidanceLogged {
			SessionGuidanceLogged = true
			fmt.Println(
				"\tUsing environment variables AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY",
			)
		}

		sess, err = session.NewSession(&aws.Config{
			Region:      aws.String(region),
			Credentials: credentials.NewStaticCredentials(awsAccessKeyID, awsSecretAccessKey, ""),
		})

		if err != nil {
			fmt.Printf("Error creating session for region %s: %v\n", region, err)
			os.Exit(1)
		}

		return sess
	}

	if VerboseModeFlag && !SessionGuidanceLogged {
		SessionGuidanceLogged = true
		fmt.Println("\tUsing default AWS profile")
	}

	sess, err = session.NewSession(&aws.Config{
		Region: aws.String(region),
	})

	if err != nil {
		fmt.Printf("Error creating session for region %s: %v\n\n", region, err)
		os.Exit(1)
	}

	return sess
}

func getUbuntuAMIId(svc *ec2.EC2, arch string) (string, error) {
	describeImagesInput := &ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("name"),
				Values: aws.StringSlice([]string{"ubuntu/images/hvm-ssd/ubuntu-*"}),
			},
			{
				Name:   aws.String("architecture"),
				Values: aws.StringSlice([]string{arch}),
			},
			{
				Name:   aws.String("state"),
				Values: aws.StringSlice([]string{"available"}),
			},
		},
		Owners: aws.StringSlice([]string{"099720109477"}), // Canonical's owner ID
	}

	// Call DescribeImages to find matching AMIs
	result, err := svc.DescribeImages(describeImagesInput)
	if err != nil {
		fmt.Printf("Failed to describe images, %v\n", err)
		return "", err
	}

	if len(result.Images) == 0 {
		fmt.Println("No Ubuntu AMIs found")
		return "", err
	}

	// Filter the results to find the latest image that matches the desired pattern
	var latestImage *ec2.Image
	for _, image := range result.Images {
		if strings.Contains(*image.Name, "ubuntu-jammy-22.04") {
			if latestImage == nil || *image.CreationDate > *latestImage.CreationDate {
				latestImage = image
			}
		}
	}

	if latestImage == nil {
		fmt.Println("No matching Ubuntu 22.04 AMIs found")
		return "", fmt.Errorf("no matching Ubuntu 22.04 AMIs found")
	}

	if VerboseModeFlag {
		fmt.Printf("Using AMI ID: %s\n", *latestImage.ImageId)
	}

	return *latestImage.ImageId, nil
}

func DeployOnAWS() {
	targetRegions := strings.Split(TargetRegionsFlag, ",")
	noOfOrchestratorNodes := ProjectSettings["NumberOfOrchestratorNodes"].(int)
	noOfComputeNodes := ProjectSettings["NumberOfComputeNodes"].(int)

	if command == "create" {
		// Ensure VPC and Security Groups exist
		ensureVPCAndSGsExist(targetRegions)
		createResources(targetRegions, noOfOrchestratorNodes, noOfComputeNodes)
	} else if command == "destroy" {
		destroyResources()
	} else if command == "list" {
		listResources()
	} else {
		fmt.Println("Unknown command. Use 'create', 'destroy', or 'list'.")
	}
}

func ensureVPCAndSGsExist(regions []string) {
	var wg sync.WaitGroup

	for _, region := range regions {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
			sess := GetSession(region)
			ec2Svc := ec2.New(sess)
			instanceType := "t2.medium"
			az, err := getAvailableZoneForInstanceType(ec2Svc, instanceType)
			if err != nil {
				fmt.Printf(
					"Instance type %s is not available in region %s: %v\n",
					instanceType,
					region,
					err,
				)
				return
			}
			createVPCAndSG(ec2Svc, region, az)
		}(region)
	}
	wg.Wait()
}

func createResources(regions []string, noOfOrchestratorNodes, noOfComputeNodes int) {
	var wg sync.WaitGroup
	var orchestratorIPs []string

	if OrchestratorIPFlag == "" {
		// Create Orchestrator nodes first
		for i := 0; i < noOfOrchestratorNodes; i++ {
			wg.Add(1)
			go func(region string) {
				defer wg.Done()
				sess := GetSession(region)
				ec2Svc := ec2.New(sess)
				instanceInfo := createInstanceInRegion(ec2Svc, region, "orchestrator", nil)
				orchestratorIPs = append(orchestratorIPs, instanceInfo.PublicIP)
			}(regions[i%len(regions)])
		}
		wg.Wait()

		// orchestratorIPsStr := formatOrchestratorIPs(orchestratorIPs)
		fmt.Println("Orchestrator nodes created with IPs:", orchestratorIPs)
	} else {
		orchestratorIPs = append(orchestratorIPs, OrchestratorIPFlag)
	}

	// Create Compute nodes next
	for i := 0; i < noOfComputeNodes; i++ {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
			sess := GetSession(region)
			ec2Svc := ec2.New(sess)
			createInstanceInRegion(ec2Svc, region, "compute", orchestratorIPs)
		}(regions[i%len(regions)])
	}
	wg.Wait()
}

func formatOrchestratorIPs(orchestratorIPs []string) string {
	var formattedIPs []string
	for _, ip := range orchestratorIPs {
		formattedIPs = append(formattedIPs, fmt.Sprintf("nats://%s:4222", ip))
	}
	return strings.Join(formattedIPs, ",")
}

func getAvailableZoneForInstanceType(svc *ec2.EC2, instanceType string) (string, error) {
	result, err := svc.DescribeInstanceTypeOfferings(&ec2.DescribeInstanceTypeOfferingsInput{
		LocationType: aws.String("availability-zone"),
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("instance-type"),
				Values: []*string{aws.String(instanceType)},
			},
		},
	})
	if err != nil {
		return "", err
	}

	for _, offering := range result.InstanceTypeOfferings {
		return *offering.Location, nil
	}

	return "", fmt.Errorf("instance type %s not available in any availability zone", instanceType)
}

//nolint:funlen,gocyclo
func createVPCAndSG(svc *ec2.EC2, region string, availabilityZone string) {
	retryPolicy := 3
	for i := 0; i < retryPolicy; i++ {
		// Check if VPC with the tag already exists
		vpcs, err := svc.DescribeVpcs(&ec2.DescribeVpcsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("tag:project"),
					Values: []*string{aws.String("andaime")},
				},
			},
		})
		if err != nil {
			if isRetryableError(err) && i < retryPolicy-1 {
				time.Sleep(RetryTimeout) //nolint:mnd
				continue
			}
			fmt.Printf("Unable to describe VPCs in region %s: %v\n", region, err)
			return
		}

		var vpcID *string

		if len(vpcs.Vpcs) > 0 {
			if VerboseModeFlag {
				fmt.Printf("VPC already exists in region %s\n", region)
			}
			vpcID = vpcs.Vpcs[0].VpcId
		} else {
			// Create VPC
			vpcOutput, err := svc.CreateVpc(&ec2.CreateVpcInput{
				CidrBlock: aws.String("10.0.0.0/16"),
				TagSpecifications: []*ec2.TagSpecification{
					{
						ResourceType: aws.String("vpc"),
						Tags: []*ec2.Tag{
							{
								Key:   aws.String("project"),
								Value: aws.String("andaime"),
							},
							{
								Key:   aws.String("Name"),
								Value: aws.String("Bacalhau-VPC"),
							},
						},
					},
				},
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout) //nolint:mnd
					continue
				}
				fmt.Printf("Unable to create VPC in region %s: %v\n", region, err)
				return
			}

			vpcID = vpcOutput.Vpc.VpcId
			if VerboseModeFlag {
				fmt.Printf("Created VPC in region %s with ID %s\n", region, *vpcID)
			}

			// Create Subnet
			subnetOutput, err := svc.CreateSubnet(&ec2.CreateSubnetInput{
				CidrBlock:        aws.String("10.0.1.0/24"),
				VpcId:            vpcID,
				AvailabilityZone: aws.String(availabilityZone),
				TagSpecifications: []*ec2.TagSpecification{
					{
						ResourceType: aws.String("subnet"),
						Tags: []*ec2.Tag{
							{
								Key:   aws.String("project"),
								Value: aws.String("andaime"),
							},
							{
								Key:   aws.String("Name"),
								Value: aws.String("Bacalhau-Subnet"),
							},
						},
					},
				},
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout) //nolint:mnd
					continue
				}
				fmt.Printf("Unable to create subnet in region %s: %v\n", region, err)
				return
			}
			subnetID := subnetOutput.Subnet.SubnetId
			if VerboseModeFlag {
				fmt.Printf("Created Subnet in region %s with ID %s\n", region, *subnetID)
			}

			// Create Internet Gateway
			igwOutput, err := svc.CreateInternetGateway(&ec2.CreateInternetGatewayInput{
				TagSpecifications: []*ec2.TagSpecification{
					{
						ResourceType: aws.String("internet-gateway"),
						Tags: []*ec2.Tag{
							{
								Key:   aws.String("project"),
								Value: aws.String("andaime"),
							},
							{
								Key:   aws.String("Name"),
								Value: aws.String("Bacalhau-IGW"),
							},
						},
					},
				},
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout) //nolint:mnd
					continue
				}
				fmt.Printf("Unable to create internet gateway in region %s: %v\n", region, err)
				return
			}
			igwID := igwOutput.InternetGateway.InternetGatewayId
			if VerboseModeFlag {
				fmt.Printf("Created Internet Gateway in region %s with ID %s\n", region, *igwID)
			}

			// Attach Internet Gateway to VPC
			_, err = svc.AttachInternetGateway(&ec2.AttachInternetGatewayInput{
				InternetGatewayId: igwID,
				VpcId:             vpcID,
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout) //nolint:mnd
					continue
				}
				fmt.Printf("Unable to attach internet gateway to VPC in region %s: %v\n", region, err)
				return
			}
			if VerboseModeFlag {
				fmt.Printf("Attached Internet Gateway to VPC in region %s\n", region)
			}

			// Create Route Table
			routeTableOutput, err := svc.CreateRouteTable(&ec2.CreateRouteTableInput{
				VpcId: vpcID,
				TagSpecifications: []*ec2.TagSpecification{
					{
						ResourceType: aws.String("route-table"),
						Tags: []*ec2.Tag{
							{
								Key:   aws.String("project"),
								Value: aws.String("andaime"),
							},
							{
								Key:   aws.String("Name"),
								Value: aws.String("Bacalhau-RouteTable"),
							},
						},
					},
				},
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout) //nolint:mnd
					continue
				}
				fmt.Printf("Unable to create route table in region %s: %v\n", region, err)
				return
			}
			routeTableID := routeTableOutput.RouteTable.RouteTableId
			if VerboseModeFlag {
				fmt.Printf("Created Route Table in region %s with ID %s\n", region, *routeTableID)
			}

			// Create Route to Internet Gateway
			_, err = svc.CreateRoute(&ec2.CreateRouteInput{
				RouteTableId:         routeTableID,
				DestinationCidrBlock: aws.String("0.0.0.0/0"),
				GatewayId:            igwID,
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout) //nolint:mnd
					continue
				}
				fmt.Printf("Unable to create route to internet gateway in region %s: %v\n", region, err)
				return
			}
			if VerboseModeFlag {
				fmt.Printf("Created route to Internet Gateway in region %s\n", region)
			}

			// Associate Subnet with Route Table
			_, err = svc.AssociateRouteTable(&ec2.AssociateRouteTableInput{
				RouteTableId: routeTableID,
				SubnetId:     subnetID,
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout) //nolint:mnd
					continue
				}
				fmt.Printf("Unable to associate subnet with route table in region %s: %v\n", region, err)
				return
			}
			if VerboseModeFlag {
				fmt.Printf("Associated Subnet with Route Table in region %s\n", region)
			}
		}

		// Create Security Group if it doesn't exist
		createSecurityGroupIfNotExists(svc, vpcID, region)
		break
	}
}

//nolint:funlen,gocyclo,unused
func createInstancesRoundRobin(regions []string, instanceCount int, orchestratorIPs []string) {
	instanceCreated := 0
	regionIndex := 0
	numRegions := len(regions)
	instanceChannel := make(chan InstanceInfo, instanceCount)

	var wg sync.WaitGroup

	for instanceCreated < instanceCount {
		region := regions[regionIndex]
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
			sess := GetSession(region)
			ec2Svc := ec2.New(sess)
			instanceInfo := createInstanceInRegion(ec2Svc, region, "compute", orchestratorIPs)
			instanceChannel <- instanceInfo
		}(region)

		instanceCreated++
		regionIndex = (regionIndex + 1) % numRegions
	}

	wg.Wait()
	close(instanceChannel)

	var instances []InstanceInfo
	for instanceInfo := range instanceChannel {
		instances = append(instances, instanceInfo)
	}

	if VerboseModeFlag {
		fmt.Println("Created instances:")
		for _, instance := range instances {
			fmt.Printf(
				"Region: %s, Instance ID: %s, Public IPv4: %s\n",
				instance.Region,
				instance.InstanceID,
				instance.PublicIP,
			)
		}
	}
}

//nolint:funlen,gocyclo,unused
func createInstanceInRegion(
	svc *ec2.EC2,
	region string,
	nodeType string,
	orchestratorIPs []string,
) InstanceInfo {
	retryPolicy := 3
	var instanceInfo InstanceInfo

	for i := 0; i < retryPolicy; i++ {
		// Find the VPC ID with the tag "project=andaime"
		vpcs, err := svc.DescribeVpcs(&ec2.DescribeVpcsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("tag:project"),
					Values: []*string{aws.String("andaime")},
				},
			},
		})
		if err != nil {
			if isRetryableError(err) && i < retryPolicy-1 {
				time.Sleep(RetryTimeout) //nolint:mnd
				continue
			}
			fmt.Printf("Unable to find VPC in region %s: %v\n", region, err)
			return instanceInfo
		}
		if len(vpcs.Vpcs) == 0 {
			fmt.Printf("No VPC found in region %s\n", region)
			return instanceInfo
		}
		vpcID := vpcs.Vpcs[0].VpcId

		// Find the Subnet ID with the tag "project=andaime"
		subnets, err := svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("vpc-id"),
					Values: []*string{vpcID},
				},
				{
					Name:   aws.String("tag:project"),
					Values: []*string{aws.String("andaime")},
				},
			},
		})
		if err != nil {
			if isRetryableError(err) && i < retryPolicy-1 {
				time.Sleep(RetryTimeout) //nolint:mnd
				continue
			}
			fmt.Printf("Unable to find subnet in region %s: %v\n", region, err)
			return instanceInfo
		}
		if len(subnets.Subnets) == 0 {
			fmt.Printf("No subnet found in region %s\n", region)
			return instanceInfo
		}
		subnetID := subnets.Subnets[0].SubnetId

		// Find the Security Group ID with the tag "project=andaime"
		sgs, err := svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("vpc-id"),
					Values: []*string{vpcID},
				},
				{
					Name:   aws.String("tag:project"),
					Values: []*string{aws.String("andaime")},
				},
			},
		})
		if err != nil {
			if isRetryableError(err) && i < retryPolicy-1 {
				time.Sleep(RetryTimeout) //nolint:mnd
				continue
			}
			fmt.Printf("Unable to find security group in region %s: %v\n", region, err)
			return instanceInfo
		}
		if len(sgs.SecurityGroups) == 0 {
			fmt.Printf("No security group found in region %s\n", region)
			return instanceInfo
		}
		sgID := sgs.SecurityGroups[0].GroupId

		// Get the instance architecture
		instanceType := "t2.medium"

		if nodeType == "orchestrator" && OrchestratorInstanceTypeFlag != "" {
			instanceType = OrchestratorInstanceTypeFlag
		}

		if nodeType == "compute" && ComputeInstanceTypeFlag != "" {
			instanceType = ComputeInstanceTypeFlag
		}

		fmt.Printf("Chosen instance type for '%s' node: %s\n", nodeType, instanceType)

		instanceTypeDetails, err := svc.DescribeInstanceTypes(&ec2.DescribeInstanceTypesInput{
			InstanceTypes: []*string{aws.String(instanceType)},
		})

		if err != nil {
			fmt.Printf("Unable to describe instance type %s: %v\n", instanceType, err)
			return instanceInfo
		}

		var architecture string
		for _, arch := range instanceTypeDetails.InstanceTypes[0].ProcessorInfo.SupportedArchitectures {
			for _, validArch := range ValidArchitectures {
				if *arch == validArch {
					architecture = *arch
					break
				}
			}
			if architecture != "" {
				break
			}
		}

		fmt.Printf("selected arch type %s\n", architecture)
		if architecture == "" {
			fmt.Printf("No valid architecture found for instance type %s\n", instanceType)
			return instanceInfo
		}

		// Get the Ubuntu 22.04 AMI ID
		amiID, err := getUbuntuAMIId(svc, architecture)
		if err != nil {
			if isRetryableError(err) && i < retryPolicy-1 {
				time.Sleep(RetryTimeout) //nolint:mnd
				continue
			}
			fmt.Printf("Unable to find Ubuntu 22.04 AMI in region %s: %v\n", region, err)
			return instanceInfo
		}

		// Read and encode startup scripts
		templateData := TemplateData{
			ProjectName:               ProjectSettings["ProjectName"].(string),
			TargetPlatform:            ProjectSettings["TargetPlatform"].(string),
			NumberOfOrchestratorNodes: ProjectSettings["NumberOfOrchestratorNodes"].(int),
			NumberOfComputeNodes:      ProjectSettings["NumberOfComputeNodes"].(int),
			TargetRegions:             TargetRegionsFlag,
			AwsProfile:                AWSProfileFlag,
			NodeType:                  nodeType,
		}

		if nodeType == "compute" && len(orchestratorIPs) > 0 {
			templateData.OrchestratorIPs = formatOrchestratorIPs(orchestratorIPs)
		}

		userData, err := readStartupScripts("startup_scripts", templateData)
		if err != nil {
			fmt.Printf("Unable to read startup scripts: %v\n", err)
			return instanceInfo
		}
		encodedUserData := base64.StdEncoding.EncodeToString([]byte(userData))

		bootVolumeSize := int64(BootVolumeSizeFlag) // Example: 50 GiB

		// Create EC2 Instance
		runResult, err := svc.RunInstances(&ec2.RunInstancesInput{
			ImageId:      aws.String(amiID),
			InstanceType: aws.String(instanceType),
			MaxCount:     aws.Int64(1),
			MinCount:     aws.Int64(1),
			BlockDeviceMappings: []*ec2.BlockDeviceMapping{
				{
					DeviceName: aws.String(
						"/dev/sda1",
					), // This might need to be adjusted based on the AMI
					Ebs: &ec2.EbsBlockDevice{
						VolumeSize:          aws.Int64(bootVolumeSize),
						DeleteOnTermination: aws.Bool(true),
						VolumeType:          aws.String("gp2"), // General Purpose SSD
					},
				},
			},
			NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
				{
					AssociatePublicIpAddress: aws.Bool(true),
					DeviceIndex:              aws.Int64(0),
					SubnetId:                 subnetID,
					Groups:                   []*string{sgID},
				},
			},
			UserData: aws.String(encodedUserData),
			TagSpecifications: []*ec2.TagSpecification{
				{
					ResourceType: aws.String("instance"),
					Tags: []*ec2.Tag{
						{
							Key:   aws.String("project"),
							Value: aws.String("andaime"),
						},
						{
							Key:   aws.String("Name"),
							Value: aws.String(fmt.Sprintf("Bacalhau-Node-%s", nodeType)),
						},
					},
				},
			},
		})
		if err != nil {
			if isRetryableError(err) && i < retryPolicy-1 {
				time.Sleep(RetryTimeout) //nolint:mnd
				continue
			}
			fmt.Printf("Unable to create instance in region %s: %v\n", region, err)
			return instanceInfo
		}

		instanceID := *runResult.Instances[0].InstanceId

		// Wait until the instance is running
		err = svc.WaitUntilInstanceRunning(&ec2.DescribeInstancesInput{
			InstanceIds: []*string{&instanceID},
		})
		if err != nil {
			fmt.Printf(
				"Error waiting for instance %s to run in region %s: %v\n",
				instanceID,
				region,
				err,
			)
			return instanceInfo
		}

		// Get instance details
		descResult, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: []*string{&instanceID},
		})
		if err != nil {
			fmt.Printf("Unable to describe instance %s in region %s: %v\n", instanceID, region, err)
			return instanceInfo
		}

		instance := descResult.Reservations[0].Instances[0]
		publicIP := "No public IP"
		if instance.PublicIpAddress != nil {
			publicIP = *instance.PublicIpAddress
		}

		if VerboseModeFlag {
			fmt.Printf(
				"Compute node created in %s, with Instance ID '%s' and public IP address: %s\n",
				region,
				instanceID,
				publicIP,
			)
		}

		instanceInfo = InstanceInfo{
			InstanceID: instanceID,
			Region:     region,
			PublicIP:   publicIP,
		}
		break
	}

	return instanceInfo
}

func createSecurityGroupIfNotExists(svc *ec2.EC2, vpcID *string, region string) *string {
	retryPolicy := 3
	for i := 0; i < retryPolicy; i++ {
		sgs, err := svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("tag:project"),
					Values: []*string{aws.String("andaime")},
				},
			},
		})
		if err != nil {
			if isRetryableError(err) && i < retryPolicy-1 {
				time.Sleep(RetryTimeout) //nolint:mnd
				continue
			}
			fmt.Printf("Unable to describe security groups in region %s: %v\n", region, err)
			return nil
		}

		if len(sgs.SecurityGroups) > 0 {
			return sgs.SecurityGroups[0].GroupId
		}

		// Create Security Group
		sgOutput, err := svc.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
			GroupName:   aws.String("custom-sg"),
			Description: aws.String("Custom security group"),
			VpcId:       vpcID,
			TagSpecifications: []*ec2.TagSpecification{
				{
					ResourceType: aws.String("security-group"),
					Tags: []*ec2.Tag{
						{
							Key:   aws.String("project"),
							Value: aws.String("andaime"),
						},
					},
				},
			},
		})
		if err != nil {
			if isRetryableError(err) && i < retryPolicy-1 {
				time.Sleep(RetryTimeout)
				continue
			}
			fmt.Printf("Unable to create security group in region %s: %v\n", region, err)
			return nil
		}
		sgID := sgOutput.GroupId

		// Authorize Inbound Traffic
		ports := []int64{22, 80, 4222, 1234}
		for _, port := range ports {
			_, err := svc.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
				GroupId: sgID,
				IpPermissions: []*ec2.IpPermission{
					{
						IpProtocol: aws.String("tcp"),
						FromPort:   aws.Int64(port),
						ToPort:     aws.Int64(port),
						IpRanges: []*ec2.IpRange{
							{
								CidrIp: aws.String("0.0.0.0/0"),
							},
						},
					},
				},
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout)
					continue
				}
				fmt.Printf(
					"Unable to authorize inbound traffic on port %d in region %s: %v\n",
					port,
					region,
					err,
				)
				return nil
			}
		}
		if VerboseModeFlag == true {
			fmt.Printf("Created security group in region %s\n", region)
		}
		return sgID
	}
	return nil
}

func destroyResources() {
	regions, err := getAllRegions()
	if err != nil {
		fmt.Printf("Unable to get list of regions: %v\n", err)
		return
	}

	var wg sync.WaitGroup

	for _, region := range regions {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
			sess := GetSession(region)
			ec2Svc := ec2.New(sess)
			deleteTaggedResources(ec2Svc, region)
		}(region)
	}

	wg.Wait()
	fmt.Println("Resources destroyed in all regions.")
}

func getAllRegions() ([]string, error) {
	sess := GetSession("us-east-1")
	ec2Svc := ec2.New(sess)
	result, err := ec2Svc.DescribeRegions(&ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, err
	}

	var regions []string
	for _, region := range result.Regions {
		regions = append(regions, *region.RegionName)
	}

	return regions, nil
}

func deleteTaggedResources(svc *ec2.EC2, region string) {
	retryPolicy := 3
	for i := 0; i < retryPolicy; i++ {
		// Describe instances with the tag "project=andaime"
		instances, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("tag:project"),
					Values: []*string{aws.String("andaime")},
				},
			},
		})
		if err != nil {
			if isRetryableError(err) && i < retryPolicy-1 {
				time.Sleep(RetryTimeout)
				continue
			}
			fmt.Printf("Unable to describe instances in region %s: %v\n", region, err)
			return
		}

		// Terminate instances
		var instanceIds []*string
		for _, reservation := range instances.Reservations {
			for _, instance := range reservation.Instances {
				instanceIds = append(instanceIds, instance.InstanceId)
			}
		}
		if len(instanceIds) > 0 {
			_, err = svc.TerminateInstances(&ec2.TerminateInstancesInput{
				InstanceIds: instanceIds,
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout)
					continue
				}
				fmt.Printf("Unable to terminate instances in region %s: %v\n", region, err)
				return
			}
			if VerboseModeFlag == true {
				fmt.Printf("Terminated instances in region %s\n", region)
			}

			// Wait until instances are terminated
			err = svc.WaitUntilInstanceTerminated(&ec2.DescribeInstancesInput{
				InstanceIds: instanceIds,
			})
			if err != nil {
				fmt.Printf(
					"Error waiting for instances to terminate in region %s: %v\n",
					region,
					err,
				)
				return
			}
		}

		// Describe security groups with the tag "project=andaime"
		sgs, err := svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("tag:project"),
					Values: []*string{aws.String("andaime")},
				},
			},
		})
		if err != nil {
			if isRetryableError(err) && i < retryPolicy-1 {
				time.Sleep(RetryTimeout)
				continue
			}
			fmt.Printf("Unable to describe security groups in region %s: %v\n", region, err)
			return
		}

		// Delete security groups
		for _, sg := range sgs.SecurityGroups {
			_, err = svc.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
				GroupId: sg.GroupId,
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout)
					continue
				}
				fmt.Printf(
					"Unable to delete security group %s in region %s: %v\n",
					*sg.GroupId,
					region,
					err,
				)
				continue
			}
			if VerboseModeFlag == true {
				fmt.Printf("Deleted security group %s in region %s\n", *sg.GroupId, region)
			}
		}

		// Describe VPCs with the tag "project=andaime"
		vpcs, err := svc.DescribeVpcs(&ec2.DescribeVpcsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("tag:project"),
					Values: []*string{aws.String("andaime")},
				},
			},
		})
		if err != nil {
			if isRetryableError(err) && i < retryPolicy-1 {
				time.Sleep(RetryTimeout)
				continue
			}
			fmt.Printf("Unable to describe VPCs in region %s: %v\n", region, err)
			return
		}

		// Delete VPCs
		for _, vpc := range vpcs.Vpcs {
			// Describe and delete subnets
			subnets, err := svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
				Filters: []*ec2.Filter{
					{
						Name:   aws.String("vpc-id"),
						Values: []*string{vpc.VpcId},
					},
				},
			})
			if err != nil {
				fmt.Printf(
					"Unable to describe subnets for VPC %s in region %s: %v\n",
					*vpc.VpcId,
					region,
					err,
				)
				continue
			}
			for _, subnet := range subnets.Subnets {
				_, err = svc.DeleteSubnet(&ec2.DeleteSubnetInput{
					SubnetId: subnet.SubnetId,
				})
				if err != nil {
					fmt.Printf(
						"Unable to delete subnet %s in region %s: %v\n",
						*subnet.SubnetId,
						region,
						err,
					)
					continue
				}
				if VerboseModeFlag == true {
					fmt.Printf("Deleted subnet %s in region %s\n", *subnet.SubnetId, region)
				}
			}

			// Describe and delete route tables
			routeTables, err := svc.DescribeRouteTables(&ec2.DescribeRouteTablesInput{
				Filters: []*ec2.Filter{
					{
						Name:   aws.String("vpc-id"),
						Values: []*string{vpc.VpcId},
					},
				},
			})
			if err != nil {
				fmt.Printf(
					"Unable to describe route tables for VPC %s in region %s: %v\n",
					*vpc.VpcId,
					region,
					err,
				)
				continue
			}
			for _, routeTable := range routeTables.RouteTables {
				if len(routeTable.Associations) > 0 && *routeTable.Associations[0].Main {
					continue // Skip the main route table
				}
				_, err = svc.DeleteRouteTable(&ec2.DeleteRouteTableInput{
					RouteTableId: routeTable.RouteTableId,
				})
				if err != nil {
					fmt.Printf(
						"Unable to delete route table %s in region %s: %v\n",
						*routeTable.RouteTableId,
						region,
						err,
					)
					continue
				}
				if VerboseModeFlag == true {
					fmt.Printf(
						"Deleted route table %s in region %s\n",
						*routeTable.RouteTableId,
						region,
					)
				}
			}

			// Detach Internet Gateway
			igws, err := svc.DescribeInternetGateways(&ec2.DescribeInternetGatewaysInput{
				Filters: []*ec2.Filter{
					{
						Name:   aws.String("attachment.vpc-id"),
						Values: []*string{vpc.VpcId},
					},
				},
			})
			if err != nil {
				fmt.Printf(
					"Unable to describe internet gateways for VPC %s in region %s: %v\n",
					*vpc.VpcId,
					region,
					err,
				)
				continue
			}
			for _, igw := range igws.InternetGateways {
				_, err = svc.DetachInternetGateway(&ec2.DetachInternetGatewayInput{
					InternetGatewayId: igw.InternetGatewayId,
					VpcId:             vpc.VpcId,
				})
				if err != nil {
					fmt.Printf(
						"Unable to detach internet gateway %s from VPC %s in region %s: %v\n",
						*igw.InternetGatewayId,
						*vpc.VpcId,
						region,
						err,
					)
					continue
				}
				if VerboseModeFlag == true {
					fmt.Printf(
						"Detached internet gateway %s from VPC %s in region %s\n",
						*igw.InternetGatewayId,
						*vpc.VpcId,
						region,
					)
				}

				// Delete Internet Gateway
				_, err = svc.DeleteInternetGateway(&ec2.DeleteInternetGatewayInput{
					InternetGatewayId: igw.InternetGatewayId,
				})
				if err != nil {
					fmt.Printf(
						"Unable to delete internet gateway %s in region %s: %v\n",
						*igw.InternetGatewayId,
						region,
						err,
					)
					continue
				}
				if VerboseModeFlag == true {
					fmt.Printf(
						"Deleted internet gateway %s in region %s\n",
						*igw.InternetGatewayId,
						region,
					)
				}
			}

			_, err = svc.DeleteVpc(&ec2.DeleteVpcInput{
				VpcId: vpc.VpcId,
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout)
					continue
				}
				fmt.Printf("Unable to delete VPC %s in region %s: %v\n", *vpc.VpcId, region, err)
				continue
			}
			if VerboseModeFlag == true {
				fmt.Printf("Deleted VPC %s in region %s\n", *vpc.VpcId, region)
			}
		}
		break
	}
}

func listResources() {
	regions, err := getAllRegions()
	if err != nil {
		fmt.Printf("Unable to get list of regions: %v\n", err)
		return
	}

	resourceTypes := []string{
		"VPC",
		"Subnet",
		"Internet Gateway",
		"Route Table",
		"Security Group",
		"Instance",
	}
	resourcesByRegion := make(map[string]map[string][]string)

	var wg sync.WaitGroup
	totalNodes := 0
	totalRegions := 0

	for _, region := range regions {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
			sess := GetSession(region)
			ec2Svc := ec2.New(sess)

			resources := make(map[string][]string)
			for _, resourceType := range resourceTypes {
				resources[resourceType] = listTaggedResources(ec2Svc, resourceType, region)
			}

			if len(resources["VPC"]) > 0 || len(resources["Subnet"]) > 0 ||
				len(resources["Internet Gateway"]) > 0 ||
				len(resources["Route Table"]) > 0 ||
				len(resources["Security Group"]) > 0 ||
				len(resources["Instance"]) > 0 {
				resourcesByRegion[region] = resources
			}
		}(region)
	}

	wg.Wait()

	fmt.Println("\n== Resources Report ==")
	for region, resources := range resourcesByRegion {
		fmt.Println("\n=======================")
		fmt.Println("||")
		fmt.Printf("|| Resources in region: %s\n", region)
		fmt.Println("||")
		fmt.Println("=======================")
		fmt.Println("")

		for resourceType, resourceList := range resources {
			if len(resourceList) > 0 {
				fmt.Printf("\t%s:\n", resourceType)
				for _, resourceID := range resourceList {
					fmt.Printf("\t\t- %s\n", resourceID)
				}
				fmt.Printf("\n")
			}
		}

		totalRegions++
		totalNodes += len(resources["Instance"])

		fmt.Printf("\n\n")
	}

	fmt.Printf("%d nodes in %d regions\n", totalNodes, totalRegions)
}

func listTaggedResources(svc *ec2.EC2, resourceType string, region string) []string {
	var resourceList []string
	retryPolicy := 3
	for i := 0; i < retryPolicy; i++ {
		switch resourceType {
		case "VPC":
			vpcs, err := svc.DescribeVpcs(&ec2.DescribeVpcsInput{
				Filters: []*ec2.Filter{
					{
						Name:   aws.String("tag:project"),
						Values: []*string{aws.String("andaime")},
					},
				},
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout)
					continue
				}
				fmt.Printf("Unable to describe VPCs in region %s: %v\n", region, err)
				return nil
			}
			for _, vpc := range vpcs.Vpcs {
				resourceList = append(resourceList, *vpc.VpcId)
			}

		case "Subnet":
			subnets, err := svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
				Filters: []*ec2.Filter{
					{
						Name:   aws.String("tag:project"),
						Values: []*string{aws.String("andaime")},
					},
				},
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout)
					continue
				}
				fmt.Printf("Unable to describe subnets in region %s: %v\n", region, err)
				return nil
			}
			for _, subnet := range subnets.Subnets {
				resourceList = append(resourceList, *subnet.SubnetId)
			}

		case "Internet Gateway":
			igws, err := svc.DescribeInternetGateways(&ec2.DescribeInternetGatewaysInput{
				Filters: []*ec2.Filter{
					{
						Name:   aws.String("tag:project"),
						Values: []*string{aws.String("andaime")},
					},
				},
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout)
					continue
				}
				fmt.Printf("Unable to describe internet gateways in region %s: %v\n", region, err)
				return nil
			}
			for _, igw := range igws.InternetGateways {
				resourceList = append(resourceList, *igw.InternetGatewayId)
			}

		case "Route Table":
			routeTables, err := svc.DescribeRouteTables(&ec2.DescribeRouteTablesInput{
				Filters: []*ec2.Filter{
					{
						Name:   aws.String("tag:project"),
						Values: []*string{aws.String("andaime")},
					},
				},
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout)
					continue
				}
				fmt.Printf("Unable to describe route tables in region %s: %v\n", region, err)
				return nil
			}
			for _, routeTable := range routeTables.RouteTables {
				resourceList = append(resourceList, *routeTable.RouteTableId)
			}

		case "Security Group":
			sgs, err := svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
				Filters: []*ec2.Filter{
					{
						Name:   aws.String("tag:project"),
						Values: []*string{aws.String("andaime")},
					},
				},
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout)
					continue
				}
				fmt.Printf("Unable to describe security groups in region %s: %v\n", region, err)
				return nil
			}
			for _, sg := range sgs.SecurityGroups {
				resourceList = append(resourceList, *sg.GroupId)
			}

		case "Instance":
			instances, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
				Filters: []*ec2.Filter{
					{
						Name:   aws.String("tag:project"),
						Values: []*string{aws.String("andaime")},
					},
					{
						Name: aws.String("instance-state-name"),
						Values: []*string{
							aws.String("running"),
							aws.String("stopped"),
						},
					},
				},
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(RetryTimeout)
					continue
				}
				fmt.Printf("Unable to describe instances in region %s: %v\n", region, err)
				return nil
			}
			for _, reservation := range instances.Reservations {
				for _, instance := range reservation.Instances {
					resourceList = append(
						resourceList,
						fmt.Sprintf(
							"ID: %s, Type: %s, IPv4: %s",
							*instance.InstanceId,
							*instance.InstanceType,
							*instance.PublicIpAddress,
						),
					)
				}
			}
		}
		break
	}

	return resourceList
}

func isRetryableError(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		switch awsErr.Code() {
		case "RequestExpired", "RequestTimeout", "Throttling", "InternalFailure":
			return true
		}
	}
	return false
}

func readStartupScripts(dir string, templateData TemplateData) (string, error) {
	var combinedScript strings.Builder

	files, err := startupScriptsFS.ReadDir(dir)
	if err != nil {
		return "", err
	}

	// Create a slice to hold the file names with their order
	type orderedFile struct {
		order int
		name  string
	}

	var orderedFiles []orderedFile

	for _, file := range files {
		if !file.IsDir() {
			parts := strings.SplitN(file.Name(), "_", 2)
			if len(parts) < 2 {
				continue
			}
			order, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}
			orderedFiles = append(orderedFiles, orderedFile{order: order, name: file.Name()})
		}
	}

	// Sort the files by the order
	sort.Slice(orderedFiles, func(i, j int) bool {
		return orderedFiles[i].order < orderedFiles[j].order
	})

	combinedScript.WriteString("#!/bin/bash\n")

	// Read and concatenate the content of the files in order
	for _, orderedFile := range orderedFiles {
		content, err := startupScriptsFS.ReadFile(filepath.Join(dir, orderedFile.name))
		if err != nil {
			return "", err
		}

		tmpl, err := template.New("script").Parse(string(content))
		if err != nil {
			return "", err
		}

		var script strings.Builder
		err = tmpl.Execute(&script, templateData)
		if err != nil {
			return "", err
		}

		combinedScript.WriteString(script.String())
		combinedScript.WriteString("\n")
	}

	return combinedScript.String(), nil
}

func ProcessEnvVars() {
	if os.Getenv("PROJECT_NAME") != "" {
		if VerboseModeFlag == true {
			fmt.Println(`Setting "PROJECT_NAME" from environment variable`)
		}

		ProjectSettings["ProjectName"] = os.Getenv("PROJECT_NAME")
		SetBy["ProjectName"] = "environment variable"
	}

	if os.Getenv("TARGET_PLATFORM") != "" {
		if VerboseModeFlag == true {
			fmt.Println(`Setting "TARGET_PLATFORM" from environment variable`)
		}

		ProjectSettings["TargetPlatform"] = os.Getenv("TARGET_PLATFORM")
		SetBy["TargetPlatform"] = "environment variable"
	}

	if os.Getenv("NUMBER_OF_ORCHESTRATOR_NODES") != "" {
		if VerboseModeFlag == true {
			fmt.Println(`Setting "NUMBER_OF_ORCHESTRATOR_NODES" from environment variable`)
		}

		ProjectSettings["NumberOfOrchestratorNodes"], _ = strconv.Atoi(
			os.Getenv("NUMBER_OF_ORCHESTRATOR_NODES"),
		)
		SetBy["NumberOfOrchestratorNodes"] = "environment variable"
	}

	if os.Getenv("NUMBER_OF_COMPUTE_NODES") != "" {
		if VerboseModeFlag == true {
			fmt.Println(`Setting "NUMBER_OF_COMPUTE_NODES" from environment variable`)
		}

		ProjectSettings["NumberOfComputeNodes"], _ = strconv.Atoi(
			os.Getenv("NUMBER_OF_COMPUTE_NODES"),
		)
		SetBy["NumberOfComputeNodes"] = "environment variable"
	}
}

func ProcessConfigFile() error {
	_, err := os.Stat("./config.json")

	if os.IsNotExist(err) {
		if VerboseModeFlag == true {
			fmt.Println("./config.json does not exist. Skipping...")
		}

		return nil
	}

	config_file, config_err := os.ReadFile("./config.json")

	if config_err != nil {
		fmt.Println("Could not read configuration file:", config_err)
		return config_err
	}

	if config_file != nil {
		var configJson map[string]interface{}

		config_err = json.Unmarshal(config_file, &configJson)

		if config_err != nil {
			return config_err
		}

		if configJson["PROJECT_NAME"] != nil {
			if VerboseModeFlag == true {
				fmt.Println(`Setting "PROJECT_NAME" from configuration file`)
			}

			ProjectSettings["ProjectName"] = configJson["PROJECT_NAME"].(string)
			SetBy["ProjectName"] = "configuration file"
		}

		if configJson["TARGET_PLATFORM"] != nil {
			if VerboseModeFlag == true {
				fmt.Println(`Setting "TARGET_PLATFORM" from configuration file`)
			}

			ProjectSettings["TargetPlatform"] = configJson["TARGET_PLATFORM"].(string)
			SetBy["TargetPlatform"] = "configuration file"
		}

		if configJson["NUMBER_OF_ORCHESTRATOR_NODES"] != nil {
			if VerboseModeFlag == true {
				fmt.Println(`Setting "NUMBER_OF_ORCHESTRATOR_NODES" from configuration file`)
			}

			ProjectSettings["NumberOfOrchestratorNodes"] = int(
				configJson["NUMBER_OF_ORCHESTRATOR_NODES"].(float64),
			)
			SetBy["NumberOfOrchestratorNodes"] = "configuration file"
		}

		if configJson["NUMBER_OF_COMPUTE_NODES"] != nil {
			if VerboseModeFlag == true {
				fmt.Println(`Setting "NUMBER_OF_COMPUTE_NODES" from configuration file`)
			}

			ProjectSettings["NumberOfComputeNodes"] = int(
				configJson["NUMBER_OF_COMPUTE_NODES"].(float64),
			)
			SetBy["NumberOfComputeNodes"] = "configuration file"
		}
	}

	return config_err
}

func ProcessFlags() {
	if ProjectNameFlag != "" {
		if VerboseModeFlag == true {
			fmt.Println(`Setting "PROJECT_NAME" by flag`)
		}

		ProjectSettings["ProjectName"] = ProjectNameFlag
		SetBy["ProjectName"] = "flag --project-name"
	}

	if TargetPlatformFlag != "" {
		if VerboseModeFlag == true {
			fmt.Println(`Setting "TARGET_PLATFORM" by flag`)
		}
		ProjectSettings["TargetPlatform"] = TargetPlatformFlag
		SetBy["TargetPlatform"] = "flag --target-platform"
	}

	if NumberOfOrchestratorNodesFlag != -1 {
		if VerboseModeFlag == true {
			fmt.Println(`Setting "NUMBER_OF_ORCHESTRATOR_NODES" by flag`)
		}

		ProjectSettings["NumberOfOrchestratorNodes"] = NumberOfOrchestratorNodesFlag
		SetBy["NumberOfOrchestratorNodes"] = "flag --orchestrator-nodes"

		if NumberOfOrchestratorNodesFlag > 1 {
			ProjectSettings["NumberOfOrchestratorNodes"] = 1
		}
	}

	if NumberOfComputeNodesFlag != -1 {
		if VerboseModeFlag == true {
			fmt.Println(`Setting "NUMBER_OF_COMPUTE_NODES_FLAG" by flag`)
		}

		ProjectSettings["NumberOfComputeNodes"] = NumberOfComputeNodesFlag
		SetBy["NumberOfComputeNodes"] = "flag --compute-nodes"
	}

	if OrchestratorIPFlag != "" {
		if VerboseModeFlag == true {
			fmt.Println(`Setting "ORCHESTRATOR_IP" by flag`)
		}
	}
}
func andaime_main(cmd string, _ ...[]string) {
	fmt.Println("\n== Andaime ==")
	fmt.Println("=======================")
	fmt.Println("")

	// Assign it to the global value
	command = cmd
	command = os.Args[1]
	if command == "--help" || command == "-h" {
		PrintUsage()
		os.Exit(0)
	}

	ProcessEnvVars()
	configErr := ProcessConfigFile()

	if configErr != nil {
		fmt.Println("Error reading configuration file:", configErr)
	}

	flag.BoolVar(
		&VerboseModeFlag,
		"verbose",
		false,
		"Generate verbose output throughout execution",
	)
	flag.StringVar(&ProjectNameFlag, "project-name", "", "Set project name")
	flag.StringVar(&TargetPlatformFlag, "target-platform", "", "Set target platform")
	flag.IntVar(
		&NumberOfOrchestratorNodesFlag,
		"orchestrator-nodes",
		-1,
		"Set number of orchestrator nodes",
	)
	flag.IntVar(&NumberOfComputeNodesFlag, "compute-nodes", -1, "Set number of compute nodes")
	flag.StringVar(
		&TargetRegionsFlag,
		"target-regions",
		"us-east-1",
		"Comma-separated list of target AWS regions",
	)
	flag.StringVar(
		&OrchestratorIPFlag,
		"orchestrator-ip",
		"",
		"IP address of existing orchestrator node",
	)
	flag.StringVar(&AWSProfileFlag, "aws-profile", "", "AWS profile to use for credentials")
	flag.StringVar(
		&InstanceTypeFlag,
		"instance-type",
		"t2.medium",
		"The instance type for both the compute and orchestrator nodes",
	)
	flag.StringVar(
		&ComputeInstanceTypeFlag,
		"compute-instance-type",
		"",
		"The instance type for the compute nodes. Overrides --instance-type for compute nodes.",
	)
	flag.StringVar(
		&OrchestratorInstanceTypeFlag,
		"orchestrator-instance-type",
		"",
		"The instance type for the orchestrator nodes. Overrides --instance-type for orchestrator nodes.",
	)
	flag.IntVar(
		&BootVolumeSizeFlag,
		"volume-size",
		8,
		"The volume size of each node created (Gigabytes). Default: 8",
	)
	flag.BoolVar(&helpFlag, "help", false, "Show help message")

	flag.Parse()

	if helpFlag {
		PrintUsage()
		os.Exit(0)
	}

	ProcessFlags()

	if command == "version" {
		fmt.Println(VersionNumber)
		os.Exit(0)
	}

	fmt.Print("\n== Andaime ==\n")

	if command == "create" {
		fmt.Println("Project configuration:")
		fmt.Println("=======================")
		fmt.Println("")
		fmt.Printf(
			"\tProject name: \"%s\" (set by %s)\n",
			ProjectSettings["ProjectName"],
			SetBy["ProjectName"],
		)
		fmt.Printf(
			"\tTarget Platform: \"%s\" (set by %s)\n",
			ProjectSettings["TargetPlatform"],
			SetBy["TargetPlatform"],
		)
		fmt.Printf(
			"\tNo. of Orchestrator Nodes: %d (set by %s)\n",
			ProjectSettings["NumberOfOrchestratorNodes"],
			SetBy["NumberOfOrchestratorNodes"],
		)
		fmt.Printf(
			"\tNo. of Compute Nodes: %d (set by %s)\n",
			ProjectSettings["NumberOfComputeNodes"],
			SetBy["NumberOfComputeNodes"],
		)
		fmt.Printf("\tAWS Profile: \"%s\"\n", AWSProfileFlag)
		fmt.Print("\n")
	}

	if command == "list" {
		fmt.Println("Listing resources...")
	}

	if command == "destroy" {
		fmt.Println("Destroying resources...")
	}

	if ProjectSettings["TargetPlatform"] == "aws" {
		DeployOnAWS()
	}
}
