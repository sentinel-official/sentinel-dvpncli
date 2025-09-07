package query

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/sentinel-official/sentinel-go-sdk/config"
	"github.com/sentinel-official/sentinel-go-sdk/core"
	"github.com/sentinel-official/sentinelhub/v12/types"
	"github.com/sentinel-official/sentinelhub/v12/types/v1"
	"github.com/spf13/cobra"
)

// queryProviderCmd returns a command to query a specific provider by address.
func queryProviderCmd(cfg *config.Config) *cobra.Command {
	h := &Handler{
		Command: cobra.Command{
			Use:   "provider [addr]",
			Short: "Query a provider",
			Args:  cobra.ExactArgs(1),
		},
		RunE: func(cmd *cobra.Command, args []string, c *core.Client) (interface{}, error) {
			addr, err := types.ProvAddressFromBech32(args[0])
			if err != nil {
				return nil, fmt.Errorf("parsing provider addr %q: %w", args[0], err)
			}

			// Fetch provider details
			item, err := c.Provider(cmd.Context(), addr)
			if err != nil {
				return nil, fmt.Errorf("querying provider: %w", err)
			}

			return item, nil
		},
	}

	cmd := NewCommand(cfg, h)

	return cmd
}

// queryProvidersCmd returns a command to query all providers.
func queryProvidersCmd(cfg *config.Config) *cobra.Command {
	pageReq := query.PageRequest{Limit: 10}

	var statusStr string

	h := &Handler{
		Command: cobra.Command{
			Use:   "providers",
			Short: "Query all providers",
		},
		RunE: func(cmd *cobra.Command, args []string, c *core.Client) (interface{}, error) {
			// Query all
			items, pageRes, err := c.Providers(cmd.Context(), v1.StatusFromString(statusStr), &pageReq)
			if err != nil {
				return nil, fmt.Errorf("querying providers: %w", err)
			}

			return PrepareResponse(items, pageRes), nil
		},
	}

	cmd := NewCommand(cfg, h)
	cmd.Flags().StringVar(&statusStr, "status", statusStr, "filter providers by status (e.g. active, inactive)")

	SetPageRequestFlags(cmd.Flags(), "providers", &pageReq)

	return cmd
}
