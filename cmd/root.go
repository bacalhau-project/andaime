package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configFile   string
	verboseMode  bool
	outputFormat string

	once sync.Once
)

func Execute() error {
	l := initLogger()
	l.Debug("Initializing configuration")

	rootCmd := SetupRootCommand()

	// Parse flags before initializing config
	if err := rootCmd.PersistentFlags().Parse(os.Args[1:]); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	if err := initConfig(); err != nil {
		l.Errorf("Failed to initialize config: %v", err)
		return err
	}

	ctx, cancel := setupSignalHandling()
	defer cancel()

	setupPanicHandling()

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

func logPanic(l *logger.Logger, r interface{}) {
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
				logger.SetLevel(logger.DEBUG)
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().
		StringVar(&configFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&verboseMode, "verbose", false, "Enable verbose output")
	rootCmd.PersistentFlags().
		StringVar(&outputFormat, "output", "text", "Output format: text or json")

	// Add beta command
	rootCmd.AddCommand(GetBetaCmd())

	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.Println("Error:", err)
		cmd.Println(cmd.UsageString())
		return err
	})

	return rootCmd
}

func setupFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().
		StringVar(&configFile, "config", "", "config file (default is ./config.yaml)")
	cmd.PersistentFlags().BoolVar(&verboseMode, "verbose", false, "Enable verbose output")
	cmd.PersistentFlags().StringVar(&outputFormat, "output", "text", "Output format: text or json")
}

func initConfig() error {
	l := logger.Get()
	l.Debug("Starting initConfig")

	viper.SetConfigType("yaml")

	if configFile != "" {
		// Use config file from the flag.
		absPath, err := filepath.Abs(configFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for config file: %w", err)
		}
		viper.SetConfigFile(absPath)
		l.Debugf("Using config file specified by flag: %s", absPath)
	} else {
		// Search for config in the working directory
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		l.Debug("No config file specified, using default: ./config.yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if configFile != "" {
				// Only return an error if a specific config file was requested
				return fmt.Errorf("config file not found: %s", configFile)
			}
			l.Debug("No config file found, using defaults and environment variables")
		} else {
			return fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		l.Infof("Using config file: %s", viper.ConfigFileUsed())
	}

	validateOutputFormat()
	l.Info("Configuration initialization complete")
	return nil
}

func validateOutputFormat() {
	l := logger.Get()
	if outputFormat != "text" && outputFormat != "json" {
		l.Warnf("Invalid output format '%s'. Using default: text", outputFormat)
		outputFormat = "text"
	}
}

// SetConfigFile allows setting the config file path programmatically
// This is useful for testing purposes
func SetConfigFile(path string) {
	configFile = path
}

// GetRootCommandForTest returns the root command without executing it
func GetRootCommandForTest() *cobra.Command {
	rootCmd := SetupRootCommand()
	// Disable the PersistentPreRunE function for testing
	rootCmd.PersistentPreRunE = nil
	return rootCmd
}
