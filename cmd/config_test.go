package cmd

import (
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
	t.Setenv("SIFEUCR_CORS_ORIGINS", "https://one.example,https://two.example")
	viper.SetEnvPrefix("SIFEUCR")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()
	if got := configuredCORSOrigins(); len(got) != 2 || got[0] != "https://one.example" || got[1] != "https://two.example" {
		t.Fatalf("CORS environment = %v", got)
	}

}
