package query

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/sentinel-official/sentinel-go-sdk/config"
	"github.com/sentinel-official/sentinel-go-sdk/core"
	"github.com/spf13/cobra"
)

// querySubscriptionCmd returns a command to query a specific subscription by ID
func querySubscriptionCmd(cfg *config.Config) *cobra.Command {
	h := &Handler{
		Command: cobra.Command{
			Use:   "subscription [id]",
			Short: "Query a subscription",
			Args:  cobra.ExactArgs(1),
		},
		RunE: func(cmd *cobra.Command, args []string, c *core.Client) (interface{}, error) {
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parsing id %q: %w", args[0], err)
			}

			// Fetch subscription details
			item, err := c.Subscription(cmd.Context(), id)
			if err != nil {
				return nil, fmt.Errorf("querying subscription %d: %w", id, err)
			}

			return item, nil
		},
	}

	cmd := NewCommand(cfg, h)

	return cmd
}

// querySubscriptionsCmd returns a command to query all subscriptions
func querySubscriptionsCmd(cfg *config.Config) *cobra.Command {
	pageReq := query.PageRequest{Limit: 10}
	var accAddrStr string
	var planID uint64

	h := &Handler{
		Command: cobra.Command{
			Use:   "subscriptions",
			Short: "Query all subscriptions",
		},
		RunE: func(cmd *cobra.Command, args []string, c *core.Client) (interface{}, error) {
			// Query for account
			if accAddrStr != "" {
				accAddr, err := types.AccAddressFromBech32(accAddrStr)
				if err != nil {
					return nil, fmt.Errorf("parsing account addr %q: %w", accAddrStr, err)
				}

				items, pageRes, err := c.SubscriptionsForAccount(cmd.Context(), accAddr, &pageReq)
				if err != nil {
					return nil, fmt.Errorf("querying subscriptions for account %q: %w", accAddr.String(), err)
				}

				return PrepareResponse(items, pageRes), nil
			}

			// Query for plan
			if planID != 0 {
				items, pageRes, err := c.SubscriptionsForPlan(cmd.Context(), planID, &pageReq)
				if err != nil {
					return nil, fmt.Errorf("querying subscriptions for plan %d: %w", planID, err)
				}

				return PrepareResponse(items, pageRes), nil
			}

			// Query all
			items, pageRes, err := c.Subscriptions(cmd.Context(), &pageReq)
			if err != nil {
				return nil, fmt.Errorf("querying subscriptions: %w", err)
			}

			return PrepareResponse(items, pageRes), nil
		},
	}

	cmd := NewCommand(cfg, h)
	cmd.Flags().Uint64Var(&planID, "plan-id", planID, "filter subscriptions by plan identifier")
	cmd.Flags().StringVar(&accAddrStr, "account-addr", accAddrStr, "filter subscriptions by account address")

	SetPageRequestFlags(cmd.Flags(), "subscriptions", &pageReq)

	return cmd
}
