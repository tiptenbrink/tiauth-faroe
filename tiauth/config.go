package tiauth

import (
	"bufio"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultSocketPath returns the default socket path in the system temp directory
func DefaultSocketPath() string {
	return filepath.Join(os.TempDir(), "tiauth_faroe.sock")
}

// Config holds all configuration for the tiauth-faroe server
type Config struct {
	// Database path for SQLite storage
	DBPath string
	// Port to listen on
	Port string
	// Path to Unix socket for Python backend communication (user actions + notifications)
	SocketPath string

	// SMTP configuration
	SMTPSenderName  string
	SMTPSenderEmail string
	SMTPServerHost  string
	SMTPServerPort  string
	SMTPDomain      string

	// Session expiration duration (default: 90 days)
	SessionExpiration time.Duration

	// Path to email templates directory (empty for defaults)
	EmailTemplatesPath string

	// CORS allowed origin (specific origin like "https://example.com", empty to not set header)
	CORSAllowOrigin string

	// Security and behavior flags
	DisableSMTP       bool // Disable SMTP entirely (only broadcast tokens, don't send emails)
	InsecureSMTP      bool // Disable TLS for SMTP (dangerous, for testing only)
	NoKeepAlive       bool // Disable SMTP keep-alive routine
	EnableReset       bool // Enable /reset endpoint to clear storage
	EnableInteractive bool // Enable interactive shell mode
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		DBPath:            "./db.sqlite",
		Port:              "3777",
		SocketPath:        DefaultSocketPath(),
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
	if v := GetEnv(envMap, "FAROE_SOCKET_PATH"); v != "" {
		cfg.SocketPath = v
	}
	cfg.SMTPSenderName = GetEnv(envMap, "FAROE_SMTP_SENDER_NAME")
	cfg.SMTPSenderEmail = GetEnv(envMap, "FAROE_SMTP_SENDER_EMAIL")
	cfg.SMTPServerHost = GetEnv(envMap, "FAROE_SMTP_SERVER_HOST")
	cfg.SMTPServerPort = GetEnv(envMap, "FAROE_SMTP_SERVER_PORT")
	cfg.SMTPDomain = GetEnv(envMap, "FAROE_SMTP_DOMAIN")
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
	EnvFile            string
	DisableSMTP        bool
	Insecure           bool
	Interactive        bool
	NoKeepAlive        bool
	EnableReset        bool
	EmailTemplatesPath string
	SocketPath         string
}

// RegisterFlags registers all tiauth-faroe flags on the given FlagSet.
// If fs is nil, uses flag.CommandLine.
func RegisterFlags(fs *flag.FlagSet) *Flags {
	if fs == nil {
		fs = flag.CommandLine
	}

	f := &Flags{}
	fs.StringVar(&f.EnvFile, "env-file", ".env", "Path to environment file")
	fs.BoolVar(&f.DisableSMTP, "no-smtp", false, "Disable SMTP entirely (only broadcast tokens via socket, don't send emails)")
	fs.BoolVar(&f.Insecure, "insecure", false, "Disable TLS encryption for SMTP (dangerous)")
	fs.BoolVar(&f.Interactive, "interactive", false, "Run in interactive mode with stdin commands")
	fs.BoolVar(&f.NoKeepAlive, "no-keep-alive", false, "Do not run SMTP keep-alive routine")
	fs.BoolVar(&f.EnableReset, "enable-reset", false, "Enable request to /reset to clear storage")
	fs.StringVar(&f.EmailTemplatesPath, "email-templates", "", "Path to email templates directory")
	fs.StringVar(&f.SocketPath, "socket", "", "Path to Unix socket for Python backend communication")

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
	cfg.DisableSMTP = f.DisableSMTP
	cfg.InsecureSMTP = f.Insecure
	cfg.EnableInteractive = f.Interactive
	cfg.NoKeepAlive = f.NoKeepAlive
	cfg.EnableReset = f.EnableReset

	if f.EmailTemplatesPath != "" {
		cfg.EmailTemplatesPath = f.EmailTemplatesPath
	}
	if f.SocketPath != "" {
		cfg.SocketPath = f.SocketPath
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
