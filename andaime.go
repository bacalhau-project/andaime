package main

import(
	"os"
	"fmt"
	"strconv"
	"flag"
	"encoding/json"
)

var (
    PROJECT_NAME string = "bacalhau-by-andaime"
    TARGET_PLATFORM string = "gcp"
    NUMBER_OF_ORCHESTRATOR_NODES  int = 1
    NUMBER_OF_COMPUTE_NODES       int = 2
)

var (
	PROJECT_NAME_SET_BY string = "default"
	TARGET_PLATFORM_SET_BY string = "default"
	NUMBER_OF_ORCHESTRATOR_NODES_SET_BY string = "default"
	NUMBER_OF_COMPUTE_NODES_SET_BY string = "default"
)

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

	var (
		VERBOSE_MODE_FLAG                       bool
		PROJECT_NAME_FLAG                  string
		TARGET_PLATFORM_FLAG               string
		NUMBER_OF_ORCHESTRATOR_NODES_FLAG  int
		NUMBER_OF_COMPUTE_NODES_FLAG       int
	)

	flag.BoolVar(&VERBOSE_MODE_FLAG, "verbose", false, "Generate verbose output throughout execution")
	flag.StringVar(&PROJECT_NAME_FLAG, "project-name", "", "Set project name")
    flag.StringVar(&TARGET_PLATFORM_FLAG, "target-platform", "", "Set target platform")
    flag.IntVar(&NUMBER_OF_ORCHESTRATOR_NODES_FLAG, "orchestrator-nodes", -1, "Set number of orchestrator nodes")
    flag.IntVar(&NUMBER_OF_COMPUTE_NODES_FLAG, "compute-nodes", -1, "Set number of compute nodes")

	flag.Parse()

	if os.Getenv("PROJECT_NAME") != ""{

		if VERBOSE_MODE_FLAG == true{
			fmt.Println(`Setting "PROJECT_NAME" from environment variable`)
		}

		PROJECT_NAME = os.Getenv("PROJECT_NAME")
		PROJECT_NAME_SET_BY = "environent variable"

	}

	if os.Getenv("TARGET_PLATFORM") != ""{
		if VERBOSE_MODE_FLAG == true{
			fmt.Println(`Setting "TARGET_PLATFORM" from environment variable`)
		}

		TARGET_PLATFORM = os.Getenv("TARGET_PLATFORM")
		TARGET_PLATFORM_SET_BY = "environent variable"

	}

	if os.Getenv("NUMBER_OF_ORCHESTRATOR_NODES") != ""{
		if VERBOSE_MODE_FLAG == true{	
			fmt.Println(`Setting "NUMBER_OF_ORCHESTRATOR_NODES" from environment variable`)
		}

		NUMBER_OF_ORCHESTRATOR_NODES, _ = strconv.Atoi( os.Getenv("NUMBER_OF_ORCHESTRATOR_NODES") )
		NUMBER_OF_ORCHESTRATOR_NODES_SET_BY = "environent variable"

	}

	if os.Getenv("NUMBER_OF_COMPUTE_NODES") != "" {
		
		if VERBOSE_MODE_FLAG == true{	
			fmt.Println(`Setting "NUMBER_OF_COMPUTE_NODES" from environment variable`)
		}

		NUMBER_OF_COMPUTE_NODES, _ = strconv.Atoi( os.Getenv("NUMBER_OF_COMPUTE_NODES") )
		NUMBER_OF_COMPUTE_NODES_SET_BY = "environent variable"

	}

	config_file, config_err := os.ReadFile("./config.json")

	if config_err != nil {
		fmt.Println("Could not read configuration file:", config_err)		
	}
	
	if config_file != nil{
		
		var configJson map[string]interface{}
	
		_ = json.Unmarshal(config_file, &configJson)

		if configJson["PROJECT_NAME"] != nil{
			
			if VERBOSE_MODE_FLAG == true{	
				fmt.Println(`Setting "PROJECT_NAME" from configuration file`)
			}

			PROJECT_NAME = configJson["PROJECT_NAME"].(string)
			PROJECT_NAME_SET_BY = "configuration file"

		}

		if configJson["TARGET_PLATFORM"] != nil{
			
			if VERBOSE_MODE_FLAG == true{	
				fmt.Println(`Setting "TARGET_PLATFORM" from configuration file`)
			}

			TARGET_PLATFORM = configJson["TARGET_PLATFORM"].(string)
			TARGET_PLATFORM_SET_BY = "configuration file"

		}

		if configJson["NUMBER_OF_ORCHESTRATOR_NODES"] != nil{

			if VERBOSE_MODE_FLAG == true{	
				fmt.Println(`Setting "NUMBER_OF_ORCHESTRATOR_NODES" from configuration file`)
			}

			NUMBER_OF_ORCHESTRATOR_NODES= int(configJson["NUMBER_OF_ORCHESTRATOR_NODES"].(float64))
			NUMBER_OF_ORCHESTRATOR_NODES_SET_BY = "configuration file"
		}

		if configJson["NUMBER_OF_COMPUTE_NODES"] != nil{

			if VERBOSE_MODE_FLAG == true{	
				fmt.Println(`Setting "NUMBER_OF_COMPUTE_NODES" from configuration file`)
			}

			NUMBER_OF_COMPUTE_NODES = int(configJson["NUMBER_OF_COMPUTE_NODES"].(float64))
			NUMBER_OF_COMPUTE_NODES_SET_BY = "configuration file"

		}

	}

	if PROJECT_NAME_FLAG != ""{

		if VERBOSE_MODE_FLAG == true{	
			fmt.Println(`Setting "PROJECT_NAME" by flag`)
		}

		PROJECT_NAME = PROJECT_NAME_FLAG
		PROJECT_NAME_SET_BY = "flag --project-name"

	}
	
	if TARGET_PLATFORM_FLAG != ""{
		
		if VERBOSE_MODE_FLAG == true{	
			fmt.Println(`Setting "TARGET_PLATFORM" by flag`)
		}

		TARGET_PLATFORM = TARGET_PLATFORM_FLAG
		TARGET_PLATFORM_SET_BY = "flag --target-platform"

	}

	if NUMBER_OF_ORCHESTRATOR_NODES_FLAG != -1{
		if VERBOSE_MODE_FLAG == true{	
			fmt.Println(`Setting "NUMBER_OF_ORCHESTRATOR_NODES" by flag`)
		}

		NUMBER_OF_ORCHESTRATOR_NODES = NUMBER_OF_ORCHESTRATOR_NODES_FLAG
		NUMBER_OF_ORCHESTRATOR_NODES_SET_BY = "flag --orchestrator-nodes"

	}

	if NUMBER_OF_COMPUTE_NODES_FLAG != -1{

		if VERBOSE_MODE_FLAG == true{	
			fmt.Println(`Setting "NUMBER_OF_COMPUTE_NODES_FLAG" by flag`)
		}

		NUMBER_OF_COMPUTE_NODES = NUMBER_OF_COMPUTE_NODES_FLAG
		NUMBER_OF_COMPUTE_NODES_SET_BY = "flag --compute-nodes"
	}

	fmt.Println("Project configuration:\n")
	fmt.Printf("\tProject name: \"%s\" (set by %s)\n", PROJECT_NAME, PROJECT_NAME_SET_BY )
	fmt.Printf("\tTarget Platform: \"%s\" (set by %s)\n", TARGET_PLATFORM, TARGET_PLATFORM_SET_BY )
	fmt.Printf("\tNo. of Orchestrator Nodes: %s (set by %s)\n", strconv.Itoa(NUMBER_OF_ORCHESTRATOR_NODES), NUMBER_OF_ORCHESTRATOR_NODES_SET_BY )
	fmt.Printf("\tNo. of Compute Nodes: %s (set by %s)\n", strconv.Itoa(NUMBER_OF_COMPUTE_NODES), NUMBER_OF_COMPUTE_NODES_SET_BY )
	fmt.Print("\n")

}