package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/sentinel-official/sentinel-go-sdk/amneziawg"
	"github.com/sentinel-official/sentinel-go-sdk/app"
	"github.com/sentinel-official/sentinel-go-sdk/core/config"
	"github.com/sentinel-official/sentinel-go-sdk/hysteria2"
	"github.com/sentinel-official/sentinel-go-sdk/libs/log"
	"github.com/sentinel-official/sentinel-go-sdk/node"
	"github.com/sentinel-official/sentinel-go-sdk/openvpn"
	"github.com/sentinel-official/sentinel-go-sdk/process"
	"github.com/sentinel-official/sentinel-go-sdk/v2ray"
	"github.com/sentinel-official/sentinel-go-sdk/wireguard"
	"github.com/sentinel-official/sentinel-go-sdk/xray"
	"github.com/sentinel-official/sentinelhub/v12/types"
	"github.com/sentinel-official/sentinelhub/v12/types/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

func NewConnectCmd(cfg *config.Config) *cobra.Command {
	// Default client configurations for each supported service type.
	wireguardCfg := wireguard.DefaultClientConfig()
	v2rayCfg := v2ray.DefaultClientConfig()
	openvpnCfg := openvpn.DefaultClientConfig()
	xrayCfg := xray.DefaultClientConfig()
	amneziawgCfg := amneziawg.DefaultClientConfig()
	hysteria2Cfg := hysteria2.DefaultClientConfig()

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
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			homeDir := viper.GetString("home")

			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("parsing id %q: %w", args[0], err)
			}

			client, err := node.NewClientFromConfig(cfg)
			if err != nil {
				return fmt.Errorf("creating client from config: %w", err)
			}

			session, err := client.Session(ctx, id)
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

			n, err := client.Node(ctx, addr)
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

			info, err := client.GetInfo(ctx)
			if err != nil {
				return fmt.Errorf("fetching node %q info: %w", addr.String(), err)
			}

			builder := &Builder{
				Client:       client,
				HomeDir:      homeDir,
				ID:           id,
				Type:         info.GetServiceType(),
				WireGuardCfg: wireguardCfg,
				V2RayCfg:     v2rayCfg,
				OpenVPNCfg:   openvpnCfg,
				XrayCfg:      xrayCfg,
				AmneziaWGCfg: amneziawgCfg,
				Hysteria2Cfg: hysteria2Cfg,
			}

			service, err := builder.Build(ctx)
			if err != nil {
				return fmt.Errorf("building service %q: %w", info.GetServiceType(), err)
			}

			manager := process.NewManager("client")

			setupFunc := func(ctx context.Context) error {
				return manager.Setup(ctx, func() error {
					log.Info("Setting up service")

					if err := service.Setup(ctx); err != nil {
						return fmt.Errorf("setting up service: %w", err)
					}

					return nil
				})
			}

			startFunc := func(ctx context.Context) (context.Context, error) {
				return manager.Start(ctx, func(ctx context.Context) error {
					log.Info("Starting service")

					serviceCtx, err := service.Start(ctx)
					if err != nil {
						return fmt.Errorf("starting service: %w", err)
					}

					manager.Go(ctx, func() error {
						if err := service.Wait(serviceCtx); err != nil {
							return fmt.Errorf("waiting service: %w", err)
						}

						return nil
					})

					return nil
				})
			}

			waitFunc := func(ctx context.Context) error {
				return manager.Wait(ctx, nil)
			}

			stopFunc := func() error {
				return manager.Stop(func() error {
					log.Info("Stopping service")

					if err := service.Stop(); err != nil {
						return fmt.Errorf("stopping service: %w", err)
					}

					return nil
				})
			}

			if err := setupFunc(ctx); err != nil {
				return fmt.Errorf("setting up client: %w", err)
			}

			// Create an errgroup with the signal-aware context.
			eg, ctx := errgroup.WithContext(ctx)

			eg.Go(func() error {
				ctx, err := startFunc(ctx)
				if err != nil {
					return fmt.Errorf("starting client: %w", err)
				}

				log.Info("Client started successfully")

				if err := waitFunc(ctx); err != nil {
					return fmt.Errorf("waiting client: %w", err)
				}

				return nil
			})

			eg.Go(func() error {
				<-ctx.Done()

				if err := stopFunc(); err != nil {
					return app.NewErrStop(fmt.Errorf("stopping client: %w", err))
				}

				log.Info("Client stopped successfully")

				return nil
			})

			if err := eg.Wait(); err != nil {
				return fmt.Errorf("waiting: %w", err)
			}

			return nil
		},
	}

	cfg.SetForFlags(cmd.Flags())
	wireguardCfg.SetForFlags(cmd.Flags(), "wireguard")
	v2rayCfg.SetForFlags(cmd.Flags(), "v2ray")
	openvpnCfg.SetForFlags(cmd.Flags(), "openvpn")
	xrayCfg.SetForFlags(cmd.Flags(), "xray")
	amneziawgCfg.SetForFlags(cmd.Flags(), "amneziawg")
	hysteria2Cfg.SetForFlags(cmd.Flags(), "hysteria2")

	return cmd
}
