package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	configFile   string
	verboseMode  bool
	outputFormat string
)

func Execute() error {
	l := initLogger()
	l.Debug("Initializing configuration")

	rootCmd := SetupRootCommand()

	// Parse config flag early
	if err := parseConfigFlag(); err != nil {
		return err
	}

	ctx, cancel := setupSignalHandling()
	defer cancel()

	setupPanicHandling()

	rootCmd.SetContext(ctx)

	// Now execute the command
	if err := rootCmd.Execute(); err != nil {
		l.Errorf("Command execution failed: %v", err)
		return err
	}

	l.Debug("Command execution completed")
	return nil
}

func parseConfigFlag() error {
	// Create a new flag set
	flagSet := pflag.NewFlagSet("config", pflag.ContinueOnError)
	flagSet.ParseErrorsWhitelist.UnknownFlags = true

	// Add only the config flag
	flagSet.StringVar(&configFile, "config", "", "config file path")

	// Skip if help is requested
	for _, arg := range os.Args[1:] {
		if arg == "--help" || arg == "-h" {
			return nil
		}
	}

	// Parse only the known flags (config in this case)
	if err := flagSet.Parse(os.Args[1:]); err != nil {
		return fmt.Errorf("failed to parse config flag: %w", err)
	}

	// If config file is provided, read it
	if configFile != "" {
		viper.SetConfigFile(configFile)
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read configuration file: %w", err)
		}
	} else {
		// Set default config file if not specified
		viper.SetConfigName("config")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.andaime")
		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return fmt.Errorf("failed to read configuration file: %w", err)
			}
			// Config file not found; ignore error if desired
		}
	}

	return nil
}

func initLogger() logger.Logger {
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
		time.Sleep(2 * time.Second) // RetryTimeout
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

func logPanic(l logger.Logger, r interface{}) {
	debugLog, err := os.OpenFile(
		"/tmp/andaime.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644, // FilePermissions
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
			if verboseMode {
				logger.Get().SetVerbose(true)
			}
			return nil
		},
	}

	// Add global flags
	rootCmd.PersistentFlags().
		StringVar(&configFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&verboseMode, "verbose", false, "Enable verbose output")
	rootCmd.PersistentFlags().
		StringVar(&outputFormat, "output", "text", "Output format: text or json")

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	// Add beta command
	rootCmd.AddCommand(GetAwsCmd())
	rootCmd.AddCommand(GetBetaCmd())

	return rootCmd
}

// GetRootCommandForTest returns the root command without executing it
func GetRootCommandForTest() *cobra.Command {
	rootCmd := SetupRootCommand()
	// Disable the PersistentPreRunE function for testing
	rootCmd.PersistentPreRunE = nil
	return rootCmd
}
