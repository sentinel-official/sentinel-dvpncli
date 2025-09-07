package tx

import (
	"github.com/sentinel-official/sentinel-go-sdk/core/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewRootCmd initializes and returns the root query command.
func NewRootCmd(cfg *config.Config) *cobra.Command {
	outputFormat := "text"

	rootCmd := &cobra.Command{
		Use:          "tx",
		Short:        "Sub-commands for transaction",
		SilenceUsage: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	// Add all query subcommands
	rootCmd.AddCommand(
		txSessionCancelCmd(cfg),
		txSessionStartCmd(cfg),
		txSubscriptionCancelCmd(cfg),
		txSubscriptionRenewCmd(cfg),
		txSubscriptionShareCmd(cfg),
		txSubscriptionStartCmd(cfg),
		txSubscriptionUpdateCmd(cfg),
	)

	// Set default configuration flags
	cfg.SetForFlags(rootCmd.PersistentFlags())

	// Add output format flag with description
	rootCmd.PersistentFlags().StringVar(&outputFormat, "output-format", outputFormat, "format for tx output (text/json)")
	_ = viper.BindPFlag("output-format", rootCmd.PersistentFlags().Lookup("output-format"))

	return rootCmd
}
