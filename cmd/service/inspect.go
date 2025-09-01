package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/sentinel-official/sentinel-go-sdk/app"
	"github.com/sentinel-official/sentinel-go-sdk/config"
	"github.com/sentinel-official/sentinel-go-sdk/libs/geoip"
	"github.com/sentinel-official/sentinel-go-sdk/libs/log"
	"github.com/sentinel-official/sentinel-go-sdk/node"
	sentinelsdk "github.com/sentinel-official/sentinel-go-sdk/types"
	"github.com/sentinel-official/sentinel-go-sdk/utils"
	"github.com/sentinel-official/sentinel-go-sdk/v2ray"
	"github.com/sentinel-official/sentinel-go-sdk/wireguard"
	"github.com/sentinel-official/sentinelhub/v12/types"
	"github.com/sentinel-official/sentinelhub/v12/types/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

// writeLocation retrieves the GeoIP location and writes it to the provided writer in JSON format.
func writeLocation(ctx context.Context, w io.Writer) error {
	// Create a new GeoIP client
	client := geoip.NewDefaultClient()

	// Lookup the node's location
	location, err := client.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("getting GeoIP location: %w", err)
	}

	// Write the location to stdout in JSON format
	if err := utils.Writeln(w, location, "json"); err != nil {
		return fmt.Errorf("writing GeoIP location: %w", err)
	}

	return nil
}

