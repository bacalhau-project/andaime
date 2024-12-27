package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config holds the configuration for the logger
type Config struct {
	Level         string `yaml:"level"          json:"level"`
	FilePath      string `yaml:"file_path"      json:"file_path"`
	Format        string `yaml:"format"         json:"format"`
	WithTrace     bool   `yaml:"with_trace"     json:"with_trace"`
	EnableConsole bool   `yaml:"enable_console" json:"enable_console"`
	EnableBuffer  bool   `yaml:"enable_buffer"  json:"enable_buffer"`
	BufferSize    int    `yaml:"buffer_size"    json:"buffer_size"`
	InstantSync   bool   `yaml:"instant_sync"   json:"instant_sync"`
}

// Initialize sets up the logger with the given configuration
func Initialize(config Config) error {
	// Set global configuration
	GlobalEnableConsoleLogger = config.EnableConsole
	GlobalEnableFileLogger = config.FilePath != ""
	GlobalEnableBufferLogger = config.EnableBuffer
	GlobalInstantSync = config.InstantSync

	if config.BufferSize > 0 {
		GlobalLoggedBufferSize = config.BufferSize
		GlobalLogBuffer.size = config.BufferSize
	}

	if config.FilePath != "" {
		GlobalLogPath = config.FilePath
	}

	// Set log level
	logLevel := config.Level
	if logLevel == "" {
		logLevel = GlobalLogLevel
	}
	GlobalLogLevel = logLevel

	// Initialize cores based on configuration
	var cores []zapcore.Core
	level := getZapLevel(logLevel)

	// Base encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     customTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Console logging
	if config.EnableConsole {
		consoleCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			level,
		)
		cores = append(cores, consoleCore)
	}

	// File logging
	if config.FilePath != "" {
		file, err := os.OpenFile(
			config.FilePath,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			LogFilePermissions,
		)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		GlobalLogFile = file

		var encoder zapcore.Encoder
		if config.Format == "json" {
			encoder = zapcore.NewJSONEncoder(encoderConfig)
		} else {
			encoder = zapcore.NewConsoleEncoder(encoderConfig)
		}

		fileCore := zapcore.NewCore(
			encoder,
			zapcore.AddSync(file),
			level,
		)
		cores = append(cores, fileCore)
	}

	// Buffer logging
	if config.EnableBuffer {
		bufferCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(&GlobalLogBuffer),
			level,
		)
		cores = append(cores, bufferCore)
	}

	// Create logger
	core := zapcore.NewTee(cores...)
	opts := []zap.Option{zap.AddCaller()}
	if config.WithTrace {
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	logger := zap.New(core, opts...).Named("andaime")
	SetGlobalLogger(&ZapLogger{logger.Sugar()})

	return nil
}
