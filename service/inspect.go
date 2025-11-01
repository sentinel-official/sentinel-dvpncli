package service

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/sentinel-official/sentinel-go-sdk/app"
	"github.com/sentinel-official/sentinel-go-sdk/core/config"
	"github.com/sentinel-official/sentinel-go-sdk/libs/geoip"
	"github.com/sentinel-official/sentinel-go-sdk/libs/log"
	"github.com/sentinel-official/sentinel-go-sdk/node"
	"github.com/sentinel-official/sentinel-go-sdk/process"
	sentinelsdk "github.com/sentinel-official/sentinel-go-sdk/types"
	"github.com/sentinel-official/sentinel-go-sdk/utils"
	"github.com/sentinel-official/sentinel-go-sdk/v2ray"
	"github.com/sentinel-official/sentinel-go-sdk/wireguard"
	"github.com/sentinel-official/sentinelhub/v12/types"
	"github.com/sentinel-official/sentinelhub/v12/types/v1"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// NewInspectCmd returns a new command for inspecting a Sentinel node's status,
// connectivity, and location information.
func NewInspectCmd(cfg *config.Config) *cobra.Command { //nolint:gocyclo,maintidx
	// Default v2ray and wireguard client configurations
	v2rayCfg := v2ray.DefaultClientConfig()
	wireguardCfg := wireguard.DefaultClientConfig()

	// Define variables for other flags
	var (
		maxPriceStr string
		timeout     = 30 * time.Second
	)

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

			// Create a temporary directory to store configuration
			tempDir, err := os.MkdirTemp("", "inspect-")
			if err != nil {
				return fmt.Errorf("creating temporary directory: %w", err)
			}

			// Ensure the temporary directory is removed when done
			defer func() {
				_ = os.RemoveAll(tempDir)
			}()

			builder := &Builder{
				Client:       client,
				HomeDir:      tempDir,
				ID:           id,
				Type:         info.GetServiceType(),
				V2RayCfg:     v2rayCfg,
				WireGuardCfg: wireguardCfg,
			}

			service, err := builder.Build(ctx)
			if err != nil {
				return fmt.Errorf("building service %q: %w", info.GetServiceType(), err)
			}

			inspectionDone := make(chan struct{})
			manager := process.NewManager("manager")

			setupFunc := func(ctx context.Context) error {
				return manager.Setup(ctx, func() error {
					log.Info("Setting up service")

					if err := service.Setup(ctx); err != nil {
						return fmt.Errorf("setting up service: %w", err)
					}

					return nil
				})
			}

			startFunc := func(parent context.Context) (context.Context, error) {
				return manager.Start(parent, func(ctx context.Context) error {
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

					manager.Go(ctx, func() error {
						defer close(inspectionDone)

						ticker := time.NewTicker(1 * time.Second)
						defer ticker.Stop()

						for {
							select {
							case <-ctx.Done():
								return ctx.Err()
							case <-ticker.C:
								ok, err := service.IsRunning()
								if err != nil {
									return fmt.Errorf("checking service status: %w", err)
								}

								if !ok {
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

								// Create a new GeoIP client
								client := geoip.NewDefaultClient()

								retryFunc := func() error {
									// Lookup the node's location
									location, err := client.Get(ctx, "")
									if err != nil {
										return fmt.Errorf("getting GeoIP location: %w", err)
									}

									// Write the location to stdout in JSON format
									if err := utils.Writeln(cmd.OutOrStdout(), location, "json"); err != nil {
										return fmt.Errorf("writing GeoIP location: %w", err)
									}

									return nil
								}

								if err := retry.Do(
									retryFunc,
									retry.Context(ctx),
									retry.Attempts(5),
									retry.Delay(1*time.Second),
									retry.DelayType(retry.FixedDelay),
								); err != nil {
									return fmt.Errorf("getting GeoIP location failed after multiple retries: %w", err)
								}

								return nil
							}
						}
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
				return fmt.Errorf("setting up: %w", err)
			}

			eg, ctx := errgroup.WithContext(ctx)

			eg.Go(func() error {
				ctx, err := startFunc(ctx)
				if err != nil {
					return fmt.Errorf("starting: %w", err)
				}

				log.Info("Inspection started successfully")

				if err := waitFunc(ctx); err != nil {
					return fmt.Errorf("waiting: %w", err)
				}

				return nil
			})

			eg.Go(func() error {
				select {
				case <-ctx.Done():
				case <-inspectionDone:
				}

				if err := stopFunc(); err != nil {
					return app.NewErrShutdown(fmt.Errorf("stopping: %w", err))
				}

				log.Info("Inspection stopped successfully")

				return nil
			})

			if err := eg.Wait(); err != nil {
				return fmt.Errorf("waiting group: %w", err)
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
