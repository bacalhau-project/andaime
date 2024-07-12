package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Struct to hold instance information
type InstanceInfo struct {
	InstanceID string
	Region     string
	PublicIP   string
}

var (
	VERBOSE_MODE_FLAG bool = false
	PROJECT_SETTINGS      = map[string]interface{}{
		"ProjectName":              "bacalhau-by-andaime",
		"TargetPlatform":           "aws",
		"NumberOfOrchestratorNodes": 1,
		"NumberOfComputeNodes":      2,
	}

	SET_BY = map[string]string{
		"ProjectName":              "default",
		"TargetPlatform":           "default",
		"NumberOfOrchestratorNodes": "default",
		"NumberOfComputeNodes":      "default",
	}
	PROJECT_NAME_FLAG                  string
	TARGET_PLATFORM_FLAG               string
	NUMBER_OF_ORCHESTRATOR_NODES_FLAG  int
	NUMBER_OF_COMPUTE_NODES_FLAG       int
	TARGET_REGIONS_FLAG                string
	command                            string
	helpFlag                           bool
	AWS_PROFILE_FLAG                   string
)

func getUbuntuAMIId(svc *ec2.EC2) (string, error) {
	describeImagesInput := &ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("name"),
				Values: aws.StringSlice([]string{"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"}),
			},
			{
				Name:   aws.String("architecture"),
				Values: aws.StringSlice([]string{"x86_64"}),
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
		fmt.Printf("Failed to describe images, %v", err)
		return "", err
	}

	if len(result.Images) == 0 {
		fmt.Println("No Ubuntu 22.04 AMIs found")
		return "", err
	}

	// Get the latest image (you might want to add more logic to select the right one)
	latestImage := result.Images[0]
	for _, image := range result.Images {
		if *image.CreationDate > *latestImage.CreationDate {
			latestImage = image
		}
	}

	fmt.Printf("Using AMI ID: %s\n", *latestImage.ImageId)
	return *latestImage.ImageId, nil
}

func DeployOnAWS() {
	targetRegions := strings.Split(TARGET_REGIONS_FLAG, ",")
	noOfInstances := PROJECT_SETTINGS["NumberOfComputeNodes"].(int) + PROJECT_SETTINGS["NumberOfOrchestratorNodes"].(int)

	if command == "create" {
		createResources(targetRegions, noOfInstances)
	} else if command == "destroy" {
		destroyResources()
	} else if command == "list" {
		listResources()
	} else {
		fmt.Println("Unknown command. Use 'create', 'destroy', or 'list'.")
	}
}

