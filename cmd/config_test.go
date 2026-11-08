package cmd

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	accessTTLKey  = "auth.access-token-ttl"
	refreshTTLKey = "auth.refresh-token-ttl"
)

func TestConfigPrecedence(t *testing.T) {
	// initConfig loads .env from the working directory. Tests must not read a
	// developer's configuration or inherit TTL overrides from their shell.
	t.Chdir(t.TempDir())
	for _, key := range []string{"SIFEUCR_AUTH_ACCESS_TOKEN_TTL", "SIFEUCR_AUTH_REFRESH_TOKEN_TTL", "SIFEUCR_AUTH_ACCESS-TOKEN-TTL", "SIFEUCR_AUTH_REFRESH-TOKEN-TTL"} {
		t.Setenv(key, "")
	}
	const configFile = `
auth:
  access-token-ttl: 10m
  refresh-token-ttl: 11m
`

	tests := []struct {
		name        string
		env         map[string]string
		accessFlag  string
		refreshFlag string
		wantAccess  time.Duration
		wantRefresh time.Duration
	}{
		{
			name:        "config file",
			wantAccess:  10 * time.Minute,
			wantRefresh: 11 * time.Minute,
		},
		{
			name: "underscore environment",
			env: map[string]string{
				"SIFEUCR_AUTH_ACCESS_TOKEN_TTL":  "20m",
				"SIFEUCR_AUTH_REFRESH_TOKEN_TTL": "21m",
			},
			wantAccess:  20 * time.Minute,
			wantRefresh: 21 * time.Minute,
		},
		{
			name: "hyphen environment does not override normalized keys",
			env: map[string]string{
				"SIFEUCR_AUTH_ACCESS-TOKEN-TTL":  "22m",
				"SIFEUCR_AUTH_REFRESH-TOKEN-TTL": "23m",
			},
			wantAccess:  10 * time.Minute,
			wantRefresh: 11 * time.Minute,
		},
		{
			name: "flag wins over environment",
			env: map[string]string{
				"SIFEUCR_AUTH_ACCESS_TOKEN_TTL":  "20m",
				"SIFEUCR_AUTH_REFRESH_TOKEN_TTL": "21m",
			},
			accessFlag:  "30m",
			wantAccess:  30 * time.Minute,
			wantRefresh: 21 * time.Minute,
		},
		{
			name: "refresh flag wins over environment",
			env: map[string]string{
				"SIFEUCR_AUTH_ACCESS_TOKEN_TTL":  "20m",
				"SIFEUCR_AUTH_REFRESH_TOKEN_TTL": "21m",
			},
			refreshFlag: "31m",
			wantAccess:  20 * time.Minute,
			wantRefresh: 31 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)

			cmd := &cobra.Command{Use: "test"}
			registerSettings(cmd)

			path := filepath.Join(t.TempDir(), "sifeucr.yaml")
			if err := os.WriteFile(path, []byte(configFile), 0o600); err != nil {
				t.Fatal(err)
			}
			previousCfgFile := cfgFile
			cfgFile = path
			t.Cleanup(func() { cfgFile = previousCfgFile })
			initConfig()

			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			if tt.accessFlag != "" {
				if err := cmd.PersistentFlags().Set("auth-access-ttl", tt.accessFlag); err != nil {
					t.Fatal(err)
				}
			}
			if tt.refreshFlag != "" {
				if err := cmd.PersistentFlags().Set("auth-refresh-ttl", tt.refreshFlag); err != nil {
					t.Fatal(err)
				}
			}

			if got := viper.GetDuration(accessTTLKey); got != tt.wantAccess {
				t.Errorf("access TTL = %s, want %s", got, tt.wantAccess)
			}
			if got := viper.GetDuration(refreshTTLKey); got != tt.wantRefresh {
				t.Errorf("refresh TTL = %s, want %s", got, tt.wantRefresh)
			}
		})
	}
}

