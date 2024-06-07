package main

import(
	"os"
	"fmt"
	"strconv"
	"flag"
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/ec2"
)

var (
	VERBOSE_MODE_FLAG bool = false
    PROJECT_SETTINGS = map[string]interface{}{
        "ProjectName":             "bacalhau-by-andaime",
        "TargetPlatform":          "gcp",
        "NumberOfOrchestratorNodes":  1,
        "NumberOfComputeNodes":       2,
    }

    SET_BY = map[string]string{
        "ProjectName":             "default",
        "TargetPlatform":          "default",
        "NumberOfOrchestratorNodes":  "default",
        "NumberOfComputeNodes":       "default",
    }
	PROJECT_NAME_FLAG                  string
	TARGET_PLATFORM_FLAG               string
	NUMBER_OF_ORCHESTRATOR_NODES_FLAG  int
	NUMBER_OF_COMPUTE_NODES_FLAG       int
)

func getAmazonLinuxAMIId(svc *ec2.EC2) (string, error) {

	describeImagesInput := &ec2.DescribeImagesInput{
        Filters: []*ec2.Filter{
            {
                Name:   aws.String("name"),
                Values: aws.StringSlice([]string{"amzn2-ami-hvm-*-arm64-gp2"}),
            },
            {
                Name:   aws.String("architecture"),
                Values: aws.StringSlice([]string{"arm64"}),
            },
            {
                Name:   aws.String("state"),
                Values: aws.StringSlice([]string{"available"}),
            },
        },
        Owners: aws.StringSlice([]string{"amazon"}),
    }

    // Call DescribeImages to find matching AMIs
    result, err := svc.DescribeImages(describeImagesInput)
    if err != nil {
        fmt.Println("failed to describe images, %v", err)
		return "", err
    }

    if len(result.Images) == 0 {
        fmt.Println("no Amazon Linux 2023 AMIs found")
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

func DeployOnAWS(){
	// Create a new session using the default configuration
    sess, err := session.NewSession(&aws.Config{
        Region: aws.String("us-east-1"), // specify the region you want
    })
    if err != nil {
        fmt.Println("failed to create session, %v", err)
		os.Exit(1)

    }

    svc := ec2.New(sess)

    blockDeviceMappings := []*ec2.BlockDeviceMapping{
        {
            DeviceName: aws.String("/dev/xvda"),
            Ebs: &ec2.EbsBlockDevice{
                VolumeSize:          aws.Int64(20),
                VolumeType:          aws.String("gp2"),
                DeleteOnTermination: aws.Bool(true),
            },
        },
    }

	amazonLinuxAMI, amiErr := getAmazonLinuxAMIId(svc)

	if amiErr != nil {
		fmt.Println("Could not find AMI ID for Amazon Linux")
		os.Exit(1)
	}

	fmt.Println(amazonLinuxAMI)
	os.Exit(0)

	// TODO:
	// Get default VPC for target region
	// Get Subnets for found VPC

    // Define the instance parameters
    runInstancesInput := &ec2.RunInstancesInput{
        ImageId:             aws.String(amazonLinuxAMI),
        InstanceType:        aws.String("t4g.nano"),
        MinCount:            aws.Int64(1),
        MaxCount:            aws.Int64(1),
        BlockDeviceMappings: blockDeviceMappings,
        NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
            {
                AssociatePublicIpAddress: aws.Bool(true),
                DeviceIndex:              aws.Int64(0),
                SubnetId:                 aws.String(""),
            },
        },
        KeyName: aws.String("my-key-pair"), // replace with your key pair
    }

    // Run the instance
    result, err := svc.RunInstances(runInstancesInput)
    if err != nil {
        fmt.Println("failed to run instance, %v", err)
		os.Exit(1)
    }

    // Print the instance ID of the newly created instance
    for _, instance := range result.Instances {
        fmt.Printf("Successfully launched instance %s\n", *instance.InstanceId)
    }
}

func ProcessEnvVars(){

	if os.Getenv("PROJECT_NAME") != ""{

		if VERBOSE_MODE_FLAG == true{
			fmt.Println(`Setting "PROJECT_NAME" from environment variable`)
		}

		PROJECT_SETTINGS["ProjectName"] = os.Getenv("PROJECT_NAME")
		SET_BY["ProjectName"] = "environent variable"

	}

	if os.Getenv("TARGET_PLATFORM") != ""{
		if VERBOSE_MODE_FLAG == true{
			fmt.Println(`Setting "TARGET_PLATFORM" from environment variable`)
		}

		PROJECT_SETTINGS["TargetPlatform"] = os.Getenv("TARGET_PLATFORM")
		SET_BY["TargetPlatform"] = "environent variable"

	}

	if os.Getenv("NUMBER_OF_ORCHESTRATOR_NODES") != ""{
		if VERBOSE_MODE_FLAG == true{	
			fmt.Println(`Setting "NUMBER_OF_ORCHESTRATOR_NODES" from environment variable`)
		}

		PROJECT_SETTINGS["NumberOfOrchestratorNodes"], _ = strconv.Atoi( os.Getenv("NUMBER_OF_ORCHESTRATOR_NODES") )
		SET_BY["NumberOfOrchestratorNodes"] = "environent variable"

	}

	if os.Getenv("NUMBER_OF_COMPUTE_NODES") != "" {
		
		if VERBOSE_MODE_FLAG == true{	
			fmt.Println(`Setting "NUMBER_OF_COMPUTE_NODES" from environment variable`)
		}

		PROJECT_SETTINGS["NumberOfComputeNodes"], _ = strconv.Atoi( os.Getenv("NUMBER_OF_COMPUTE_NODES") )
		SET_BY["NumberOfComputeNodes"] = "environent variable"

	}

}

func ProcessConfigFile() error{
	
	_, err := os.Stat("./config.json")

	if os.IsNotExist(err){
		
		if VERBOSE_MODE_FLAG == true{
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

		if config_err != nil{
			return config_err
		}

		if configJson["PROJECT_NAME"] != nil{
			
			if VERBOSE_MODE_FLAG == true{	
				fmt.Println(`Setting "PROJECT_NAME" from configuration file`)
			}

			PROJECT_SETTINGS["ProjectName"] = configJson["PROJECT_NAME"].(string)
			SET_BY["ProjectName"] = "configuration file"

		}

		if configJson["TARGET_PLATFORM"] != nil{
			
			if VERBOSE_MODE_FLAG == true{	
				fmt.Println(`Setting "TARGET_PLATFORM" from configuration file`)
			}

			PROJECT_SETTINGS["TargetPlatform"] = configJson["TARGET_PLATFORM"].(string)
			SET_BY["TargetPlatform"] = "configuration file"

		}

		if configJson["NUMBER_OF_ORCHESTRATOR_NODES"] != nil{

			if VERBOSE_MODE_FLAG == true{	
				fmt.Println(`Setting "NUMBER_OF_ORCHESTRATOR_NODES" from configuration file`)
			}

			PROJECT_SETTINGS["NumberOfOrchestratorNodes"] = int(configJson["NUMBER_OF_ORCHESTRATOR_NODES"].(float64))
			SET_BY["NumberOfOrchestratorNodes"] = "configuration file"
		}

		if configJson["NUMBER_OF_COMPUTE_NODES"] != nil{

			if VERBOSE_MODE_FLAG == true{	
				fmt.Println(`Setting "NUMBER_OF_COMPUTE_NODES" from configuration file`)
			}

			PROJECT_SETTINGS["NumberOfComputeNodes"] = int(configJson["NUMBER_OF_COMPUTE_NODES"].(float64))
			SET_BY["NumberOfComputeNodes"] = "configuration file"

		}

	}

	return config_err

}

func ProcessFlags(){
	
	if PROJECT_NAME_FLAG != ""{

		if VERBOSE_MODE_FLAG == true{	
			fmt.Println(`Setting "PROJECT_NAME" by flag`)
		}

		PROJECT_SETTINGS["ProjectName"] = PROJECT_NAME_FLAG
		SET_BY["ProjectName"] = "flag --project-name"

	}
	
	if TARGET_PLATFORM_FLAG != ""{
		
		if VERBOSE_MODE_FLAG == true{	
			fmt.Println(`Setting "TARGET_PLATFORM" by flag`)
		}

		PROJECT_SETTINGS["TargetPlatform"] = TARGET_PLATFORM_FLAG
		SET_BY["TargetPlatform"] = "flag --target-platform"

	}

	if NUMBER_OF_ORCHESTRATOR_NODES_FLAG != -1{
		if VERBOSE_MODE_FLAG == true{	
			fmt.Println(`Setting "NUMBER_OF_ORCHESTRATOR_NODES" by flag`)
		}

		PROJECT_SETTINGS["NumberOfOrchestratorNodes"] = NUMBER_OF_ORCHESTRATOR_NODES_FLAG
		SET_BY["NumberOfOrchestratorNodes"] = "flag --orchestrator-nodes"

	}

	if NUMBER_OF_COMPUTE_NODES_FLAG != -1{

		if VERBOSE_MODE_FLAG == true{	
			fmt.Println(`Setting "NUMBER_OF_COMPUTE_NODES_FLAG" by flag`)
		}

		PROJECT_SETTINGS["NumberOfComputeNodes"] = NUMBER_OF_COMPUTE_NODES_FLAG
		SET_BY["NumberOfComputeNodes"] = "flag --compute-nodes"
	}

}

func main(){
	fmt.Println("\n== Andaime ==\n")

	// Order of business:
	// 1. Check whether there are any env vars in the existing directory	
	// 1a. If so, set the appropriate parameters in the project
	// 2. Check if there is a config file
	// 2a. If so, overwrite any env vars, and set remaining parameters from file
	// 3. Check if there are any flags set on the binary
	// 3a. If so, overwrite all prev. values, and set from flags.
	// 4. If key variables are missing, start the questions


	// Parse flags first, but set later so we can check whether
	// or not the user has passed the --verbose flag

	ProcessEnvVars()
	configErr := ProcessConfigFile()

	if configErr != nil{
		fmt.Println("Error reading configuration file:", configErr)
	}

	flag.BoolVar(&VERBOSE_MODE_FLAG, "verbose", false, "Generate verbose output throughout execution")
	flag.StringVar(&PROJECT_NAME_FLAG, "project-name", "", "Set project name")
    flag.StringVar(&TARGET_PLATFORM_FLAG, "target-platform", "", "Set target platform")
    flag.IntVar(&NUMBER_OF_ORCHESTRATOR_NODES_FLAG, "orchestrator-nodes", -1, "Set number of orchestrator nodes")
    flag.IntVar(&NUMBER_OF_COMPUTE_NODES_FLAG, "compute-nodes", -1, "Set number of compute nodes")

	flag.Parse()
	
	ProcessFlags()

	fmt.Println("Project configuration:\n")
	fmt.Printf("\tProject name: \"%s\" (set by %s)\n", PROJECT_SETTINGS["ProjectName"], SET_BY["ProjectName"] )
	fmt.Printf("\tTarget Platform: \"%s\" (set by %s)\n", PROJECT_SETTINGS["TargetPlatform"], SET_BY["TargetPlatform"] )
	fmt.Printf("\tNo. of Orchestrator Nodes: %d (set by %s)\n", PROJECT_SETTINGS["NumberOfOrchestratorNodes"], SET_BY["NumberOfOrchestratorNodes"])
	fmt.Printf("\tNo. of Compute Nodes: %d (set by %s)\n", PROJECT_SETTINGS["NumberOfComputeNodes"], SET_BY["NumberOfComputeNodes"] )
	fmt.Print("\n")

	DeployOnAWS()

}