package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	AppName    string
	AppVersion string

	DatabasePath     string
	TestDatabasePath string
	Port             string
	Environment      string
	Debug            bool
	LogLevel         string
	LogFormat        string
	LogOutput        string
	JWTSecret        string
	AppURL           string

	// EncryptionKey seals stored credentials (32 bytes, ENCRYPTION_KEY in base64). Unset outside
	// production it is devEncryptionKey; see ValidateEncryptionKey.
	EncryptionKey []byte

	// Auth
	AccessTokenTTL       time.Duration
	RefreshTokenTTL      time.Duration
	PasswordResetTTL     time.Duration
	EmailVerificationTTL time.Duration
	// AllowRegistration opens POST /auth/register to anyone
	AllowRegistration bool
	// AuthRateLimit throttles login, refresh and the emailed-link endpoints
	AuthRateLimit bool
	// FrontendURL is the base of the links sent in emails
	FrontendURL string
	// TrustedProxies are the proxies whose X-Forwarded-For is believed when resolving the client IP
	TrustedProxies []string

	// Mail (SMTP)
	MailMailer      string
	MailScheme      string
	MailHost        string
	MailPort        int
	MailUsername    string
	MailPassword    string
	MailFromAddress string
	MailFromName    string

	// CORSAllowedOrigins is empty (or contains "*") to allow any origin
	CORSAllowedOrigins []string

	EnableScheduler bool
	// RunnerConcurrency is how many backups and restores run at once. A plan runs one at a time.
	RunnerConcurrency int
	// WorkDir is the absolute directory restores download archives to and extract into by
	// default. It needs room for the largest Postgres archive.
	WorkDir string
	// RestoreJobs is how many parallel jobs pg_restore runs.
	RestoreJobs int
}

// Load loads configuration from environment
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if missing)
	_ = godotenv.Load()

	encryptionKey, err := parseEncryptionKey(os.Getenv("ENCRYPTION_KEY"))
	if err != nil {
		return nil, err
	}
	workDir, err := filepath.Abs(getEnv("WORK_DIR", "./data/work"))
	if err != nil {
		return nil, fmt.Errorf("WORK_DIR: %w", err)
	}

	return &Config{
		AppName:    getEnv("APP_NAME", "Gogobackup"),
		AppVersion: "1.0.0",

		DatabasePath:     getEnv("DATABASE_PATH", "./data/gogobackup.db"),
		TestDatabasePath: getEnv("TEST_DATABASE_PATH", "./data/gogobackup_test.db"),
		Port:             getEnv("PORT", "8181"),
		Environment:      getEnv("APP_ENV", "development"),
		Debug:            getEnvBool("APP_DEBUG", false),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		LogFormat:        getEnv("LOG_FORMAT", "json"),
		LogOutput:        getEnv("LOG_OUTPUT", "both"),
		JWTSecret:        getEnv("JWT_SECRET", ""),
		AppURL:           getEnv("APP_URL", "localhost:8181"),
		EncryptionKey:    encryptionKey,

		AccessTokenTTL:       getEnvDuration("JWT_ACCESS_TOKEN_TTL", time.Hour),
		RefreshTokenTTL:      getEnvDuration("JWT_REFRESH_TOKEN_TTL", 30*24*time.Hour),
		PasswordResetTTL:     getEnvDuration("PASSWORD_RESET_TTL", time.Hour),
		EmailVerificationTTL: getEnvDuration("EMAIL_VERIFICATION_TTL", 48*time.Hour),
		AllowRegistration:    getEnvBool("ALLOW_REGISTRATION", false),
		AuthRateLimit:        getEnvBool("AUTH_RATE_LIMIT", true),
		FrontendURL:          strings.TrimRight(getEnv("FRONTEND_URL", "http://localhost:5173"), "/"),
		TrustedProxies:       getEnvListDefault("TRUSTED_PROXIES", []string{"127.0.0.1", "::1"}),

		MailMailer:      getEnv("MAIL_MAILER", "smtp"),
		MailScheme:      getEnv("MAIL_SCHEME", "smtp"),
		MailHost:        getEnv("MAIL_HOST", ""),
		MailPort:        getEnvInt("MAIL_PORT", 587),
		MailUsername:    getEnv("MAIL_USERNAME", ""),
		MailPassword:    getEnv("MAIL_PASSWORD", ""),
		MailFromAddress: strings.Trim(getEnv("MAIL_FROM_ADDRESS", ""), `"'`),
		MailFromName:    strings.Trim(getEnv("MAIL_FROM_NAME", ""), `"'`),

		CORSAllowedOrigins: getEnvList("CORS_ALLOWED_ORIGINS"),

		EnableScheduler:   getEnvBool("ENABLE_SCHEDULER", true),
		RunnerConcurrency: max(getEnvInt("RUNNER_CONCURRENCY", 2), 1),
		WorkDir:           workDir,
		RestoreJobs:       max(getEnvInt("RESTORE_JOBS", 4), 1),
	}, nil
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvDuration accepts Go duration strings such as "30m" or "720h". An unparsable or
// non-positive value falls back to the default rather than producing tokens that expire instantly.
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultValue
}