func createResources(regions []string, instanceCount int) {
	var wg sync.WaitGroup

	// Create VPCs and Security Groups in all regions first
	for _, region := range regions {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
			sess := session.Must(session.NewSessionWithOptions(session.Options{
				Profile: AWS_PROFILE_FLAG,
				Config:  aws.Config{Region: aws.String(region)},
			}))
			ec2Svc := ec2.New(sess)
			instanceType := "t2.medium"
			az, err := getAvailableZoneForInstanceType(ec2Svc, instanceType)
			if err != nil {
				fmt.Printf("Instance type %s is not available in region %s: %v\n", instanceType, region, err)
				return
			}
			createVPCAndSG(ec2Svc, region, az)
		}(region)
	}

	wg.Wait()
	fmt.Println("VPCs and Security Groups created in all regions.")

	// Create EC2 instances in a round-robin fashion
	createInstancesRoundRobin(regions, instanceCount)
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
				time.Sleep(2 * time.Second)
				continue
			}
			fmt.Printf("Unable to describe VPCs in region %s: %v\n", region, err)
			return
		}

		var vpcID *string

		if len(vpcs.Vpcs) > 0 {
			fmt.Printf("VPC already exists in region %s\n", region)
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
					time.Sleep(2 * time.Second)
					continue
				}
				fmt.Printf("Unable to create VPC in region %s: %v\n", region, err)
				return
			}

			vpcID = vpcOutput.Vpc.VpcId
			fmt.Printf("Created VPC in region %s with ID %s\n", region, *vpcID)

			// Create Subnet
			subnetOutput, err := svc.CreateSubnet(&ec2.CreateSubnetInput{
				CidrBlock: aws.String("10.0.1.0/24"),
				VpcId:     vpcID,
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
					time.Sleep(2 * time.Second)
					continue
				}
				fmt.Printf("Unable to create subnet in region %s: %v\n", region, err)
				return
			}
			subnetID := subnetOutput.Subnet.SubnetId
			fmt.Printf("Created Subnet in region %s with ID %s\n", region, *subnetID)

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
					time.Sleep(2 * time.Second)
					continue
				}
				fmt.Printf("Unable to create internet gateway in region %s: %v\n", region, err)
				return
			}
			igwID := igwOutput.InternetGateway.InternetGatewayId
			fmt.Printf("Created Internet Gateway in region %s with ID %s\n", region, *igwID)

			// Attach Internet Gateway to VPC
			_, err = svc.AttachInternetGateway(&ec2.AttachInternetGatewayInput{
				InternetGatewayId: igwID,
				VpcId:             vpcID,
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(2 * time.Second)
					continue
				}
				fmt.Printf("Unable to attach internet gateway to VPC in region %s: %v\n", region, err)
				return
			}
			fmt.Printf("Attached Internet Gateway to VPC in region %s\n", region)

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
					time.Sleep(2 * time.Second)
					continue
				}
				fmt.Printf("Unable to create route table in region %s: %v\n", region, err)
				return
			}
			routeTableID := routeTableOutput.RouteTable.RouteTableId
			fmt.Printf("Created Route Table in region %s with ID %s\n", region, *routeTableID)

			// Create Route to Internet Gateway
			_, err = svc.CreateRoute(&ec2.CreateRouteInput{
				RouteTableId:         routeTableID,
				DestinationCidrBlock: aws.String("0.0.0.0/0"),
				GatewayId:            igwID,
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(2 * time.Second)
					continue
				}
				fmt.Printf("Unable to create route to internet gateway in region %s: %v\n", region, err)
				return
			}
			fmt.Printf("Created route to Internet Gateway in region %s\n", region)

			// Associate Subnet with Route Table
			_, err = svc.AssociateRouteTable(&ec2.AssociateRouteTableInput{
				RouteTableId: routeTableID,
				SubnetId:     subnetID,
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(2 * time.Second)
					continue
				}
				fmt.Printf("Unable to associate subnet with route table in region %s: %v\n", region, err)
				return
			}
			fmt.Printf("Associated Subnet with Route Table in region %s\n", region)
		}

		// Create Security Group if it doesn't exist
		createSecurityGroupIfNotExists(svc, vpcID, region)
		break
	}
}

func createInstancesRoundRobin(regions []string, instanceCount int) {
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
			sess := session.Must(session.NewSessionWithOptions(session.Options{
				Profile: AWS_PROFILE_FLAG,
				Config:  aws.Config{Region: aws.String(region)},
			}))
			ec2Svc := ec2.New(sess)
			instanceInfo := createInstanceInRegion(ec2Svc, region)
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

	fmt.Println("Created instances:")
	for _, instance := range instances {
		fmt.Printf("Region: %s, Instance ID: %s, Public IPv4: %s\n", instance.Region, instance.InstanceID, instance.PublicIP)
	}
}

