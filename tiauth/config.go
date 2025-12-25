package tiauth

import (
	"bufio"
	"flag"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the tiauth-faroe server
type Config struct {
	// Database path for SQLite storage
	DBPath string
	// Port to listen on
	Port string
	// Port for Python backend communication (binds to 127.0.0.2)
	PrivatePort int

	// Session expiration duration (default: 90 days)
	SessionExpiration time.Duration

	// CORS allowed origin (specific origin like "https://example.com", empty to not set header)
	CORSAllowOrigin string

	// Behavior flags
	EnableReset       bool // Enable /reset endpoint to clear storage
	EnableInteractive bool // Enable interactive shell mode
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		DBPath:            "./db.sqlite",
		Port:              "3777",
		PrivatePort:       8079,
		SessionExpiration: 90 * 24 * time.Hour, // 90 days
	}
}

// LoadEnv loads environment variables from a file into a map
func LoadEnv(filename string) (map[string]string, error) {
	env := make(map[string]string)

	file, err := os.Open(filename)
	if err != nil {
		return env, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first = sign
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		env[key] = value
	}

	return env, scanner.Err()
}

// GetEnv gets a value from the env map or OS environment, returns empty string if not found
func GetEnv(envMap map[string]string, key string) string {
	if value, exists := envMap[key]; exists && value != "" {
		return value
	}
	return os.Getenv(key)
}

// GetEnvDefault gets a value from the env map or OS environment, with a default fallback
func GetEnvDefault(envMap map[string]string, key string, defaultValue string) string {
	if value := GetEnv(envMap, key); value != "" {
		return value
	}
	return defaultValue
}

// ConfigFromEnv creates a Config from an environment file and/or OS environment
func ConfigFromEnv(envFile string) (Config, error) {
	cfg := DefaultConfig()

	envMap, err := LoadEnv(envFile)
	if err != nil && !os.IsNotExist(err) {
		return cfg, err
	}
	if envMap == nil {
		envMap = make(map[string]string)
	}

	if v := GetEnv(envMap, "FAROE_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := GetEnv(envMap, "FAROE_PORT"); v != "" {
		cfg.Port = v
	}
	if v := GetEnv(envMap, "FAROE_PRIVATE_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.PrivatePort = port
		}
	}
	cfg.CORSAllowOrigin = GetEnv(envMap, "FAROE_CORS_ALLOW_ORIGIN")

	if v := GetEnv(envMap, "FAROE_SESSION_EXPIRATION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.SessionExpiration = d
		}
	}

	return cfg, nil
}

// Flags holds the parsed command line flags
type Flags struct {
	EnvFile     string
	Interactive bool
	EnableReset bool
	PrivatePort int
}

// RegisterFlags registers all tiauth-faroe flags on the given FlagSet.
// If fs is nil, uses flag.CommandLine.
func RegisterFlags(fs *flag.FlagSet) *Flags {
	if fs == nil {
		fs = flag.CommandLine
	}

	f := &Flags{}
	fs.StringVar(&f.EnvFile, "env-file", ".env", "Path to environment file")
	fs.BoolVar(&f.Interactive, "interactive", false, "Run in interactive mode with stdin commands")
	fs.BoolVar(&f.EnableReset, "enable-reset", false, "Enable request to /reset to clear storage")
	fs.IntVar(&f.PrivatePort, "private-port", 0, "Port for Python backend communication (binds to 127.0.0.2)")

	return f
}

// ConfigFromFlags loads config from env file and applies flag overrides.
// Call this after flag.Parse().
func ConfigFromFlags(f *Flags) (Config, error) {
	cfg, err := ConfigFromEnv(f.EnvFile)
	if err != nil {
		return cfg, err
	}

	// Apply flag overrides
	cfg.EnableInteractive = f.Interactive
	cfg.EnableReset = f.EnableReset

	if f.PrivatePort != 0 {
		cfg.PrivatePort = f.PrivatePort
	}

	return cfg, nil
}

// ParseFlagsAndConfig is a convenience function that registers flags,
// parses them, and returns the resulting Config.
// Uses flag.CommandLine and calls flag.Parse().
func ParseFlagsAndConfig() (Config, error) {
	f := RegisterFlags(nil)
	flag.Parse()
	return ConfigFromFlags(f)
}
