package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sifeucr/database/provider"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use: "migrate", Short: "Apply database migrations and exit", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			db, err := provider.Open(ctx, viper.GetString("db"))
			if err != nil {
				return err
			}
			return db.Close()
		},
	})
}
