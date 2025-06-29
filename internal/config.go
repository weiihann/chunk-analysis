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

	StartBlocks []uint64 `mapstructure:"START_BLOCKS"`
	EndBlocks   []uint64 `mapstructure:"END_BLOCKS"`

	// Retry configuration
	RetryMaxAttempts int  `mapstructure:"RETRY_MAX_ATTEMPTS"`
	RetryBaseDelay   int  `mapstructure:"RETRY_BASE_DELAY_MS"`
	RetryMaxDelay    int  `mapstructure:"RETRY_MAX_DELAY_MS"`
	RetryJitter      bool `mapstructure:"RETRY_JITTER"`
}

func (c *Config) String() string {
	return fmt.Sprintf("Config{RPCURLs: %v, TraceDir: %s, LogLevel: %s, LogFormat: %s, LogFile: %s, StartBlocks: %v, EndBlocks: %v, RetryMaxAttempts: %d, RetryBaseDelay: %d, RetryMaxDelay: %d, RetryJitter: %t}",
		c.RPCURLs, c.TraceDir, c.LogLevel, c.LogFormat, c.LogFile, c.StartBlocks, c.EndBlocks, c.RetryMaxAttempts, c.RetryBaseDelay, c.RetryMaxDelay, c.RetryJitter)
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

	// Retry configuration validation
	if config.RetryMaxAttempts < 1 {
		errors = append(errors, ValidationError{
			Field:   "RETRY_MAX_ATTEMPTS",
			Message: "retry max attempts must be at least 1",
		})
	}

	if config.RetryBaseDelay < 0 {
		errors = append(errors, ValidationError{
			Field:   "RETRY_BASE_DELAY_MS",
			Message: "retry base delay must be non-negative",
		})
	}

	if config.RetryMaxDelay < config.RetryBaseDelay {
		errors = append(errors, ValidationError{
			Field:   "RETRY_MAX_DELAY_MS",
			Message: "retry max delay must be greater than or equal to base delay",
		})
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
	viper.SetDefault("RETRY_MAX_ATTEMPTS", 100)
	viper.SetDefault("RETRY_BASE_DELAY_MS", 1000)
	viper.SetDefault("RETRY_MAX_DELAY_MS", 20000)
	viper.SetDefault("RETRY_JITTER", true)
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
