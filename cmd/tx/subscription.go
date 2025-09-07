package tx

import (
	"fmt"
	"strconv"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/sentinel-official/sentinel-go-sdk/config"
	"github.com/sentinel-official/sentinelhub/v12/types/v1"
	"github.com/sentinel-official/sentinelhub/v12/x/subscription/types/v3"
	"github.com/spf13/cobra"
)

func txSubscriptionCancelCmd(cfg *config.Config) *cobra.Command {
	h := &Handler{
		Command: cobra.Command{
			Use:   "subscription-cancel [id]",
			Short: "Cancel a subscription",
			Args:  cobra.ExactArgs(1),
		},
		RunE: func(cmd *cobra.Command, args []string, fromAddr types.AccAddress) (types.Msg, error) {
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parsing id %q: %w", args[0], err)
			}

			return v3.NewMsgCancelSubscriptionRequest(
				fromAddr.Bytes(),
				id,
			), nil
		},
	}

	cmd := NewCommand(cfg, h)

	return cmd
}

func txSubscriptionRenewCmd(cfg *config.Config) *cobra.Command {
	var denom string

	h := &Handler{
		Command: cobra.Command{
			Use:   "subscription-renew [id]",
			Short: "Renew a subscription",
			Args:  cobra.ExactArgs(1),
		},
		RunE: func(cmd *cobra.Command, args []string, fromAddr types.AccAddress) (types.Msg, error) {
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parsing id %q: %w", args[0], err)
			}

			return v3.NewMsgRenewSubscriptionRequest(
				fromAddr.Bytes(),
				id,
				denom,
			), nil
		},
	}

	cmd := NewCommand(cfg, h)
	cmd.Flags().StringVar(&denom, "denom", denom, "denomination of the currency to be used for renewal")

	return cmd
}

func txSubscriptionShareCmd(cfg *config.Config) *cobra.Command {
	h := &Handler{
		Command: cobra.Command{
			Use:   "subscription-share [id] [acc-addr] [bytes]",
			Short: "Share a subscription",
			Args:  cobra.ExactArgs(3),
		},
		RunE: func(cmd *cobra.Command, args []string, fromAddr types.AccAddress) (types.Msg, error) {
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parsing id %q: %w", args[0], err)
			}

			accAddr, err := types.AccAddressFromBech32(args[1])
			if err != nil {
				return nil, fmt.Errorf("parsing account addr %q: %w", args[1], err)
			}

			bytes, ok := math.NewIntFromString(args[2])
			if !ok {
				return nil, fmt.Errorf("invalid bytes %q", args[2])
			}

			return v3.NewMsgShareSubscriptionRequest(
				fromAddr.Bytes(),
				id,
				accAddr.Bytes(),
				bytes,
			), nil
		},
	}

	cmd := NewCommand(cfg, h)

	return cmd
}

func txSubscriptionStartCmd(cfg *config.Config) *cobra.Command {
	var (
		denom                 string
		renewalPricePolicyStr string
	)

	h := &Handler{
		Command: cobra.Command{
			Use:   "subscription-start [id]",
			Short: "Start a subscription",
			Args:  cobra.ExactArgs(1),
		},
		RunE: func(cmd *cobra.Command, args []string, fromAddr types.AccAddress) (types.Msg, error) {
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parsing id %q: %w", args[0], err)
			}

			return v3.NewMsgStartSubscriptionRequest(
				fromAddr.Bytes(),
				id,
				denom,
				v1.RenewalPricePolicyFromString(renewalPricePolicyStr),
			), nil
		},
	}

	cmd := NewCommand(cfg, h)
	cmd.Flags().StringVar(&denom, "denom", denom, "denomination of the currency to be used for the payment")
	cmd.Flags().StringVar(&renewalPricePolicyStr, "renewal-price-policy", renewalPricePolicyStr, "price policy to apply when renewing the subscription")

	return cmd
}

func txSubscriptionUpdateCmd(cfg *config.Config) *cobra.Command {
	var renewalPricePolicyStr string

	h := &Handler{
		Command: cobra.Command{
			Use:   "subscription-update [id]",
			Short: "Update a subscription",
			Args:  cobra.ExactArgs(1),
		},
		RunE: func(cmd *cobra.Command, args []string, fromAddr types.AccAddress) (types.Msg, error) {
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parsing id %q: %w", args[0], err)
			}

			return v3.NewMsgUpdateSubscriptionRequest(
				fromAddr.Bytes(),
				id,
				v1.RenewalPricePolicyFromString(renewalPricePolicyStr),
			), nil
		},
	}

	cmd := NewCommand(cfg, h)
	cmd.Flags().StringVar(&renewalPricePolicyStr, "renewal-price-policy", renewalPricePolicyStr, "price policy to apply when renewing the subscription")

	return cmd
}
