package unit

import (
	"bytes"
	"encoding/base64"
	"testing"

	"app/config"
	"app/internal/secretbox"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ValidateJWTSecret(t *testing.T) {
	strong := "3f9c1e7a5b2d4c6e8f0a1b3c5d7e9f1a2b4c6d8e"

	cases := []struct {
		name    string
		env     string
		secret  string
		wantErr bool
		weak    bool
	}{
		{"empty is always refused", "development", "", true, true},
		{"blank is always refused", "development", "   ", true, true},
		{"short is tolerated in development", "development", "dev-secret", false, true},
		{"short is refused in production", "production", "dev-secret", true, true},
		{"the .env.example value is refused in production", "production", "your-super-secret-jwt-key-change-this-in-production", true, true},
		{"the .env.example value is tolerated in development", "development", "your-super-secret-jwt-key-change-this-in-production", false, true},
		{"a long random secret passes in production", "production", strong, false, false},
		{"production is matched case-insensitively", "Production", "dev-secret", true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Environment: tc.env, JWTSecret: tc.secret}
			err := cfg.ValidateJWTSecret()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.weak, cfg.JWTSecretIsWeak())
		})
	}
}

func TestConfig_LoadAuthDefaults(t *testing.T) {
	for _, key := range []string{"JWT_ACCESS_TOKEN_TTL", "ALLOW_REGISTRATION", "AUTH_RATE_LIMIT", "TRUSTED_PROXIES", "FRONTEND_URL"} {
		t.Setenv(key, "")
	}
	cfg, err := config.Load()
	assert.NoError(t, err)
	assert.Equal(t, "1h0m0s", cfg.AccessTokenTTL.String())
	assert.False(t, cfg.AllowRegistration)
	assert.True(t, cfg.AuthRateLimit)
	assert.Equal(t, []string{"127.0.0.1", "::1"}, cfg.TrustedProxies)

	t.Setenv("JWT_ACCESS_TOKEN_TTL", "15m")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, 10.0.0.2")
	t.Setenv("FRONTEND_URL", "https://admin.example.com/")
	cfg, err = config.Load()
	assert.NoError(t, err)
	assert.Equal(t, "15m0s", cfg.AccessTokenTTL.String())
	assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, cfg.TrustedProxies)
	assert.Equal(t, "https://admin.example.com", cfg.FrontendURL)

	t.Setenv("JWT_ACCESS_TOKEN_TTL", "soon")
	cfg, _ = config.Load()
	assert.Equal(t, "1h0m0s", cfg.AccessTokenTTL.String(), "an unparsable duration falls back to the default")
}

func TestConfig_EncryptionKey(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))

	t.Setenv("ENCRYPTION_KEY", key)
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, bytes.Repeat([]byte{7}, 32), cfg.EncryptionKey)
	assert.False(t, cfg.EncryptionKeyIsInsecure())
	cfg.Environment = "production"
	assert.NoError(t, cfg.ValidateEncryptionKey())

	for _, bad := range []string{"not base64!", base64.StdEncoding.EncodeToString([]byte("too short"))} {
		t.Setenv("ENCRYPTION_KEY", bad)
		_, err := config.Load()
		assert.Error(t, err, "a malformed key stops every process at load: %q", bad)
	}

	t.Setenv("ENCRYPTION_KEY", "")
	cfg, err = config.Load()
	require.NoError(t, err)
	assert.True(t, cfg.EncryptionKeyIsInsecure(), "unset falls back to the development key")
	cfg.Environment = "development"
	assert.NoError(t, cfg.ValidateEncryptionKey())
	cfg.Environment = "production"
	assert.Error(t, cfg.ValidateEncryptionKey(), "production refuses the development key")
}

func TestSecretbox(t *testing.T) {
	box, err := secretbox.New(bytes.Repeat([]byte{1}, 32))
	require.NoError(t, err)
	other, err := secretbox.New(bytes.Repeat([]byte{2}, 32))
	require.NoError(t, err)

	sealed := box.Seal([]byte("secret"))
	assert.NotEqual(t, sealed, box.Seal([]byte("secret")), "every seal has its own nonce")
	plain, err := box.Open(sealed)
	require.NoError(t, err)
	assert.Equal(t, "secret", string(plain))

	_, err = other.Open(sealed)
	assert.ErrorIs(t, err, secretbox.ErrUnreadable)
	sealed[len(sealed)-1] ^= 1
	_, err = box.Open(sealed)
	assert.ErrorIs(t, err, secretbox.ErrUnreadable)
	_, err = box.Open([]byte("short"))
	assert.ErrorIs(t, err, secretbox.ErrUnreadable)

	_, err = secretbox.New([]byte("short"))
	assert.Error(t, err)
}
