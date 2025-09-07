package query

import (
	"fmt"

	"github.com/sentinel-official/sentinel-go-sdk/core"
	"github.com/sentinel-official/sentinel-go-sdk/core/config"
	"github.com/sentinel-official/sentinel-go-sdk/libs/safe"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func queryParamsCmd(cfg *config.Config) *cobra.Command {
	h := &Handler{
		Command: cobra.Command{
			Use:   "params",
			Short: "Query all parameters",
		},
		RunE: func(cmd *cobra.Command, args []string, c *core.Client) (interface{}, error) {
			m := safe.NewMap[string, interface{}]()

			eg, ctx := errgroup.WithContext(cmd.Context())
			eg.Go(func() error {
				res, err := c.LeaseParams(ctx)
				if err != nil {
					return fmt.Errorf("querying lease params: %w", err)
				}

				m.Set("lease", res)

				return nil
			})

			eg.Go(func() error {
				res, err := c.NodeParams(ctx)
				if err != nil {
					return fmt.Errorf("querying node params: %w", err)
				}

				m.Set("node", res)

				return nil
			})

			eg.Go(func() error {
				res, err := c.ProviderParams(ctx)
				if err != nil {
					return fmt.Errorf("querying provider params: %w", err)
				}

				m.Set("provider", res)

				return nil
			})

			eg.Go(func() error {
				res, err := c.SessionParams(ctx)
				if err != nil {
					return fmt.Errorf("querying session params: %w", err)
				}

				m.Set("session", res)

				return nil
			})

			eg.Go(func() error {
				res, err := c.SubscriptionParams(ctx)
				if err != nil {
					return fmt.Errorf("querying subscription params: %w", err)
				}

				m.Set("subscription", res)

				return nil
			})

			if err := eg.Wait(); err != nil {
				return nil, fmt.Errorf("querying params: %w", err)
			}

			items := make(map[string]interface{})
			m.RangeGet(func(k string, v interface{}) bool {
				items[k] = v

				return false
			})

			return items, nil
		},
	}

	cmd := NewCommand(cfg, h)

	return cmd
}