func getEnvListDefault(key string, defaultValue []string) []string {
	if values := getEnvList(key); len(values) > 0 {
		return values
	}
	return defaultValue
}

const (
	minJWTSecretLen      = 32
	placeholderJWTSecret = "your-super-secret-jwt-key-change-this-in-production"
)

func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.Environment, "production")
}

// JWTSecretIsWeak reports a secret that is too short or is the value shipped in .env.example.
func (c *Config) JWTSecretIsWeak() bool {
	return len(c.JWTSecret) < minJWTSecretLen || c.JWTSecret == placeholderJWTSecret
}

// ValidateJWTSecret must pass before any process issues or accepts tokens. An empty HMAC key
// lets anyone forge a super-admin token, so starting with it unset is worse than not starting.
// A weak secret is refused in production and left to the caller to warn about elsewhere.
func (c *Config) ValidateJWTSecret() error {
	if strings.TrimSpace(c.JWTSecret) == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if c.IsProduction() && c.JWTSecretIsWeak() {
		return fmt.Errorf("JWT_SECRET is too weak for production: use at least %d random characters, not the .env.example value", minJWTSecretLen)
	}
	return nil
}

const encryptionKeySize = 32

// devEncryptionKey lets a development checkout run without setting ENCRYPTION_KEY. It is public,
// so anything sealed with it is as good as plain text; production refuses it.
var devEncryptionKey = []byte("gogobackup-insecure-dev-key-0000")

func parseEncryptionKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return devEncryptionKey, nil
	}
	key, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(key) != encryptionKeySize {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be %d random bytes in base64: generate one with openssl rand -base64 32", encryptionKeySize)
	}
	return key, nil
}

// EncryptionKeyIsInsecure reports the built-in development key, used when ENCRYPTION_KEY is unset.
func (c *Config) EncryptionKeyIsInsecure() bool {
	return bytes.Equal(c.EncryptionKey, devEncryptionKey)
}

// ValidateEncryptionKey refuses production without a key of its own: with the public development
// key, anyone holding the database could read every stored credential.
func (c *Config) ValidateEncryptionKey() error {
	if len(c.EncryptionKey) != encryptionKeySize {
		return fmt.Errorf("ENCRYPTION_KEY must be %d bytes", encryptionKeySize)
	}
	if c.IsProduction() && c.EncryptionKeyIsInsecure() {
		return fmt.Errorf("ENCRYPTION_KEY is required in production: generate one with openssl rand -base64 32")
	}
	return nil
}

func getEnvList(key string) []string {
	var values []string
	for _, item := range strings.Split(os.Getenv(key), ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
