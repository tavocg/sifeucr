// Package cmd configures and runs the sifeucr HTTP server.
package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "sifeucr",
	Short: "Run the sifeucr HTTP server",
	Long: `sifeucr serves REST APIs and small HTML pages.
Run sifeucr or sifeucr serve to start the server. Configure it using sifeucr.yaml,
SIFEUCR_ environment variables or command-line flags.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServerFromConfig()
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
}

func initConfig() {
	_ = godotenv.Load()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		for _, d := range findConfigDirs() {
			viper.AddConfigPath(d)
		}

		viper.SetConfigType("yaml")
		viper.SetConfigName("sifeucr")
	}

	viper.SetEnvPrefix("SIFEUCR")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	var notFound viper.ConfigFileNotFoundError
	if err != nil && !errors.As(err, &notFound) {
		cobra.CheckErr(err)
	}
}

func findConfigDirs() []string {
	dirs := []string{}

	if wd, err := os.Getwd(); err == nil && wd != "" {
		dirs = append(dirs, wd)
	}

	if xdgCfgDir := os.Getenv("XDG_CONFIG_HOME"); xdgCfgDir != "" {
		if !slices.Contains(dirs, xdgCfgDir) {
			dirs = append(dirs, xdgCfgDir)
		}
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		defCfgDir := filepath.Join(home, ".config")
		if !slices.Contains(dirs, defCfgDir) {
			dirs = append(dirs, defCfgDir)
		}
	}

	dirs = append(dirs, "/usr/local/etc", "/etc")

	return dirs
}

func mustBindPersistentFlag(key string, cmd *cobra.Command, name string) {
	flag := cmd.PersistentFlags().Lookup(name)
	if flag == nil {
		panic("missing persistent flag: " + name)
	}

	cobra.CheckErr(viper.BindPFlag(key, flag))
}
