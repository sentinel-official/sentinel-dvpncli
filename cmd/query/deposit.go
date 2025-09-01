package query

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/sentinel-official/sentinel-go-sdk/config"
	"github.com/sentinel-official/sentinel-go-sdk/core"
	"github.com/spf13/cobra"
)

// queryDepositCmd returns a command to query a specific deposit by address
func queryDepositCmd(cfg *config.Config) *cobra.Command {
	h := &Handler{
		Command: cobra.Command{
			Use:   "deposit [addr]",
			Short: "Query a deposit",
			Args:  cobra.ExactArgs(1),
		},
		RunE: func(cmd *cobra.Command, args []string, c *core.Client) (interface{}, error) {
			addr, err := types.AccAddressFromBech32(args[0])
			if err != nil {
				return nil, fmt.Errorf("parse addr %q: %w", args[0], err)
			}

			// Fetch deposit details
			item, err := c.Deposit(cmd.Context(), addr)
			if err != nil {
				return nil, fmt.Errorf("querying deposit %q: %w", addr.String(), err)
			}

			return item, nil
		},
	}

	cmd := NewCommand(cfg, h)

	return cmd
}

// queryDepositsCmd returns a command to query all deposits
func queryDepositsCmd(cfg *config.Config) *cobra.Command {
	pageReq := query.PageRequest{Limit: 10}

	h := &Handler{
		Command: cobra.Command{
			Use:   "deposits",
			Short: "Query all deposits",
		},
		RunE: func(cmd *cobra.Command, args []string, c *core.Client) (interface{}, error) {
			// Query all
			items, pageRes, err := c.Deposits(cmd.Context(), &pageReq)
			if err != nil {
				return nil, fmt.Errorf("querying deposits: %w", err)
			}

			return PrepareResponse(items, pageRes), nil
		},
	}

	cmd := NewCommand(cfg, h)
	SetPageRequestFlags(cmd.Flags(), "deposits", &pageReq)

	return cmd
}