func createInstanceInRegion(svc *ec2.EC2, region string) InstanceInfo {
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
				time.Sleep(2 * time.Second)
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
				time.Sleep(2 * time.Second)
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
				time.Sleep(2 * time.Second)
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

		// Get the Ubuntu 22.04 AMI ID
		amiID, err := getUbuntuAMIId(svc)
		if err != nil {
			if isRetryableError(err) && i < retryPolicy-1 {
				time.Sleep(2 * time.Second)
				continue
			}
			fmt.Printf("Unable to find Ubuntu 22.04 AMI in region %s: %v\n", region, err)
			return instanceInfo
		}

		// Read and encode startup scripts
		userData, err := readStartupScripts("startup_scripts")
		if err != nil {
			fmt.Printf("Unable to read startup scripts: %v\n", err)
			return instanceInfo
		}
		encodedUserData := base64.StdEncoding.EncodeToString([]byte(userData))

		// Create EC2 Instance
		runResult, err := svc.RunInstances(&ec2.RunInstancesInput{
			ImageId:      aws.String(amiID),
			InstanceType: aws.String("t2.medium"),
			MaxCount:     aws.Int64(1),
			MinCount:     aws.Int64(1),
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
							Value: aws.String("Bacalhau-Node"),
						},
					},
				},
			},
		})
		if err != nil {
			if isRetryableError(err) && i < retryPolicy-1 {
				time.Sleep(2 * time.Second)
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
			fmt.Printf("Error waiting for instance %s to run in region %s: %v\n", instanceID, region, err)
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
				time.Sleep(2 * time.Second)
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
				time.Sleep(2 * time.Second)
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
					time.Sleep(2 * time.Second)
					continue
				}
				fmt.Printf("Unable to authorize inbound traffic on port %d in region %s: %v\n", port, region, err)
				return nil
			}
		}

		fmt.Printf("Created security group in region %s\n", region)
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
			sess := session.Must(session.NewSessionWithOptions(session.Options{
				Profile: AWS_PROFILE_FLAG,
				Config:  aws.Config{Region: aws.String(region)},
			}))
			ec2Svc := ec2.New(sess)
			deleteTaggedResources(ec2Svc, region)
		}(region)
	}

	wg.Wait()
	fmt.Println("Resources destroyed in all regions.")
}

