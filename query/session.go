package query

import (
	"fmt"
	"strconv"

	cosmossdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/sentinel-official/sentinel-go-sdk/core"
	"github.com/sentinel-official/sentinel-go-sdk/core/config"
	"github.com/sentinel-official/sentinelhub/v12/types"
	"github.com/spf13/cobra"
)

// querySessionCmd returns a command to query a specific session by ID.
func querySessionCmd(cfg *config.Config) *cobra.Command {
	h := &Handler{
		Command: cobra.Command{
			Use:   "session [id]",
			Short: "Query a session",
			Args:  cobra.ExactArgs(1),
		},
		RunE: func(cmd *cobra.Command, args []string, c *core.Client) (any, error) {
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parsing id %q: %w", args[0], err)
			}

			// Fetch session details
			item, err := c.Session(cmd.Context(), id)
			if err != nil {
				return nil, fmt.Errorf("querying session: %w", err)
			}

			return item, nil
		},
	}

	cmd := NewCommand(cfg, h)

	return cmd
}

// querySessionsCmd returns a command to query all sessions.
func querySessionsCmd(cfg *config.Config) *cobra.Command {
	pageReq := query.PageRequest{Limit: 10}

	var (
		accAddrStr     string
		nodeAddrStr    string
		subscriptionID uint64
	)

	h := &Handler{
		Command: cobra.Command{
			Use:   "sessions",
			Short: "Query all sessions",
		},
		RunE: func(cmd *cobra.Command, args []string, c *core.Client) (any, error) {
			// Query for account
			if accAddrStr != "" {
				accAddr, err := cosmossdk.AccAddressFromBech32(accAddrStr)
				if err != nil {
					return nil, fmt.Errorf("parsing account addr %q: %w", accAddrStr, err)
				}

				// Query for subscription allocation
				if subscriptionID != 0 {
					items, pageRes, err := c.SessionsForSubscriptionAllocation(cmd.Context(), subscriptionID, accAddr, &pageReq)
					if err != nil {
						return nil, fmt.Errorf("querying sessions for subscription allocation %q: %w", fmt.Sprintf("%d/%s", subscriptionID, accAddr.String()), err)
					}

					return PrepareResponse(items, pageRes), nil
				}

				items, pageRes, err := c.SessionsForAccount(cmd.Context(), accAddr, &pageReq)
				if err != nil {
					return nil, fmt.Errorf("querying sessions for account %q: %w", accAddr.String(), err)
				}

				return PrepareResponse(items, pageRes), nil
			}

			// Query for node
			if nodeAddrStr != "" {
				nodeAddr, err := types.NodeAddressFromBech32(nodeAddrStr)
				if err != nil {
					return nil, fmt.Errorf("parsing node addr %q: %w", nodeAddrStr, err)
				}

				items, pageRes, err := c.SessionsForNode(cmd.Context(), nodeAddr, &pageReq)
				if err != nil {
					return nil, fmt.Errorf("querying sessions for node %q: %w", nodeAddr.String(), err)
				}

				return PrepareResponse(items, pageRes), nil
			}

			// Query for subscription
			if subscriptionID != 0 {
				items, pageRes, err := c.SessionsForSubscription(cmd.Context(), subscriptionID, &pageReq)
				if err != nil {
					return nil, fmt.Errorf("querying sessions for subscription %d: %w", subscriptionID, err)
				}

				return PrepareResponse(items, pageRes), nil
			}

			// Query all
			items, pageRes, err := c.Sessions(cmd.Context(), &pageReq)
			if err != nil {
				return nil, fmt.Errorf("querying sessions: %w", err)
			}

			return PrepareResponse(items, pageRes), nil
		},
	}

	cmd := NewCommand(cfg, h)
	cmd.Flags().StringVar(&accAddrStr, "account-addr", accAddrStr, "filter sessions by account address")
	cmd.Flags().StringVar(&nodeAddrStr, "node-addr", nodeAddrStr, "filter sessions by node address")
	cmd.Flags().Uint64Var(&subscriptionID, "subscription-id", subscriptionID, "filter sessions by subscription identifier")

	SetPageRequestFlags(cmd.Flags(), "sessions", &pageReq)

	return cmd
}
