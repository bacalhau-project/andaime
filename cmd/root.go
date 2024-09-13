package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/bacalhau-project/andaime/cmd/beta/aws"
	"github.com/bacalhau-project/andaime/cmd/beta/azure"
	"github.com/bacalhau-project/andaime/cmd/beta/gcp"
	"github.com/bacalhau-project/andaime/pkg/logger"
	aws_provider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	azure_provider "github.com/bacalhau-project/andaime/pkg/providers/azure"
	gcp_provider "github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/bacalhau-project/andaime/pkg/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configFile                string
	projectName               string
	targetPlatform            string
	numberOfOrchestratorNodes int
	numberOfComputeNodes      int
	targetRegions             string
	orchestratorIP            string
	awsProfile                string
	verboseMode               bool
	outputFormat              string

	once                             sync.Once
	numberOfDefaultOrchestratorNodes = 1
	numberOfDefaultComputeNodes      = 2
)

var shouldInitAWSFlag bool
var shouldInitAzureFlag bool
var shouldInitGCPFlag bool

type cloudProvider struct {
	awsProvider   aws_provider.AWSProviderer
	azureProvider azure_provider.AzureProviderer
	gcpProvider   gcp_provider.GCPProviderer
}

func Execute() error {
	l := initLogger()
	l.Debug("Initializing configuration")
	initConfig()
	l.Debug("Configuration initialized")

	ctx, cancel := setupSignalHandling()
	defer cancel()

	setupPanicHandling()

	rootCmd := SetupRootCommand()
	rootCmd.SetContext(ctx)
	if err := rootCmd.Execute(); err != nil {
		l.Errorf("Command execution failed: %v", err)
		return err
	}

	l.Debug("Command execution completed")
	return nil
}

func initLogger() *logger.Logger {
	logger.InitLoggerOutputs()
	logger.InitProduction()
	return logger.Get()
}

func setupSignalHandling() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		l := logger.Get()
		l.Info("Interrupt received, cancelling execution...")
		cancel()
		time.Sleep(RetryTimeout)
		l.Info("Forcing exit")
		os.Exit(0)
	}()

	return ctx, cancel
}

func setupPanicHandling() {
	defer func() {
		if r := recover(); r != nil {
			l := logger.Get()
			_ = l.Sync()
			logPanic(l, r)
		}
	}()
}