func getAllRegions() ([]string, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Profile: AWS_PROFILE_FLAG,
		Config:  aws.Config{Region: aws.String("us-east-1")},
	}))
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
				time.Sleep(2 * time.Second)
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
					time.Sleep(2 * time.Second)
					continue
				}
				fmt.Printf("Unable to terminate instances in region %s: %v\n", region, err)
				return
			}
			fmt.Printf("Terminated instances in region %s\n", region)

			// Wait until instances are terminated
			err = svc.WaitUntilInstanceTerminated(&ec2.DescribeInstancesInput{
				InstanceIds: instanceIds,
			})
			if err != nil {
				fmt.Printf("Error waiting for instances to terminate in region %s: %v\n", region, err)
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
				time.Sleep(2 * time.Second)
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
					time.Sleep(2 * time.Second)
					continue
				}
				fmt.Printf("Unable to delete security group %s in region %s: %v\n", *sg.GroupId, region, err)
				continue
			}
			fmt.Printf("Deleted security group %s in region %s\n", *sg.GroupId, region)
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
				time.Sleep(2 * time.Second)
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
				fmt.Printf("Unable to describe subnets for VPC %s in region %s: %v\n", *vpc.VpcId, region, err)
				continue
			}
			for _, subnet := range subnets.Subnets {
				_, err = svc.DeleteSubnet(&ec2.DeleteSubnetInput{
					SubnetId: subnet.SubnetId,
				})
				if err != nil {
					fmt.Printf("Unable to delete subnet %s in region %s: %v\n", *subnet.SubnetId, region, err)
					continue
				}
				fmt.Printf("Deleted subnet %s in region %s\n", *subnet.SubnetId, region)
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
				fmt.Printf("Unable to describe route tables for VPC %s in region %s: %v\n", *vpc.VpcId, region, err)
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
					fmt.Printf("Unable to delete route table %s in region %s: %v\n", *routeTable.RouteTableId, region, err)
					continue
				}
				fmt.Printf("Deleted route table %s in region %s\n", *routeTable.RouteTableId, region)
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
				fmt.Printf("Unable to describe internet gateways for VPC %s in region %s: %v\n", *vpc.VpcId, region, err)
				continue
			}
			for _, igw := range igws.InternetGateways {
				_, err = svc.DetachInternetGateway(&ec2.DetachInternetGatewayInput{
					InternetGatewayId: igw.InternetGatewayId,
					VpcId:             vpc.VpcId,
				})
				if err != nil {
					fmt.Printf("Unable to detach internet gateway %s from VPC %s in region %s: %v\n", *igw.InternetGatewayId, *vpc.VpcId, region, err)
					continue
				}
				fmt.Printf("Detached internet gateway %s from VPC %s in region %s\n", *igw.InternetGatewayId, *vpc.VpcId, region)

				// Delete Internet Gateway
				_, err = svc.DeleteInternetGateway(&ec2.DeleteInternetGatewayInput{
					InternetGatewayId: igw.InternetGatewayId,
				})
				if err != nil {
					fmt.Printf("Unable to delete internet gateway %s in region %s: %v\n", *igw.InternetGatewayId, region, err)
					continue
				}
				fmt.Printf("Deleted internet gateway %s in region %s\n", *igw.InternetGatewayId, region)
			}

			_, err = svc.DeleteVpc(&ec2.DeleteVpcInput{
				VpcId: vpc.VpcId,
			})
			if err != nil {
				if isRetryableError(err) && i < retryPolicy-1 {
					time.Sleep(2 * time.Second)
					continue
				}
				fmt.Printf("Unable to delete VPC %s in region %s: %v\n", *vpc.VpcId, region, err)
				continue
			}
			fmt.Printf("Deleted VPC %s in region %s\n", *vpc.VpcId, region)
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

	resourceTypes := []string{"VPC", "Subnet", "Internet Gateway", "Route Table", "Security Group", "Instance"}
	resourcesByRegion := make(map[string]map[string][]string)

	var wg sync.WaitGroup
	totalNodes := 0
	totalRegions := 0

	for _, region := range regions {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
			sess := session.Must(session.NewSessionWithOptions(session.Options{
				Profile: AWS_PROFILE_FLAG,
				Config:  aws.Config{Region: aws.String(region)},
			}))
			ec2Svc := ec2.New(sess)

			resources := make(map[string][]string)
			for _, resourceType := range resourceTypes {
				resources[resourceType] = listTaggedResources(ec2Svc, resourceType, region)
			}

			if len(resources["VPC"]) > 0 || len(resources["Subnet"]) > 0 || len(resources["Internet Gateway"]) > 0 || len(resources["Route Table"]) > 0 || len(resources["Security Group"]) > 0 || len(resources["Instance"]) > 0 {
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
		fmt.Println("=======================\n")

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
					time.Sleep(2 * time.Second)
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
					time.Sleep(2 * time.Second)
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
					time.Sleep(2 * time.Second)
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
					time.Sleep(2 * time.Second)
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
					time.Sleep(2 * time.Second)
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
					time.Sleep(2 * time.Second)
					continue
				}
				fmt.Printf("Unable to describe instances in region %s: %v\n", region, err)
				return nil
			}
			for _, reservation := range instances.Reservations {
				for _, instance := range reservation.Instances {
					resourceList = append(resourceList, fmt.Sprintf("ID: %s, Type: %s, IPv4: %s", *instance.InstanceId, *instance.InstanceType, *instance.PublicIpAddress))
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

func readStartupScripts(dir string) (string, error) {
	var combinedScript strings.Builder

	files, err := ioutil.ReadDir(dir)
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
		fmt.Println(orderedFile.name)
		content, err := ioutil.ReadFile(filepath.Join(dir, orderedFile.name))
		if err != nil {
			return "", err
		}
		combinedScript.Write(content)
		combinedScript.WriteString("\n")
	}

	return combinedScript.String(), nil
}


func ProcessEnvVars() {

	if os.Getenv("PROJECT_NAME") != "" {

		if VERBOSE_MODE_FLAG == true {
			fmt.Println(`Setting "PROJECT_NAME" from environment variable`)
		}

		PROJECT_SETTINGS["ProjectName"] = os.Getenv("PROJECT_NAME")
		SET_BY["ProjectName"] = "environment variable"

	}

	if os.Getenv("TARGET_PLATFORM") != "" {
		if VERBOSE_MODE_FLAG == true {
			fmt.Println(`Setting "TARGET_PLATFORM" from environment variable`)
		}

		PROJECT_SETTINGS["TargetPlatform"] = os.Getenv("TARGET_PLATFORM")
		SET_BY["TargetPlatform"] = "environment variable"

	}

	if os.Getenv("NUMBER_OF_ORCHESTRATOR_NODES") != "" {
		if VERBOSE_MODE_FLAG == true {
			fmt.Println(`Setting "NUMBER_OF_ORCHESTRATOR_NODES" from environment variable`)
		}

		PROJECT_SETTINGS["NumberOfOrchestratorNodes"], _ = strconv.Atoi(os.Getenv("NUMBER_OF_ORCHESTRATOR_NODES"))
		SET_BY["NumberOfOrchestratorNodes"] = "environment variable"

	}

	if os.Getenv("NUMBER_OF_COMPUTE_NODES") != "" {

		if VERBOSE_MODE_FLAG == true {
			fmt.Println(`Setting "NUMBER_OF_COMPUTE_NODES" from environment variable`)
		}

		PROJECT_SETTINGS["NumberOfComputeNodes"], _ = strconv.Atoi(os.Getenv("NUMBER_OF_COMPUTE_NODES"))
		SET_BY["NumberOfComputeNodes"] = "environment variable"

	}

}

func ProcessConfigFile() error {

	_, err := os.Stat("./config.json")

	if os.IsNotExist(err) {

		if VERBOSE_MODE_FLAG == true {
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

			if VERBOSE_MODE_FLAG == true {
				fmt.Println(`Setting "PROJECT_NAME" from configuration file`)
			}

			PROJECT_SETTINGS["ProjectName"] = configJson["PROJECT_NAME"].(string)
			SET_BY["ProjectName"] = "configuration file"

		}

		if configJson["TARGET_PLATFORM"] != nil {

			if VERBOSE_MODE_FLAG == true {
				fmt.Println(`Setting "TARGET_PLATFORM" from configuration file`)
			}

			PROJECT_SETTINGS["TargetPlatform"] = configJson["TARGET_PLATFORM"].(string)
			SET_BY["TargetPlatform"] = "configuration file"

		}

		if configJson["NUMBER_OF_ORCHESTRATOR_NODES"] != nil {

			if VERBOSE_MODE_FLAG == true {
				fmt.Println(`Setting "NUMBER_OF_ORCHESTRATOR_NODES" from configuration file`)
			}

			PROJECT_SETTINGS["NumberOfOrchestratorNodes"] = int(configJson["NUMBER_OF_ORCHESTRATOR_NODES"].(float64))
			SET_BY["NumberOfOrchestratorNodes"] = "configuration file"
		}

		if configJson["NUMBER_OF_COMPUTE_NODES"] != nil {

			if VERBOSE_MODE_FLAG == true {
				fmt.Println(`Setting "NUMBER_OF_COMPUTE_NODES" from configuration file`)
			}

			PROJECT_SETTINGS["NumberOfComputeNodes"] = int(configJson["NUMBER_OF_COMPUTE_NODES"].(float64))
			SET_BY["NumberOfComputeNodes"] = "configuration file"

		}

	}

	return config_err

}

func ProcessFlags() {

	if PROJECT_NAME_FLAG != "" {

		if VERBOSE_MODE_FLAG == true {
			fmt.Println(`Setting "PROJECT_NAME" by flag`)
		}

		PROJECT_SETTINGS["ProjectName"] = PROJECT_NAME_FLAG
		SET_BY["ProjectName"] = "flag --project-name"

	}

	if TARGET_PLATFORM_FLAG != "" {

		if VERBOSE_MODE_FLAG == true {
			fmt.Println(`Setting "TARGET_PLATFORM" by flag`)
		}

		PROJECT_SETTINGS["TargetPlatform"] = TARGET_PLATFORM_FLAG
		SET_BY["TargetPlatform"] = "flag --target-platform"

	}

	if NUMBER_OF_ORCHESTRATOR_NODES_FLAG != -1 {
		if VERBOSE_MODE_FLAG == true {
			fmt.Println(`Setting "NUMBER_OF_ORCHESTRATOR_NODES" by flag`)
		}

		PROJECT_SETTINGS["NumberOfOrchestratorNodes"] = NUMBER_OF_ORCHESTRATOR_NODES_FLAG
		SET_BY["NumberOfOrchestratorNodes"] = "flag --orchestrator-nodes"

	}

	if NUMBER_OF_COMPUTE_NODES_FLAG != -1 {

		if VERBOSE_MODE_FLAG == true {
			fmt.Println(`Setting "NUMBER_OF_COMPUTE_NODES_FLAG" by flag`)
		}

		PROJECT_SETTINGS["NumberOfComputeNodes"] = NUMBER_OF_COMPUTE_NODES_FLAG
		SET_BY["NumberOfComputeNodes"] = "flag --compute-nodes"
	}

}

func PrintUsage() {
	fmt.Println("Usage: ./main <create|destroy|list> [options]")
	fmt.Println("Commands:")
	fmt.Println("  create              Create AWS resources")
	fmt.Println("  destroy             Destroy AWS resources")
	fmt.Println("  list                List AWS resources tagged with 'project: andaime'")
	fmt.Println("Options:")
	flag.PrintDefaults()
}

func main() {
	fmt.Println("\n== Andaime ==\n")

	if len(os.Args) < 2 {
		PrintUsage()
		os.Exit(1)
	}

	command = os.Args[1]
	if command == "--help" || command == "-h" {
		PrintUsage()
		os.Exit(0)
	}

	os.Args = append(os.Args[:1], os.Args[2:]...) // Remove the command from the args

	ProcessEnvVars()
	configErr := ProcessConfigFile()

	if configErr != nil {
		fmt.Println("Error reading configuration file:", configErr)
	}

	flag.BoolVar(&VERBOSE_MODE_FLAG, "verbose", false, "Generate verbose output throughout execution")
	flag.StringVar(&PROJECT_NAME_FLAG, "project-name", "", "Set project name")
	flag.StringVar(&TARGET_PLATFORM_FLAG, "target-platform", "", "Set target platform")
	flag.IntVar(&NUMBER_OF_ORCHESTRATOR_NODES_FLAG, "orchestrator-nodes", -1, "Set number of orchestrator nodes")
	flag.IntVar(&NUMBER_OF_COMPUTE_NODES_FLAG, "compute-nodes", -1, "Set number of compute nodes")
	flag.StringVar(&TARGET_REGIONS_FLAG, "target-regions", "us-east-1", "Comma-separated list of target AWS regions")
	flag.StringVar(&AWS_PROFILE_FLAG, "aws-profile", "default", "AWS profile to use for credentials")
	flag.BoolVar(&helpFlag, "help", false, "Show help message")

	flag.Parse()

	if helpFlag {
		PrintUsage()
		os.Exit(0)
	}

	ProcessFlags()

	if command == "create" {

		fmt.Println("Project configuration:\n")
		fmt.Printf("\tProject name: \"%s\" (set by %s)\n", PROJECT_SETTINGS["ProjectName"], SET_BY["ProjectName"])
		fmt.Printf("\tTarget Platform: \"%s\" (set by %s)\n", PROJECT_SETTINGS["TargetPlatform"], SET_BY["TargetPlatform"])
		fmt.Printf("\tNo. of Orchestrator Nodes: %d (set by %s)\n", PROJECT_SETTINGS["NumberOfOrchestratorNodes"], SET_BY["NumberOfOrchestratorNodes"])
		fmt.Printf("\tNo. of Compute Nodes: %d (set by %s)\n", PROJECT_SETTINGS["NumberOfComputeNodes"], SET_BY["NumberOfComputeNodes"])
		fmt.Printf("\tAWS Profile: \"%s\"\n", AWS_PROFILE_FLAG)
		fmt.Print("\n")

	}

	if command == "list" {
		fmt.Println("Listing resources...")
	}

	if command == "destroy" {
		fmt.Println("Destroying resources...")
	}

	if PROJECT_SETTINGS["TargetPlatform"] == "aws" {
		DeployOnAWS()
	}
}
