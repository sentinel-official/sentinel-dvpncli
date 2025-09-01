package query

import (
	"github.com/sentinel-official/sentinel-go-sdk/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewRootCmd initializes and returns the root query command
func NewRootCmd(cfg *config.Config) *cobra.Command {
	outputFormat := "text"

	rootCmd := &cobra.Command{
		Use:          "query",
		Short:        "Sub-commands for query",
		SilenceUsage: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	// Add all query subcommands
	rootCmd.AddCommand(
		queryDepositCmd(cfg),
		queryDepositsCmd(cfg),
		queryNodeCmd(cfg),
		queryNodesCmd(cfg),
		queryParamsCmd(cfg),
		queryPlanCmd(cfg),
		queryPlansCmd(cfg),
		queryProviderCmd(cfg),
		queryProvidersCmd(cfg),
		querySessionCmd(cfg),
		querySessionsCmd(cfg),
		querySubscriptionCmd(cfg),
		querySubscriptionsCmd(cfg),
	)

	// Set query and RPC-related flags
	cfg.Query.SetForFlags(rootCmd.PersistentFlags())
	cfg.RPC.SetForFlags(rootCmd.PersistentFlags())

	// Add output format flag with description
	rootCmd.PersistentFlags().StringVar(&outputFormat, "output-format", outputFormat, "format for query output (text/json)")
	_ = viper.BindPFlag("output-format", rootCmd.PersistentFlags().Lookup("output-format"))

	return rootCmd
}
