package config

import (
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
	// WorkDir is where the optimizer's tools work on copies of images. Only this process's user may
	// use it; what is in it at start is deleted.
	WorkDir string
}

// Load loads configuration from environment
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if missing)
	_ = godotenv.Load()

	return &Config{
		AppName:    getEnv("APP_NAME", "Goptimizer"),
		AppVersion: "1.0.0",

		DatabasePath:     getEnv("DATABASE_PATH", "./data/goptimizer.db"),
		TestDatabasePath: getEnv("TEST_DATABASE_PATH", "./data/goptimizer_test.db"),
		Port:             getEnv("PORT", "8747"),
		Environment:      getEnv("APP_ENV", "development"),
		Debug:            getEnvBool("APP_DEBUG", false),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		LogFormat:        getEnv("LOG_FORMAT", "json"),
		LogOutput:        getEnv("LOG_OUTPUT", "both"),
		JWTSecret:        getEnv("JWT_SECRET", ""),
		AppURL:           getEnv("APP_URL", "localhost:8747"),

		AccessTokenTTL:       getEnvDuration("JWT_ACCESS_TOKEN_TTL", time.Hour),
		RefreshTokenTTL:      getEnvDuration("JWT_REFRESH_TOKEN_TTL", 30*24*time.Hour),
		PasswordResetTTL:     getEnvDuration("PASSWORD_RESET_TTL", time.Hour),
		EmailVerificationTTL: getEnvDuration("EMAIL_VERIFICATION_TTL", 48*time.Hour),
		AllowRegistration:    getEnvBool("ALLOW_REGISTRATION", false),
		AuthRateLimit:        getEnvBool("AUTH_RATE_LIMIT", true),
		FrontendURL:          strings.TrimRight(getEnv("FRONTEND_URL", "http://localhost:5747"), "/"),
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

		EnableScheduler: getEnvBool("ENABLE_SCHEDULER", true),
		WorkDir:         getEnv("WORK_DIR", filepath.Join(os.TempDir(), "goptimizer-work")),
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
// lets anyone forge an admin's token, so starting with it unset is worse than not starting.
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

func getEnvList(key string) []string {
	var values []string
	for _, item := range strings.Split(os.Getenv(key), ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
