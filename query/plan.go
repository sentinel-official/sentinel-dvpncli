package query

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/sentinel-official/sentinel-go-sdk/core"
	"github.com/sentinel-official/sentinel-go-sdk/core/config"
	"github.com/sentinel-official/sentinelhub/v12/types"
	"github.com/sentinel-official/sentinelhub/v12/types/v1"
	"github.com/spf13/cobra"
)

// queryPlanCmd returns a command to query a specific plan by ID.
func queryPlanCmd(cfg *config.Config) *cobra.Command {
	h := &Handler{
		Command: cobra.Command{
			Use:   "plan [id]",
			Short: "Query a plan",
			Args:  cobra.ExactArgs(1),
		},
		RunE: func(cmd *cobra.Command, args []string, c *core.Client) (any, error) {
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parsing id %q: %w", args[0], err)
			}

			// Fetch plan details
			item, err := c.Plan(cmd.Context(), id)
			if err != nil {
				return nil, fmt.Errorf("querying plan: %w", err)
			}

			return item, nil
		},
	}

	cmd := NewCommand(cfg, h)

	return cmd
}

// queryPlansCmd returns a command to query all plans.
func queryPlansCmd(cfg *config.Config) *cobra.Command {
	pageReq := query.PageRequest{Limit: 10}

	var (
		provAddrStr string
		statusStr   string
	)

	h := &Handler{
		Command: cobra.Command{
			Use:   "plans",
			Short: "Query all plans",
		},
		RunE: func(cmd *cobra.Command, args []string, c *core.Client) (any, error) {
			// Query for provider
			if provAddrStr != "" {
				provAddr, err := types.ProvAddressFromBech32(provAddrStr)
				if err != nil {
					return nil, fmt.Errorf("parsing provider addr %q: %w", provAddrStr, err)
				}

				items, pageRes, err := c.PlansForProvider(cmd.Context(), provAddr, v1.StatusFromString(statusStr), &pageReq)
				if err != nil {
					return nil, fmt.Errorf("querying plans for provider %q: %w", provAddr.String(), err)
				}

				return PrepareResponse(items, pageRes), nil
			}

			// Query all
			items, pageRes, err := c.Plans(cmd.Context(), v1.StatusFromString(statusStr), &pageReq)
			if err != nil {
				return nil, fmt.Errorf("querying plans: %w", err)
			}

			return PrepareResponse(items, pageRes), nil
		},
	}

	cmd := NewCommand(cfg, h)
	cmd.Flags().StringVar(&provAddrStr, "provider-addr", provAddrStr, "filter nodes by provider address")
	cmd.Flags().StringVar(&statusStr, "status", statusStr, "filter nodes by status (e.g. active, inactive)")

	SetPageRequestFlags(cmd.Flags(), "plans", &pageReq)

	return cmd
}
