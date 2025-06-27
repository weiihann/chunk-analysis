package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	RPCURLs []string `mapstructure:"RPC_URLS"`

	// File storage configuration
	TraceDir  string `mapstructure:"TRACE_DIR"`
	ResultDir string `mapstructure:"RESULT_DIR"`

	// Logging configuration
	LogLevel  string `mapstructure:"LOG_LEVEL"`
	LogFormat string `mapstructure:"LOG_FORMAT"`
	LogFile   string `mapstructure:"LOG_FILE"`

	StartBlock uint64 `mapstructure:"START_BLOCK"`
	EndBlock   uint64 `mapstructure:"END_BLOCK"`
}

func (c *Config) String() string {
	return fmt.Sprintf("Config{RPCURLs: %v, TraceDir: %s, LogLevel: %s, LogFormat: %s, LogFile: %s, StartBlock: %d, EndBlock: %d}",
		c.RPCURLs, c.TraceDir, c.LogLevel, c.LogFormat, c.LogFile, c.StartBlock, c.EndBlock)
}

func LoadConfig(path string) (config Config, err error) {
	// Configure viper
	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	// Set comprehensive defaults
	setDefaults()

	// Try to read config file
	if err := viper.ReadInConfig(); err != nil {
		// Check if it's a file not found error
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, but that's okay - we can use environment variables and defaults
		} else {
			// Config file was found but another error was produced
			return config, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal into config struct
	if err := viper.Unmarshal(&config); err != nil {
		return config, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return config, err
	}

	// Expand data directory paths
	config.TraceDir = expandPath(config.TraceDir)
	if config.LogFile != "" {
		config.LogFile = expandPath(config.LogFile)
	}

	return config, nil
}

func validateConfig(config Config) error {
	var errors ValidationErrors

	// Log level validation
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !slices.Contains(validLogLevels, strings.ToLower(config.LogLevel)) {
		errors = append(errors, ValidationError{
			Field:   "LOG_LEVEL",
			Message: fmt.Sprintf("log level must be one of: %s", strings.Join(validLogLevels, ", ")),
		})
	}

	// Log format validation
	validLogFormats := []string{"text", "json"}
	if !slices.Contains(validLogFormats, strings.ToLower(config.LogFormat)) {
		errors = append(errors, ValidationError{
			Field:   "LOG_FORMAT",
			Message: fmt.Sprintf("log format must be one of: %s", strings.Join(validLogFormats, ", ")),
		})
	}

	// Log file validation (if specified)
	if config.LogFile != "" {
		logDir := filepath.Dir(config.LogFile)
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			errors = append(errors, ValidationError{
				Field:   "LOG_FILE",
				Message: fmt.Sprintf("cannot create log file directory '%s': %v", logDir, err),
			})
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func setDefaults() {
	viper.SetDefault("RPC_URLS", []string{"http://localhost:8545"})
	viper.SetDefault("DATA_DIR", "data")
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("LOG_FORMAT", "text")
	viper.SetDefault("LOG_FILE", "")
}

func expandPath(path string) string {
	if path == "" {
		return path
	}

	// Expand environment variables
	path = os.ExpandEnv(path)

	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	return path
}

// ValidationError represents configuration validation errors
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("config validation error for field '%s': %s", e.Field, e.Message)
}

type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return fmt.Sprintf("configuration validation failed:\n%s", strings.Join(messages, "\n"))
}
