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

// queryNodeCmd returns a command to query a specific node by address.
func queryNodeCmd(cfg *config.Config) *cobra.Command {
	h := &Handler{
		Command: cobra.Command{
			Use:   "node [addr]",
			Short: "Query a node",
			Args:  cobra.ExactArgs(1),
		},
		RunE: func(cmd *cobra.Command, args []string, c *core.Client) (interface{}, error) {
			addr, err := types.NodeAddressFromBech32(args[0])
			if err != nil {
				return nil, fmt.Errorf("parsing node addr %q: %w", args[0], err)
			}

			// Fetch node details
			item, err := c.Node(cmd.Context(), addr)
			if err != nil {
				return nil, fmt.Errorf("querying node %q: %w", addr.String(), err)
			}

			return item, nil
		},
	}

	cmd := NewCommand(cfg, h)

	return cmd
}

// queryNodesCmd returns a command to query all nodes.
func queryNodesCmd(cfg *config.Config) *cobra.Command {
	pageReq := query.PageRequest{Limit: 10}

	var (
		planID    uint64
		statusStr string
	)

	h := &Handler{
		Command: cobra.Command{
			Use:   "nodes",
			Short: "Query all nodes",
		},
		RunE: func(cmd *cobra.Command, args []string, c *core.Client) (interface{}, error) {
			// Query for plan
			if planID != 0 {
				items, pageRes, err := c.NodesForPlan(cmd.Context(), planID, v1.StatusFromString(statusStr), &pageReq)
				if err != nil {
					return nil, fmt.Errorf("querying nodes for plan %d: %w", planID, err)
				}

				return PrepareResponse(items, pageRes), nil
			}

			// Query all
			items, pageRes, err := c.Nodes(cmd.Context(), v1.StatusFromString(statusStr), &pageReq)
			if err != nil {
				return nil, fmt.Errorf("querying nodes: %w", err)
			}

			return PrepareResponse(items, pageRes), nil
		},
	}

	cmd := NewCommand(cfg, h)
	cmd.Flags().Uint64Var(&planID, "plan-id", planID, "filter nodes by plan identifier")
	cmd.Flags().StringVar(&statusStr, "status", statusStr, "filter nodes by status (e.g. active, inactive)")

	SetPageRequestFlags(cmd.Flags(), "nodes", &pageReq)

	return cmd
}
