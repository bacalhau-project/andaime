package main

import(
	"os"
	"fmt"
	"strconv"
	"flag"
	"encoding/json"
)

var (
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

	fmt.Println("Project configuration:\n")
	fmt.Printf("\tProject name: \"%s\" (set by %s)\n", PROJECT_SETTINGS["ProjectName"], SET_BY["ProjectName"] )
	fmt.Printf("\tTarget Platform: \"%s\" (set by %s)\n", PROJECT_SETTINGS["TargetPlatform"], SET_BY["TargetPlatform"] )
	fmt.Printf("\tNo. of Orchestrator Nodes: %d (set by %s)\n", PROJECT_SETTINGS["NumberOfOrchestratorNodes"], SET_BY["NumberOfOrchestratorNodes"])
	fmt.Printf("\tNo. of Compute Nodes: %d (set by %s)\n", PROJECT_SETTINGS["NumberOfComputeNodes"], SET_BY["NumberOfComputeNodes"] )
	fmt.Print("\n")

}