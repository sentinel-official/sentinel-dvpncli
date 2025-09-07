package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/sentinel-official/sentinel-go-sdk/cmd"
	"github.com/sentinel-official/sentinel-go-sdk/core/config"
	"github.com/sentinel-official/sentinel-go-sdk/libs/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sentinel-official/sentinel-dvpncli/cmd/query"
	"github.com/sentinel-official/sentinel-dvpncli/cmd/service"
	"github.com/sentinel-official/sentinel-dvpncli/cmd/tx"
)

// NewRootCmd creates and returns the root command for the CLI.
func NewRootCmd(userDir string) *cobra.Command {
	// Declare variables for CLI flags
	var (
		homeDir   = filepath.Join(userDir, ".sentinel-dvpncli")
		logFormat = "text"
		logLevel  = "info"
	)

	// Initialize default configuration
	cfg := config.DefaultConfig()

	// Create the root command
	rootCmd := &cobra.Command{
		Use:          "sentinel-dvpncli",
		SilenceUsage: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// Initialize logger with selected format and level
			logger, err := log.NewLogger(cmd.OutOrStdout(), logFormat, logLevel)
			if err != nil {
				return fmt.Errorf("initializing logger: %w", err)
			}

			// Set the global logger instance
			log.SetLogger(logger)

			// Update the keyring configuration
			cfg.Keyring.HomeDir = homeDir
			cfg.Keyring.Input = cmd.InOrStdin()

			log.Info("Validating configuration")
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("validating config: %w", err)
			}

			return nil
		},
	}

	// Add subcommands
	rootCmd.AddCommand(
		cmd.NewKeysCmd(cfg.Keyring),
		cmd.NewVersionCmd(),
		query.NewRootCmd(cfg),
		service.NewConnectCmd(cfg),
		service.NewInspectCmd(cfg),
		tx.NewRootCmd(cfg),
	)

	// Add persistent flags
	rootCmd.PersistentFlags().StringVar(&homeDir, "home", homeDir, "home directory for application config and data")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log.format", logFormat, "format of the log output (json or text)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log.level", logLevel, "log level for output (debug, error, info, none, warn)")

	// Bind flags to global viper instance
	_ = viper.BindPFlag("home", rootCmd.PersistentFlags().Lookup("home"))
	_ = viper.BindPFlag("log.format", rootCmd.PersistentFlags().Lookup("log.format"))
	_ = viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log.level"))

	return rootCmd
}
