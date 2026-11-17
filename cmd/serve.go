package cmd

import (
	"strings"
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
		{"http.drain-delay", "drain-delay", "serve while readiness withdrawal propagates on shutdown (ignored with --dev)", 5 * time.Second},
		{"http.shutdown-timeout", "shutdown-timeout", "grace period for active HTTP requests after draining", 25 * time.Second},
		{"dev", "dev", "enable debug text logs, allow all origins and development signing secrets", false},
		{"host", "host", "bind host", ""},
		{"port", "port", "bind port", 3080},
		{"logfmt", "logfmt", "log format (defaults to text with --dev)", "json"},
		{"loglvl", "loglvl", "log level (defaults to debug with --dev)", "info"},
		{"auth.secret", "auth-secret", "JWT signing secret", ""},
		{"otp.secret", "otp-secret", "independent HMAC secret for verification codes", ""},
		{"auth.access-token-ttl", "auth-access-ttl", "access token TTL", 15 * time.Minute},
		{"auth.refresh-token-ttl", "auth-refresh-ttl", "refresh token TTL", 30 * 24 * time.Hour},
		{"mail.smtp.address", "smtp-address", "SMTP host:port; empty disables delivery", ""},
		{"mail.smtp.user", "smtp-user", "SMTP username", ""},
		{"mail.smtp.password", "smtp-password", "SMTP password", ""},
		{"mail.smtp.starttls", "smtp-starttls", "use required STARTTLS instead of implicit TLS", false},
		{"mail.from", "mail-from", "sender address for application emails", "no-reply@example.com"},
		{"cors.origins", "cors-origins", "allowed CORS origins (comma-separated); empty disables CORS", []string{}},
		{"csrf.origins", "csrf-origins", "origins allowed to post browser forms (comma-separated); empty compares Origin with Host", []string{}},
		{"root", "root", "route prefix to mount the sifeucr under", ""},
		{"database.auto-migrate", "auto-migrate", "apply migrations at server startup", true},
		{"database.max-connections", "db-max-connections", "maximum SQL connections per replica (SQLite uses one)", 10},
		{"cache.enabled", "cache", "cache profiles locally; other replicas may serve stale profiles", false},
		{"db", "db", "database DSN", "data/sifeucr.sqlite"},
	}
	flags := cmd.PersistentFlags()
	for _, setting := range settings {
		viper.SetDefault(setting.key, setting.value)
		switch value := setting.value.(type) {
		case string:
			flags.String(setting.flag, value, setting.usage)
		case []string:
			flags.StringSlice(setting.flag, value, setting.usage)
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
	level, format := "info", "json"
	if viper.GetBool("dev") {
		level, format = "debug", "text"
	}
	// Only change defaults: flags, environment and config retain their precedence.
	viper.SetDefault("loglvl", level)
	viper.SetDefault("logfmt", format)
	return runServer(serverConfig{
		SMTPAddress:     viper.GetString("mail.smtp.address"),
		SMTPUser:        viper.GetString("mail.smtp.user"),
		SMTPPassword:    viper.GetString("mail.smtp.password"),
		SMTPStartTLS:    viper.GetBool("mail.smtp.starttls"),
		DrainDelay:      viper.GetDuration("http.drain-delay"),
		ShutdownTimeout: viper.GetDuration("http.shutdown-timeout"),
		DatabaseDSN:     viper.GetString("db"),
		AutoMigrate:     viper.GetBool("database.auto-migrate"),
		MaxConnections:  viper.GetInt("database.max-connections"),
		CacheEnabled:    viper.GetBool("cache.enabled"),
		Dev:             viper.GetBool("dev"),
		LogLevel:        viper.GetString("loglvl"),
		LogFormat:       viper.GetString("logfmt"),
		AuthSecret:      viper.GetString("auth.secret"),
		OTPSecret:       viper.GetString("otp.secret"),
		AccessTokenTTL:  viper.GetDuration("auth.access-token-ttl"),
		RefreshTokenTTL: viper.GetDuration("auth.refresh-token-ttl"),
		RootPrefix:      viper.GetString("root"),
		CORSOrigins:     configuredCORSOrigins(),
		CSRFOrigins:     configuredOrigins("csrf.origins"),
		MailFrom:        viper.GetString("mail.from"),
		Host:            viper.GetString("host"),
		Port:            viper.GetInt("port"),
	})
}

// Viper leaves environment overrides as strings; accept comma-separated origins
// consistently for flags, environment variables and YAML lists.
func configuredCORSOrigins() []string {
	return configuredOrigins("cors.origins")
}

func configuredOrigins(key string) []string {
	var origins []string
	for _, value := range viper.GetStringSlice(key) {
		for _, origin := range strings.Split(value, ",") {
			if origin = strings.TrimSpace(origin); origin != "" {
				origins = append(origins, origin)
			}
		}
	}
	return origins
}