func logPanic(l *logger.Logger, r interface{}) {
	debugLog, err := os.OpenFile(
		"/tmp/andaime.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		FilePermissions,
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
}

func SetupRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "andaime",
		Short: "Andaime is a tool for managing cloud resources",
		Long: `Andaime is a comprehensive tool for managing cloud resources,
       including deploying and managing Bacalhau nodes across multiple cloud providers.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.ParseFlags(os.Args[1:]); err != nil {
				return err
			}
			if verboseMode {
				logger.SetLevel(logger.DEBUG)
			}
			initConfig()
			return nil
		},
	}

	setupFlags(rootCmd)
	betaCmd := getBetaCmd(rootCmd)
	betaCmd.AddCommand(aws.AwsCmd)
	betaCmd.AddCommand(azure.GetAzureCmd())
	betaCmd.AddCommand(gcp.GetGCPCmd())

	initializeCloudProviders()

	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.Println("Error:", err)
		cmd.Println(cmd.UsageString())
		return err
	})

	return rootCmd
}

func setupFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().
		StringVar(&configFile, "config", "", "config file (default is $HOME/.andaime.yaml)")
	cmd.PersistentFlags().BoolVar(&verboseMode, "verbose", false, "Enable verbose output")
	cmd.PersistentFlags().StringVar(&outputFormat, "output", "text", "Output format: text or json")
	cmd.PersistentFlags().StringVar(&projectName, "project-name", "", "Set project name")
	cmd.PersistentFlags().StringVar(&targetPlatform, "target-platform", "", "Set target platform")
	cmd.PersistentFlags().
		IntVar(&numberOfOrchestratorNodes,
			"orchestrator-nodes",
			numberOfDefaultOrchestratorNodes,
			"Set number of orchestrator nodes")
	cmd.PersistentFlags().
		IntVar(&numberOfComputeNodes, "compute-nodes", numberOfDefaultComputeNodes, "Set number of compute nodes")
	cmd.PersistentFlags().
		StringVar(&targetRegions, "target-regions", "us-east-1", "Comma-separated list of target AWS regions")
	cmd.PersistentFlags().
		StringVar(&orchestratorIP, "orchestrator-ip", "", "IP address of existing orchestrator node")
	cmd.PersistentFlags().
		StringVar(&awsProfile, "aws-profile", "default", "AWS profile to use for credentials")
}

func initConfig() {
	l := logger.Get()
	l.Debug("Starting initConfig")

	viper.SetConfigType("yaml")
	setupConfigFile()
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			l.Debug("No config file found")
		} else {
			l.Errorf("Error reading config file: %v", err)
		}
	} else {
		l.Debugf("Successfully read config file: %s", viper.ConfigFileUsed())
	}

	validateOutputFormat()
	l.Info("Configuration initialization complete")
}

func setupConfigFile() {
	l := logger.Get()
	if configFile != "" {
		l.Debugf("Using config file: %s", configFile)
		viper.SetConfigFile(configFile)
		return
	}

	l.Debug("No config file specified, using default paths")
	home, err := os.UserHomeDir()
	if err != nil {
		l.Error(
			"Unable to determine home directory. Please specify a config file using the --config flag.",
		)
		return
	}

	viper.AddConfigPath(home)
	viper.AddConfigPath(".")
	viper.SetConfigName(".andaime")
	viper.SetConfigName("config")
	l.Debugf("Default config paths: %s/.andaime.yaml, %s/config.yaml, ./config.yaml", home, home)
}

func validateOutputFormat() {
	l := logger.Get()
	if outputFormat != "text" && outputFormat != "json" {
		l.Warnf("Invalid output format '%s'. Using default: text", outputFormat)
		outputFormat = "text"
	}
}

func initializeCloudProviders() (*cloudProvider, error) {
	cp := &cloudProvider{}

	if shouldInitAWS() {
		if err := initAWSProvider(cp); err != nil {
			logger.Get().Errorf("Failed to initialize AWS provider: %v", err)
			return nil, err
		}
	}

	if shouldInitAzure() {
		if err := initAzureProvider(cp); err != nil {
			logger.Get().Errorf("Failed to initialize Azure provider: %v", err)
			return nil, err
		}
	}

	if shouldInitGCP() {
		if err := initGCPProvider(cp); err != nil {
			logger.Get().Errorf("Failed to initialize GCP provider: %v", err)
			return nil, err
		}
	}

	return cp, nil
}

func initAWSProvider(c *cloudProvider) error {
	awsProvider, err := aws_provider.NewAWSProvider(viper.GetViper())
	if err != nil {
		return fmt.Errorf("failed to initialize AWS provider: %w", err)
	}
	c.awsProvider = awsProvider
	return nil
}

func initAzureProvider(c *cloudProvider) error {
	azureProvider, err := azure_provider.NewAzureProviderFunc()
	if err != nil {
		return fmt.Errorf("failed to initialize Azure provider: %w", err)
	}
	c.azureProvider = azureProvider
	return nil
}

func initGCPProvider(c *cloudProvider) error {
	ctx := context.Background()
	gcpProvider, err := gcp_provider.NewGCPProviderFunc(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize GCP provider: %w", err)
	}
	c.gcpProvider = gcpProvider
	return nil
}

func shouldInitAWS() bool {
	return viper.IsSet("aws")
}

func shouldInitAzure() bool {
	return viper.IsSet("azure")
}

func shouldInitGCP() bool {
	return viper.IsSet("gcp")
}
