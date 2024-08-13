package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"

	"github.com/bacalhau-project/andaime/cmd/beta/aws"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	awsprovider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	azureprovider "github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	ConfigFile                string
	projectName               string
	targetPlatform            string
	numberOfOrchestratorNodes int
	numberOfComputeNodes      int
	targetRegions             string
	orchestratorIP            string
	awsProfile                string
	verboseMode               bool
	outputFormat              string

	once sync.Once
)

var (
	NumberOfDefaultOrchestratorNodes = 1
	NumberOfDefaultComputeNodes      = 2
)

type CloudProvider struct {
	awsProvider   awsprovider.AWSProviderer
	azureProvider azureprovider.AzureProviderer
}

// This comes from the /main.go file (in workspace root directory NOT in this directory)
func Execute() error {
	// Initialize the logger
	logger.InitLoggerOutputs()
	logger.InitProduction()
	initConfig()

	// Set up signal handling
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-c
		logger.Get().Info("Interrupt received, cancelling execution...")
		cancel()
	}()

	// Set up panic handling
	defer func() {
		if r := recover(); r != nil {
			l := logger.Get()
			_ = l.Sync()

			// Stop the display first
			if disp := display.GetGlobalDisplay(); disp != nil {
				disp.Stop()
			}

			// Log the panic to debug.log
			debugLog, err := os.OpenFile(
				"/tmp/andaime.log",
				os.O_APPEND|os.O_CREATE|os.O_WRONLY,
				0644,
			)
			if err == nil {
				defer debugLog.Close()
				l.Errorf("Panic occurred: %v\n", r)
				l.Error(string(debug.Stack()))
				l.Errorf("Open Channels: %v", utils.GlobalChannels)
				if err, ok := r.(error); ok {
					l.Errorf("Error details: %v", err)
				}
			} else {
				l.Errorf("Failed to open debug log file: %v", err)
			}

			// Print the stack trace to stderr
			fmt.Fprintf(os.Stderr, "Panic occurred: %v\n", r)
			debug.PrintStack()
			if err, ok := r.(error); ok {
				fmt.Fprintf(os.Stderr, "Error details: %v\n", err)
			}
		}
	}()

	rootCmd := SetupRootCommand()
	rootCmd.SetContext(ctx)
	err := rootCmd.Execute()
	if err != nil {
		logger.Get().Errorf("Command execution failed: %v", err)
		os.Exit(1)
	}
	cancel() // Ensure context is cancelled
	logger.Get().Debug("Command execution completed")
	return nil
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "andaime",
	Short: "Andaime is a tool for managing cloud resources",
	Long: `Andaime is a comprehensive tool for managing cloud resources,
including deploying and managing Bacalhau nodes across multiple cloud providers.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initConfig()
	},
}

func initConfig() {
	// Use a temporary logger for initial debugging
	debugLog, err := os.OpenFile(
		"/tmp/andaime-start.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening debug log file: %v\n", err)
		os.Exit(1)
	}
	defer debugLog.Close()

	tmpLogger := log.New(debugLog, "", log.LstdFlags)

	tmpLogger.Printf("Debug: Starting initConfig")
	tmpLogger.Printf("Debug: ConfigFile value: %s", ConfigFile)
	tmpLogger.Printf("Debug: Output format: %s", outputFormat)

	viper.SetConfigType("yaml")

	if ConfigFile != "" {
		tmpLogger.Printf("Debug: Using config file: %s", ConfigFile)
		viper.SetConfigFile(ConfigFile)
	} else {
		tmpLogger.Print("Debug: No config file specified, using default paths")
		home, err := os.UserHomeDir()
		if err != nil {
			tmpLogger.Printf("Error getting user home directory: %v", err)
			tmpLogger.Printf(`Error: Unable to determine home directory.
 Please specify a config file using the --config flag.`)
			os.Exit(1)
		}
		viper.AddConfigPath(home)
		viper.AddConfigPath(".") // Add current directory as a search path
		viper.SetConfigName(".andaime")
		viper.SetConfigName("config") // Add "config" as a config name to search for
		tmpLogger.Printf("Debug: Default config paths: %s/.andaime.yaml, %s/config.yaml, ./config.yaml", home, home)
	}

	viper.AutomaticEnv()
	tmpLogger.Print("Debug: Environment variables loaded into viper")

	tmpLogger.Print("Debug: Attempting to read config file")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			tmpLogger.Print("Debug: No config file found")
		} else {
			tmpLogger.Printf("Error reading config file: %v", err)
		}
	} else {
		tmpLogger.Printf("Debug: Successfully read config file: %s", viper.ConfigFileUsed())
	}

	// Ensure output format is set correctly
	if outputFormat != "text" && outputFormat != "json" {
		tmpLogger.Printf("Debug: Invalid output format '%s'. Using default: text", outputFormat)
		outputFormat = "text"
	}

	logger.GlobalEnableConsoleLogger = false
	logger.GlobalEnableFileLogger = true
	logger.GlobalEnableBufferLogger = true
	logger.GlobalLogPath = "/tmp/andaime.log"
	logger.GlobalLogLevel = "info"

	// Initialize the logger after config is read
	logger.InitProduction()
	logger.SetOutputFormat(outputFormat)
	logger.GlobalInstantSync = true
	// Set log level based on verbose flag
	if verboseMode {
		logger.SetLevel(logger.DEBUG)
	}

	tmpLogger.Printf(
		"Logger initialized with configuration: %v",
		zap.String("outputFormat", outputFormat),
	)
	tmpLogger.Print("Configuration initialization complete")
}

func SetupRootCommand() *cobra.Command {
	// Setup flags
	setupFlags()

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Parse flags for the current command and all parent commands
		err := cmd.ParseFlags(os.Args[1:])
		if err != nil {
			return err
		}

		// Set log level based on verbose flag
		if verboseMode {
			logger.SetLevel(logger.DEBUG)
		}

		// Now that flags are parsed, we can initialize the config
		initConfig()

		return nil
	}
	// Add commands
	rootCmd.AddCommand(getMainCmd())

	betaCmd := getBetaCmd(rootCmd)
	betaCmd.AddCommand(aws.AwsCmd)
	// Dynamically initialize required cloud providers based on configuration
	initializeCloudProviders()

	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.Println("Error:", err)
		cmd.Println(cmd.UsageString())
		return err
	})

	return rootCmd
}

func setupFlags() {
	rootCmd.PersistentFlags().
		StringVar(&ConfigFile, "config", "", "config file (default is $HOME/.andaime.yaml)")
	rootCmd.PersistentFlags().BoolVar(&verboseMode, "verbose", false, "Enable verbose output")
	rootCmd.PersistentFlags().
		StringVar(&outputFormat, "output", "text", "Output format: text or json")

	rootCmd.PersistentFlags().StringVar(&projectName, "project-name", "", "Set project name")
	rootCmd.PersistentFlags().
		StringVar(&targetPlatform, "target-platform", "", "Set target platform")
	rootCmd.PersistentFlags().IntVar(&numberOfOrchestratorNodes,
		"orchestrator-nodes",
		NumberOfDefaultOrchestratorNodes,
		"Set number of orchestrator nodes")
	rootCmd.PersistentFlags().IntVar(&numberOfComputeNodes,
		"compute-nodes",
		numberOfComputeNodes,
		"Set number of compute nodes")
	rootCmd.PersistentFlags().StringVar(&targetRegions,
		"target-regions",
		"us-east-1",
		"Comma-separated list of target AWS regions")
	rootCmd.PersistentFlags().
		StringVar(&orchestratorIP, "orchestrator-ip", "", "IP address of existing orchestrator node")
	rootCmd.PersistentFlags().
		StringVar(&awsProfile, "aws-profile", "default", "AWS profile to use for credentials")
}

func initializeCloudProviders() *CloudProvider {
	cloudProvider := &CloudProvider{}

	// Example: Initialize AWS provider
	if shouldInitAWS() {
		if err := initAWSProvider(cloudProvider); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize AWS provider: %v\n", err)
			os.Exit(1)
		}
	}

	if shouldInitAzure() {
		if err := initAzureProvider(cloudProvider); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize Azure provider: %v\n", err)
			os.Exit(1)
		}
	}
	return cloudProvider
}

// initAWSProvider initializes the AWS provider with the given profile and region
func initAWSProvider(c *CloudProvider) error {
	awsProvider, err := awsprovider.NewAWSProvider(viper.GetViper())
	if err != nil {
		return fmt.Errorf("failed to initialize AWS provider: %w", err)
	}
	c.awsProvider = awsProvider
	return nil
}

func shouldInitAWS() bool {
	// TODO: Detect if AWS should be instantiated
	return true
}

func initAzureProvider(c *CloudProvider) error {
	azureProvider, err := azureprovider.NewAzureProvider()
	if err != nil {
		return fmt.Errorf("failed to initialize Azure provider: %w", err)
	}
	c.azureProvider = azureProvider
	return nil
}

func shouldInitAzure() bool {
	// TODO: Detect if Azure should be instantiated
	return true
}
