package tx

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/sentinel-official/sentinel-go-sdk/config"
	"github.com/sentinel-official/sentinel-go-sdk/core"
	"github.com/sentinel-official/sentinel-go-sdk/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Handler defines all components of a transaction command.
type Handler struct {
	cobra.Command

	RunE func(cmd *cobra.Command, args []string, fromAddr types.AccAddress) (types.Msg, error)
}

// NewCommand builds a Cobra command from a Handler.
func NewCommand(cfg *config.Config, h *Handler) *cobra.Command {
	cmd := &cobra.Command{
		Use:   h.Use,
		Short: h.Short,
		Long:  h.Long,
		Args:  h.Args,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := core.NewClientFromConfig(cfg)
			if err != nil {
				return fmt.Errorf("creating client from config: %w", err)
			}

			fromAddr, err := c.MsgFromAddr()
			if err != nil {
				return fmt.Errorf("getting msg from addr: %w", err)
			}

			msg, err := h.RunE(cmd, args, fromAddr.Bytes())
			if err != nil {
				return fmt.Errorf("running handler: %w", err)
			}

			resp, res, err := c.BroadcastTxCommit(cmd.Context(), msg)
			if err != nil {
				return fmt.Errorf("broadcasting tx commit: %w", err)
			}

			output := PrepareResponse(resp, res)

			format := viper.GetString("output-format")
			if err := utils.Write(cmd.OutOrStdout(), output, format); err != nil {
				return fmt.Errorf("writing output: %w", err)
			}

			return nil
		},
	}

	return cmd
}

func PrepareResponse(resp, res interface{}) map[string]interface{} {
	return map[string]interface{}{
		"response": resp,
		"result":   res,
	}
}
