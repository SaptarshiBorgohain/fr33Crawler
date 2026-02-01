package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/knowledge-engine/backend/internal/config"
)

func TestLoadDefaultConfig(t *testing.T) {
	// Clear environment variables
	clearEnvVars()
	
	cfg := config.Load()
	
	// Test default values
	assert.Equal(t, 10000, cfg.Frontier.MaxURLsInMemory)
	assert.Equal(t, 3, cfg.Frontier.MaxRetries)
	assert.Equal(t, 1, cfg.Frontier.DefaultPriority)
	assert.Equal(t, 5*time.Minute, cfg.Frontier.CleanupInterval)
	assert.Equal(t, 1*time.Second, cfg.Frontier.PolitenessDelay)
	assert.Equal(t, 10, cfg.Frontier.MaxConcurrency)
	assert.True(t, cfg.Frontier.EnablePersistence)
	
	assert.Equal(t, "localhost:6379", cfg.Storage.RedisURL)
	assert.Equal(t, "", cfg.Storage.RedisPassword)
	assert.Equal(t, 0, cfg.Storage.RedisDB)
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Set environment variables
	envVars := map[string]string{
		"FRONTIER_MAX_URLS":           "5000",
		"FRONTIER_MAX_RETRIES":        "5",
		"FRONTIER_DEFAULT_PRIORITY":   "2",
		"FRONTIER_CLEANUP_INTERVAL":   "10m",
		"FRONTIER_POLITENESS_DELAY":   "2s",
		"FRONTIER_MAX_CONCURRENCY":    "20",
		"FRONTIER_ENABLE_PERSISTENCE": "false",
		"REDIS_URL":                   "redis.example.com:6379",
		"REDIS_PASSWORD":              "secret123",
		"REDIS_DB":                    "1",
	}
	
	// Set env vars
	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer clearEnvVars()
	
	cfg := config.Load()
	
	// Test loaded values
	assert.Equal(t, 5000, cfg.Frontier.MaxURLsInMemory)
	assert.Equal(t, 5, cfg.Frontier.MaxRetries)
	assert.Equal(t, 2, cfg.Frontier.DefaultPriority)
	assert.Equal(t, 10*time.Minute, cfg.Frontier.CleanupInterval)
	assert.Equal(t, 2*time.Second, cfg.Frontier.PolitenessDelay)
	assert.Equal(t, 20, cfg.Frontier.MaxConcurrency)
	assert.False(t, cfg.Frontier.EnablePersistence)
	
	assert.Equal(t, "redis.example.com:6379", cfg.Storage.RedisURL)
	assert.Equal(t, "secret123", cfg.Storage.RedisPassword)
	assert.Equal(t, 1, cfg.Storage.RedisDB)
}

func TestGetStringEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue string
		expected     string
	}{
		{"Existing env var", "TEST_STRING", "test_value", "default", "test_value"},
		{"Non-existing env var", "NON_EXISTENT", "", "default", "default"},
		{"Empty env var", "EMPTY_VAR", "", "default", "default"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean slate
			os.Unsetenv(tt.key)
			
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}
			
			result := config.GetStringEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetIntEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue int
		expected     int
	}{
		{"Valid int", "TEST_INT", "42", 10, 42},
		{"Invalid int", "TEST_INT_INVALID", "not_a_number", 10, 10},
		{"Negative int", "TEST_INT_NEG", "-5", 10, -5},
		{"Zero", "TEST_INT_ZERO", "0", 10, 0},
		{"Non-existing env var", "NON_EXISTENT", "", 10, 10},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(tt.key)
			
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}
			
			result := config.GetIntEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetBoolEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue bool
		expected     bool
	}{
		{"True string", "TEST_BOOL", "true", false, true},
		{"False string", "TEST_BOOL", "false", true, false},
		{"1 (true)", "TEST_BOOL", "1", false, true},
		{"0 (false)", "TEST_BOOL", "0", true, false},
		{"Invalid bool", "TEST_BOOL", "invalid", true, true},
		{"Non-existing env var", "NON_EXISTENT", "", true, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(tt.key)
			
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}
			
			result := config.GetBoolEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDurationEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue time.Duration
		expected     time.Duration
	}{
		{"Valid duration - seconds", "TEST_DURATION", "5s", 1*time.Second, 5*time.Second},
		{"Valid duration - minutes", "TEST_DURATION", "10m", 1*time.Second, 10*time.Minute},
		{"Valid duration - hours", "TEST_DURATION", "2h", 1*time.Second, 2*time.Hour},
		{"Valid duration - combined", "TEST_DURATION", "1h30m", 1*time.Second, 90*time.Minute},
		{"Invalid duration", "TEST_DURATION", "invalid", 5*time.Second, 5*time.Second},
		{"Non-existing env var", "NON_EXISTENT", "", 10*time.Second, 10*time.Second},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(tt.key)
			
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}
			
			result := config.GetDurationEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigStructure(t *testing.T) {
	cfg := config.Load()
	
	// Test that all required fields are present
	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.Frontier)
	assert.NotNil(t, cfg.Storage)
	
	// Test that frontier config has all fields
	assert.Greater(t, cfg.Frontier.MaxURLsInMemory, 0)
	assert.Greater(t, cfg.Frontier.MaxRetries, 0)
	assert.GreaterOrEqual(t, cfg.Frontier.DefaultPriority, 0)
	assert.Greater(t, int64(cfg.Frontier.CleanupInterval), int64(0))
	assert.Greater(t, int64(cfg.Frontier.PolitenessDelay), int64(0))
	assert.Greater(t, cfg.Frontier.MaxConcurrency, 0)
	
	// Test that storage config has required fields
	assert.NotEmpty(t, cfg.Storage.RedisURL)
	assert.GreaterOrEqual(t, cfg.Storage.RedisDB, 0)
}

func TestEnvironmentIsolation(t *testing.T) {
	// Save original env state
	originalEnv := make(map[string]string)
	envKeys := []string{
		"FRONTIER_MAX_URLS", "FRONTIER_MAX_RETRIES", "FRONTIER_DEFAULT_PRIORITY",
		"FRONTIER_CLEANUP_INTERVAL", "FRONTIER_POLITENESS_DELAY", "FRONTIER_MAX_CONCURRENCY",
		"FRONTIER_ENABLE_PERSISTENCE", "REDIS_URL", "REDIS_PASSWORD", "REDIS_DB",
	}
	
	for _, key := range envKeys {
		if value, exists := os.LookupEnv(key); exists {
			originalEnv[key] = value
		}
	}
	
	// Clear all env vars
	clearEnvVars()
	
	// Load config - should use defaults
	cfg1 := config.Load()
	
	// Set some env vars
	os.Setenv("FRONTIER_MAX_URLS", "999")
	os.Setenv("REDIS_URL", "test.redis.com")
	
	// Load config again - should use new values
	cfg2 := config.Load()
	
	// Configs should be different
	assert.NotEqual(t, cfg1.Frontier.MaxURLsInMemory, cfg2.Frontier.MaxURLsInMemory)
	assert.NotEqual(t, cfg1.Storage.RedisURL, cfg2.Storage.RedisURL)
	
	// Restore original environment
	clearEnvVars()
	for key, value := range originalEnv {
		os.Setenv(key, value)
	}
}

// Helper function to clear environment variables used in tests
func clearEnvVars() {
	envKeys := []string{
		"FRONTIER_MAX_URLS",
		"FRONTIER_MAX_RETRIES",
		"FRONTIER_DEFAULT_PRIORITY",
		"FRONTIER_CLEANUP_INTERVAL",
		"FRONTIER_POLITENESS_DELAY",
		"FRONTIER_MAX_CONCURRENCY",
		"FRONTIER_ENABLE_PERSISTENCE",
		"REDIS_URL",
		"REDIS_PASSWORD",
		"REDIS_DB",
		"TEST_STRING",
		"TEST_INT",
		"TEST_INT_INVALID",
		"TEST_INT_NEG",
		"TEST_INT_ZERO",
		"TEST_BOOL",
		"TEST_DURATION",
		"EMPTY_VAR",
	}
	
	for _, key := range envKeys {
		os.Unsetenv(key)
	}
}