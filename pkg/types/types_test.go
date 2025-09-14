//nolint:testpackage // Package name "types" is conventional and needed for testing unexported functions
package types

import (
	"os"
	"testing"
)

func TestParseHostnames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "single hostname",
			input:    "example.com",
			expected: []string{"example.com"},
		},
		{
			name:     "multiple hostnames",
			input:    "example.com,test.org,demo.net",
			expected: []string{"demo.net", "example.com", "test.org"}, // sorted.
		},
		{
			name:     "hostnames with spaces",
			input:    " example.com , test.org , demo.net ",
			expected: []string{"demo.net", "example.com", "test.org"},
		},
		{
			name:     "duplicates removed",
			input:    "example.com,test.org,example.com",
			expected: []string{"example.com", "test.org"},
		},
		{
			name:     "case normalization",
			input:    "Example.COM,TEST.org",
			expected: []string{"example.com", "test.org"},
		},
		{
			name:     "empty items filtered",
			input:    "example.com,,test.org,",
			expected: []string{"example.com", "test.org"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseHostnames(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d hostnames, got %d", len(tt.expected), len(result))
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("expected hostname %d to be %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}

func TestBuildExpression(t *testing.T) {
	tests := []struct {
		name      string
		hostnames []string
		expected  string
	}{
		{
			name:      "empty hostnames",
			hostnames: []string{},
			expected:  "false",
		},
		{
			name:      "single hostname",
			hostnames: []string{"example.com"},
			expected:  `http.host in {"example.com"}`,
		},
		{
			name:      "multiple hostnames",
			hostnames: []string{"example.com", "test.org"},
			expected:  `http.host in {"example.com" "test.org"}`,
		},
		{
			name:      "special characters",
			hostnames: []string{"sub-domain.example.com", "test_site.org"},
			expected:  `http.host in {"sub-domain.example.com" "test_site.org"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildExpression(tt.hostnames)
			if result != tt.expected {
				t.Errorf("expected expression %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Save original env vars.
	originalHostnames := os.Getenv("DEST_HOSTNAMES")
	originalZoneID := os.Getenv("CLOUDFLARE_ZONE_ID")
	originalAPIToken := os.Getenv("CLOUDFLARE_API_TOKEN")

	// Cleanup function.
	defer func() {
		setEnv("DEST_HOSTNAMES", originalHostnames)
		setEnv("CLOUDFLARE_ZONE_ID", originalZoneID)
		setEnv("CLOUDFLARE_API_TOKEN", originalAPIToken)
	}()

	t.Run("missing required fields", func(t *testing.T) {
		// Clear all env vars.
		setEnv("DEST_HOSTNAMES", "")
		setEnv("CLOUDFLARE_ZONE_ID", "")
		setEnv("CLOUDFLARE_API_TOKEN", "")

		_, err := LoadConfig()
		if err == nil {
			t.Error("expected error for missing required fields")
		}
	})

	t.Run("missing zone ID", func(t *testing.T) {
		setEnv("DEST_HOSTNAMES", "test.com")
		setEnv("CLOUDFLARE_ZONE_ID", "")
		setEnv("CLOUDFLARE_API_TOKEN", "test-token")

		_, err := LoadConfig()
		if err == nil {
			t.Error("expected error for missing zone ID")
		}
	})

	t.Run("missing API token", func(t *testing.T) {
		setEnv("DEST_HOSTNAMES", "test.com")
		setEnv("CLOUDFLARE_ZONE_ID", "test-zone")
		setEnv("CLOUDFLARE_API_TOKEN", "")

		_, err := LoadConfig()
		if err == nil {
			t.Error("expected error for missing API token")
		}
	})

	t.Run("missing hostnames", func(t *testing.T) {
		setEnv("DEST_HOSTNAMES", "")
		setEnv("CLOUDFLARE_ZONE_ID", "test-zone")
		setEnv("CLOUDFLARE_API_TOKEN", "test-token")

		_, err := LoadConfig()
		if err == nil {
			t.Error("expected error for missing hostnames")
		}
	})

	t.Run("valid configuration", func(t *testing.T) {
		clearEnv()
		setEnv("CLOUDFLARE_ZONE_ID", "test-zone")
		setEnv("CLOUDFLARE_API_TOKEN", "test-token")
		setEnv("DEST_HOSTNAMES", "example.com,test.com")
		setEnv("CF_RULE_DEFAULT_ENABLED", "true")
		setEnv("HTTP_ADDR", ":9000")
		setEnv("RECONCILE_INTERVAL", "30s")

		config, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.CloudflareZoneID != "test-zone" {
			t.Errorf("expected zone ID %q, got %q", "test-zone", config.CloudflareZoneID)
		}

		if len(config.DestHostnames) != 2 {
			t.Errorf("expected 2 hostnames, got %d", len(config.DestHostnames))
		}
	})

	t.Run("invalid reconcile interval", func(t *testing.T) {
		clearEnv()
		setEnv("DEST_HOSTNAMES", "test.com")
		setEnv("CLOUDFLARE_ZONE_ID", "test-zone")
		setEnv("CLOUDFLARE_API_TOKEN", "test-token")
		setEnv("RECONCILE_INTERVAL", "invalid")

		_, err := LoadConfig()
		if err == nil {
			t.Error("expected error for invalid reconcile interval")
		}
	})
}

// Helper functions for testing.
func clearEnv() {
	os.Unsetenv("CLOUDFLARE_ZONE_ID")
	os.Unsetenv("CLOUDFLARE_API_TOKEN")
	os.Unsetenv("DEST_HOSTNAMES")
	os.Unsetenv("CF_RULE_DEFAULT_ENABLED")
	os.Unsetenv("HTTP_ADDR")
	os.Unsetenv("RECONCILE_INTERVAL")
	os.Unsetenv("NAMESPACE")
	os.Unsetenv("SERVICE_ACCOUNT_NAME")
}

func setEnv(key, value string) {
	os.Setenv(key, value)
}
