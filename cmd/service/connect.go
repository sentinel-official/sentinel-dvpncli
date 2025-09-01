package service

import (
	"fmt"
	"strconv"

	"github.com/sentinel-official/sentinel-go-sdk/config"
	"github.com/sentinel-official/sentinel-go-sdk/libs/log"
	"github.com/sentinel-official/sentinel-go-sdk/node"
	sentinelsdk "github.com/sentinel-official/sentinel-go-sdk/types"
	"github.com/sentinel-official/sentinel-go-sdk/v2ray"
	"github.com/sentinel-official/sentinel-go-sdk/wireguard"
	"github.com/sentinel-official/sentinelhub/v12/types"
	"github.com/sentinel-official/sentinelhub/v12/types/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

func NewConnectCmd(cfg *config.Config) *cobra.Command {
	// Default v2ray and wireguard client configurations
	v2rayCfg := v2ray.DefaultClientConfig()
	wireguardCfg := wireguard.DefaultClientConfig()

	cmd := &cobra.Command{
		Use:   "connect [id]",
		Args:  cobra.ExactArgs(1),
		Short: "Connect to a node and start the client",
		Long: `Connect to a Sentinel node using an existing active session and start the client service
(e.g., V2Ray, WireGuard, or OpenVPN). The command validates the session and node status,
fetches node info to determine the service type, builds the appropriate client, and brings it
up. It listens for SIGINT/SIGTERM and gracefully shuts the client down by running pre-down,
down, and post-down tasks.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir := viper.GetString("home")

			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("parsing id %q: %w", args[0], err)
			}

			client, err := node.NewClientFromConfig(cfg)
			if err != nil {
				return fmt.Errorf("creating client from config: %w", err)
			}

			session, err := client.Session(cmd.Context(), id)
			if err != nil {
				return fmt.Errorf("querying session %d: %w", id, err)
			}
			if session == nil {
				return fmt.Errorf("session %d does not exist", id)
			}
			if !session.GetStatus().Equal(v1.StatusActive) {
				return fmt.Errorf("invalid session status %q, expected %q", session.GetStatus(), v1.StatusActive)
			}

			addr, err := types.NodeAddressFromBech32(session.GetNodeAddress())
			if err != nil {
				return fmt.Errorf("parsing Bech32 node addr %q: %w", session.GetNodeAddress(), err)
			}

			n, err := client.Node(cmd.Context(), addr)
			if err != nil {
				return fmt.Errorf("querying node %q: %w", addr.String(), err)
			}
			if n == nil {
				return fmt.Errorf("node %q does not exist", addr.String())
			}
			if !n.Status.Equal(v1.StatusActive) {
				return fmt.Errorf("invalid node status %q; expected %q", n.Status, v1.StatusActive)
			}

			client.WithAddr(addr)
			client.WithInsecure(true)

			info, err := client.GetInfo(cmd.Context())
			if err != nil {
				return fmt.Errorf("fetching node %q info: %w", addr.String(), err)
			}
			if info.GetServiceType() == sentinelsdk.ServiceTypeUnspecified {
				return fmt.Errorf("unspecified service type for node %q", addr.String())
			}

			builder := &Builder{
				Client:       client,
				HomeDir:      homeDir,
				ID:           id,
				Type:         info.GetServiceType(),
				V2RayCfg:     v2rayCfg,
				WireGuardCfg: wireguardCfg,
			}

			service, err := builder.Build(cmd.Context())
			if err != nil {
				return fmt.Errorf("building service %q: %w", info.GetServiceType(), err)
			}

			// Create an errgroup with the signal-aware context.
			eg, ctx := errgroup.WithContext(cmd.Context())

			eg.Go(func() error {
				<-ctx.Done()

				log.Info("Running service pre-down task")
				if err := service.PreDown(); err != nil {
					return fmt.Errorf("running service pre-down task: %w", err)
				}

				log.Info("Running service down task")
				if err := service.Down(); err != nil {
					return fmt.Errorf("running service down task: %w", err)
				}

				log.Info("Running service post-down task")
				if err := service.PostDown(); err != nil {
					return fmt.Errorf("running service post-down task: %w", err)
				}

				return nil
			})

			eg.Go(func() error {
				log.Info("Running service pre-up task")
				if err := service.PreUp(); err != nil {
					return fmt.Errorf("running service pre-up task: %w", err)
				}

				log.Info("Running service up task")
				if err := service.Up(); err != nil {
					return fmt.Errorf("running service up task: %w", err)
				}

				log.Info("Running service post-up task")
				if err := service.PostUp(); err != nil {
					return fmt.Errorf("running service post-up task: %w", err)
				}

				log.Info("Client started successfully")
				eg.Go(func() error {
					if err := service.Wait(); err != nil {
						return fmt.Errorf("waiting service: %w", err)
					}

					return nil
				})

				return nil
			})

			if err := eg.Wait(); err != nil {
				return err
			}

			log.Info("Client stopped successfully")
			return nil
		},
	}

	cfg.SetForFlags(cmd.Flags())
	v2rayCfg.SetForFlags(cmd.Flags(), "v2ray")
	wireguardCfg.SetForFlags(cmd.Flags(), "wireguard")

	return cmd
}