func TestSettingsDefaults(t *testing.T) {
	for _, key := range []string{"SIFEUCR_AUTH_ACCESS_TOKEN_TTL", "SIFEUCR_AUTH_REFRESH_TOKEN_TTL", "SIFEUCR_PORT", "SIFEUCR_DB"} {
		t.Setenv(key, "")
	}
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := &cobra.Command{Use: "test"}
	registerSettings(cmd)

	if got := viper.GetDuration(accessTTLKey); got != 15*time.Minute {
		t.Errorf("access TTL default = %s, want 15m", got)
	}
	if got := viper.GetDuration(refreshTTLKey); got != 30*24*time.Hour {
		t.Errorf("refresh TTL default = %s, want 720h", got)
	}
	if got := viper.GetInt("port"); got != 3080 {
		t.Errorf("port default = %d, want 3080", got)
	}
	if got := viper.GetString("db"); got != "data/sifeucr.sqlite" {
		t.Errorf("db default = %q, want data/sifeucr.sqlite", got)
	}
}

func TestResolveOTPSecret(t *testing.T) {
	authSecret := strings.Repeat("a", 32)
	otpSecret := strings.Repeat("b", 32)

	tests := []struct {
		name          string
		cfg           serverConfig
		wantErr       string
		wantSecret    string
		wantGenerated bool
		wantWarnings  int
	}{
		{
			name:    "short auth secret outside development",
			cfg:     serverConfig{OTPSecret: otpSecret},
			wantErr: "auth.secret must contain at least 32 bytes",
		},
		{
			name:    "31 byte auth secret outside development",
			cfg:     serverConfig{AuthSecret: strings.Repeat("a", 31), OTPSecret: otpSecret},
			wantErr: "auth.secret must contain at least 32 bytes",
		},
		{
			name:    "empty OTP secret outside development",
			cfg:     serverConfig{AuthSecret: authSecret},
			wantErr: "otp.secret must contain at least 32 bytes",
		},
		{
			name:    "short OTP secret outside development",
			cfg:     serverConfig{AuthSecret: authSecret, OTPSecret: "short"},
			wantErr: "otp.secret must contain at least 32 bytes",
		},
		{
			name:    "31 byte OTP secret outside development",
			cfg:     serverConfig{AuthSecret: authSecret, OTPSecret: strings.Repeat("b", 31)},
			wantErr: "otp.secret must contain at least 32 bytes",
		},
		{
			name:    "equal secrets",
			cfg:     serverConfig{AuthSecret: authSecret, OTPSecret: authSecret},
			wantErr: "must differ",
		},
		{
			name:         "equal nonempty development secrets",
			cfg:          serverConfig{Dev: true, AuthSecret: "same-dev-canary", OTPSecret: "same-dev-canary"},
			wantErr:      "must differ",
			wantWarnings: 2,
		},
		{
			name:       "valid secrets",
			cfg:        serverConfig{AuthSecret: authSecret, OTPSecret: otpSecret},
			wantSecret: otpSecret,
		},
		{
			name:         "development allows short secrets",
			cfg:          serverConfig{Dev: true, AuthSecret: "dev-auth-canary", OTPSecret: "dev-otp-canary"},
			wantSecret:   "dev-otp-canary",
			wantWarnings: 2,
		},
		{
			name:         "development allows empty auth secret",
			cfg:          serverConfig{Dev: true, OTPSecret: otpSecret},
			wantSecret:   otpSecret,
			wantWarnings: 1,
		},
		{
			name:          "development generates a random secret",
			cfg:           serverConfig{Dev: true},
			wantGenerated: true,
			wantWarnings:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			got, err := resolveOTPSecret(tt.cfg, logger)
			for _, secret := range []string{tt.cfg.AuthSecret, tt.cfg.OTPSecret} {
				if secret != "" && strings.Contains(logs.String(), secret) {
					t.Error("logs exposed a signing secret")
				}
			}
			warnings := 0
			decoder := json.NewDecoder(&logs)
			for {
				var record struct{ Level string }
				err := decoder.Decode(&record)
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if record.Level != "WARN" {
					t.Errorf("log level = %q, want WARN", record.Level)
				}
				warnings++
			}
			if warnings != tt.wantWarnings {
				t.Errorf("warnings = %d, want %d", warnings, tt.wantWarnings)
			}
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantGenerated {
				if len(got) != 64 {
					t.Fatalf("generated secret length = %d, want 64 hex characters", len(got))
				}
				if _, err := hex.DecodeString(got); err != nil {
					t.Fatal(err)
				}
				second, err := resolveOTPSecret(tt.cfg, logger)
				if err != nil || second == got {
					t.Fatalf("development key was reused: %v", err)
				}
				return
			}
			if got != tt.wantSecret {
				t.Errorf("secret = %q, want %q", got, tt.wantSecret)
			}
		})
	}
}
