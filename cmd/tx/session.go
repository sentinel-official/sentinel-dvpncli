package tx

import (
	"fmt"
	"strconv"

	cosmossdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/sentinel-official/sentinel-go-sdk/config"
	"github.com/sentinel-official/sentinelhub/v12/types"
	"github.com/sentinel-official/sentinelhub/v12/types/v1"
	nodev3 "github.com/sentinel-official/sentinelhub/v12/x/node/types/v3"
	planv3 "github.com/sentinel-official/sentinelhub/v12/x/plan/types/v3"
	sessionv3 "github.com/sentinel-official/sentinelhub/v12/x/session/types/v3"
	subscriptionv3 "github.com/sentinel-official/sentinelhub/v12/x/subscription/types/v3"
	"github.com/spf13/cobra"
)

func txSessionCancelCmd(cfg *config.Config) *cobra.Command {
	h := &Handler{
		Command: cobra.Command{
			Use:   "session-cancel [id]",
			Short: "Cancel a session",
			Args:  cobra.ExactArgs(1),
		},
		RunE: func(cmd *cobra.Command, args []string, fromAddr cosmossdk.AccAddress) (cosmossdk.Msg, error) {
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parsing id %q: %w", args[0], err)
			}

			return sessionv3.NewMsgCancelSessionRequest(
				fromAddr.Bytes(),
				id,
			), nil
		},
	}

	cmd := NewCommand(cfg, h)

	return cmd
}

func txSessionStartCmd(cfg *config.Config) *cobra.Command {
	var (
		denom                 string
		gigabytes             int64
		hours                 int64
		maxPriceStr           string
		planID                uint64
		renewalPricePolicyStr string
		subscriptionID        uint64
	)

	h := &Handler{
		Command: cobra.Command{
			Use:   "session-start [node-addr]",
			Short: "Start a session",
			Args:  cobra.ExactArgs(1),
		},
		RunE: func(cmd *cobra.Command, args []string, fromAddr cosmossdk.AccAddress) (cosmossdk.Msg, error) {
			nodeAddr, err := types.NodeAddressFromBech32(args[0])
			if err != nil {
				return nil, fmt.Errorf("parsing node addr %q: %w", args[0], err)
			}

			if planID != 0 {
				return planv3.NewMsgStartSessionRequest(
					fromAddr.Bytes(),
					planID,
					denom,
					v1.RenewalPricePolicyFromString(renewalPricePolicyStr),
					nodeAddr.Bytes(),
				), nil
			}

			if subscriptionID != 0 {
				return subscriptionv3.NewMsgStartSessionRequest(
					fromAddr.Bytes(),
					subscriptionID,
					nodeAddr.Bytes(),
				), nil
			}

			maxPrice, err := v1.NewPriceFromString(maxPriceStr)
			if err != nil {
				return nil, fmt.Errorf("parsing max price %q: %w", maxPriceStr, err)
			}

			return nodev3.NewMsgStartSessionRequest(
				fromAddr.Bytes(),
				nodeAddr.Bytes(),
				gigabytes,
				hours,
				maxPrice,
			), nil
		},
	}

	cmd := NewCommand(cfg, h)
	cmd.Flags().StringVar(&denom, "denom", denom, "denomination of the currency to be used for payment")
	cmd.Flags().Int64Var(&gigabytes, "gigabytes", gigabytes, "amount of data in gigabytes to allocate for the session")
	cmd.Flags().Int64Var(&hours, "hours", hours, "duration in hours to allocate for the session")
	cmd.Flags().StringVar(&maxPriceStr, "max-price", maxPriceStr, "maximum price per gigabyte or per hour for the session")
	cmd.Flags().Uint64Var(&planID, "plan-id", planID, "plan identifier to start the session from")
	cmd.Flags().StringVar(&renewalPricePolicyStr, "renewal-price-policy", renewalPricePolicyStr, "price policy to apply when renewing the plan")
	cmd.Flags().Uint64Var(&subscriptionID, "subscription-id", subscriptionID, "subscription identifier to start the session from")

	return cmd
}
