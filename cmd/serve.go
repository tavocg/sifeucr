package cmd

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServerFromConfig()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	registerSettings(rootCmd)
}

// registerSettings owns the Viper key, CLI spelling, help and default value for
// every runtime setting.
func registerSettings(cmd *cobra.Command) {
	settings := []struct {
		key, flag, usage string
		value            any
	}{
		{"dev", "dev", "enable development features and allow empty or short signing secrets", false},
		{"host", "host", "bind host", ""},
		{"port", "port", "bind port", 3080},
		{"logfmt", "logfmt", "log format", "json"},
		{"loglvl", "loglvl", "log level", "info"},
		{"auth.secret", "auth-secret", "JWT signing secret", ""},
		{"otp.secret", "otp-secret", "independent HMAC secret for verification codes", ""},
		{"auth.access-token-ttl", "auth-access-ttl", "access token TTL", 15 * time.Minute},
		{"auth.refresh-token-ttl", "auth-refresh-ttl", "refresh token TTL", 30 * 24 * time.Hour},
		{"mail.from", "mail-from", "sender address for application emails", "no-reply@example.com"},
		{"root", "root", "route prefix to mount the sifeucr under", ""},
		{"db", "db", "database DSN", "data/sifeucr.sqlite"},
	}
	flags := cmd.PersistentFlags()
	for _, setting := range settings {
		viper.SetDefault(setting.key, setting.value)
		switch value := setting.value.(type) {
		case string:
			flags.String(setting.flag, value, setting.usage)
		case bool:
			flags.Bool(setting.flag, value, setting.usage)
		case int:
			flags.Int(setting.flag, value, setting.usage)
		case time.Duration:
			flags.Duration(setting.flag, value, setting.usage)
		default:
			panic("unsupported configuration default: " + setting.key)
		}
		mustBindPersistentFlag(setting.key, cmd, setting.flag)
	}
}

func runServerFromConfig() error {
	return runServer(serverConfig{
		DatabaseDSN:     viper.GetString("db"),
		Dev:             viper.GetBool("dev"),
		LogLevel:        viper.GetString("loglvl"),
		LogFormat:       viper.GetString("logfmt"),
		AuthSecret:      viper.GetString("auth.secret"),
		OTPSecret:       viper.GetString("otp.secret"),
		AccessTokenTTL:  viper.GetDuration("auth.access-token-ttl"),
		RefreshTokenTTL: viper.GetDuration("auth.refresh-token-ttl"),
		RootPrefix:      viper.GetString("root"),
		MailFrom:        viper.GetString("mail.from"),
		Host:            viper.GetString("host"),
		Port:            viper.GetInt("port"),
	})
}