// NewInspectCmd returns a new command for inspecting a Sentinel node's status,
// connectivity, and location information.
func NewInspectCmd(cfg *config.Config) *cobra.Command {
	// Default v2ray and wireguard client configurations
	v2rayCfg := v2ray.DefaultClientConfig()
	wireguardCfg := wireguard.DefaultClientConfig()

	// Define variables for other flags
	var maxPriceStr string
	var timeout = 30 * time.Second

	// Define the inspect command
	cmd := &cobra.Command{
		Use:   "inspect [node-addr]",
		Args:  cobra.ExactArgs(1),
		Short: "Inspect the status, connectivity, and location info of a node",
		Long: `This command evaluates a Sentinel node by checking its status, verifying connectivity,
and retrieving location details. It ensures the node is active, assesses its availability, and
establishes a temporary session to gather network-related information. Additionally, it fetches
geolocation data using a proxy (if applicable) and then gracefully terminates the session. This
helps users determine whether a node meets their requirements before initiating a full connection.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancelTimeout := context.WithTimeout(cmd.Context(), timeout)
			defer cancelTimeout()

			homeDir := viper.GetString("home")

			// Parse the node address from the command argument
			addr, err := types.NodeAddressFromBech32(args[0])
			if err != nil {
				return fmt.Errorf("parsing node addr %q: %w", args[0], err)
			}

			// Create a new node client from the configuration
			client, err := node.NewClientFromConfig(cfg)
			if err != nil {
				return fmt.Errorf("creating client from config: %w", err)
			}

			// Query the node and check its status
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

			// Parse the maximum price if node has gigabyte pricing
			maxPrice := v1.ZeroPrice("")
			if n.GetGigabytePrices().Len() > 0 {
				maxPrice, err = v1.NewPriceFromString(maxPriceStr)
				if err != nil {
					return fmt.Errorf("parsing max price %q: %w", maxPriceStr, err)
				}
			}

			// Get the node's price for the specified denomination
			price, found := n.GigabytePrice(maxPrice.Denom)
			if !found {
				return fmt.Errorf("price for denom %q does not exist", maxPrice.Denom)
			}

			// Check if the node's price exceeds the maximum allowed price
			if price.IsGT(maxPrice) {
				return fmt.Errorf("price %q is greater than max price %q", price, maxPrice)
			}

			client.WithAddr(addr)
			client.WithInsecure(true)

			// Get information about the node
			info, err := client.GetInfo(ctx)
			if err != nil {
				return fmt.Errorf("getting node %q info: %w", addr.String(), err)
			}
			if info.GetServiceType() == sentinelsdk.ServiceTypeUnspecified {
				return fmt.Errorf("unspecified service type for node %q", addr.String())
			}

			// Start a new session with the node
			id, err := client.NodeStartSession(ctx, addr, 1, 0, maxPrice)
			if err != nil {
				return fmt.Errorf("starting session for node %q: %w", addr.String(), err)
			}

			builder := &Builder{
				Client:       client,
				HomeDir:      homeDir,
				ID:           id,
				Type:         info.GetServiceType(),
				V2RayCfg:     v2rayCfg,
				WireGuardCfg: wireguardCfg,
			}

			service, err := builder.Build(ctx)
			if err != nil {
				return fmt.Errorf("building service %q: %w", info.GetServiceType(), err)
			}

			jobCtx, jobCancel := context.WithCancel(context.Background())
			defer jobCancel()

			jobGroup, jobCtx := errgroup.WithContext(jobCtx)

			inspectionDone := make(chan struct{})

			startFunc := func() error {
				sg := &errgroup.Group{}

				sg.Go(func() error {
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

					jobGroup.Go(func() error {
						if err := service.Wait(); err != nil {
							return fmt.Errorf("waiting service: %w", err)
						}

						return nil
					})

					return nil
				})

				jobGroup.Go(func() error {
					defer close(inspectionDone)

					ticker := time.NewTicker(1 * time.Second)
					defer ticker.Stop()

					for {
						select {
						case <-jobCtx.Done():
							return jobCtx.Err()
						case <-ticker.C:
							up, err := service.IsUp()
							if err != nil {
								return fmt.Errorf("checking service status: %w", err)
							}
							if !up {
								continue
							}

							// Set the proxy address to use for geolocation lookup
							if service.Type() == sentinelsdk.ServiceTypeV2Ray {
								proxyAddr := fmt.Sprintf("socks5://127.0.0.1:%d", v2rayCfg.Proxy.Port)
								m := map[string]string{
									"HTTP_PROXY":  proxyAddr,
									"HTTPS_PROXY": proxyAddr,
								}

								for key, value := range m {
									if err := os.Setenv(key, value); err != nil {
										return fmt.Errorf("setting environment variable %q: %w", key, err)
									}
								}
							}

							// Write GeoIP location to CLI Stdout
							if err := writeLocation(jobCtx, cmd.OutOrStdout()); err != nil {
								return err
							}

							return nil
						}
					}
				})

				if err := sg.Wait(); err != nil {
					return err
				}

				return nil
			}

			stopFunc := func() error {
				sg := &errgroup.Group{}

				sg.Go(func() error {
					jobCancel()
					return nil
				})

				sg.Go(func() error {
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

				if err := sg.Wait(); err != nil {
					return err
				}

				return nil
			}

			waitFunc := func() error {
				if err := jobGroup.Wait(); err != nil {
					log.Debug("jobGroup.Wait()",
						"err:", err,
						"ctx.Err():", jobCtx.Err(),
						"context.Cause(ctx):", context.Cause(jobCtx),
					)

					return err
				}

				return nil
			}

			eg, egCtx := errgroup.WithContext(ctx)
			running := atomic.Bool{}

			eg.Go(func() error {
				log.Info("Starting client")
				if err := startFunc(); err != nil {
					return fmt.Errorf("starting client: %w", err)
				}

				running.Store(true)
				log.Info("Client started successfully")

				eg.Go(func() error {
					if err := waitFunc(); err != nil {
						return fmt.Errorf("waiting client: %w", err)
					}

					return nil
				})

				return nil
			})

			eg.Go(func() error {
				select {
				case <-egCtx.Done():
					if !running.Load() {
						return egCtx.Err()
					}
				case <-inspectionDone:
				}

				log.Info("Stopping client")
				if err := stopFunc(); err != nil {
					return app.NewShutdownError(fmt.Errorf("stopping client: %w", err))
				}

				running.Store(false)
				log.Info("Client stopped successfully")

				return nil
			})

			if err := eg.Wait(); err != nil {
				log.Debug("eg.Wait()",
					"err:", err,
					"ctx.Err():", egCtx.Err(),
					"context.Cause(ctx):", context.Cause(egCtx),
				)

				return err
			}

			return nil
		},
	}

	// Set flags for the command
	cfg.SetForFlags(cmd.Flags())
	v2rayCfg.SetForFlags(cmd.Flags(), "v2ray")
	wireguardCfg.SetForFlags(cmd.Flags(), "wireguard")

	cmd.Flags().StringVar(&maxPriceStr, "max-price", maxPriceStr, "maximum price per gigabyte for the session")
	cmd.Flags().DurationVar(&timeout, "timeout", timeout, "maximum duration to wait for inspection before timing out")

	return cmd
}
