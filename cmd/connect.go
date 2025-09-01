package cmd

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/go-kit/kit/transport/http/jsonrpc"
	"github.com/hashicorp/go-uuid"
	"github.com/pkg/errors"
	hubtypes "github.com/sentinel-official/hub/types"
	nodetypes "github.com/sentinel-official/hub/x/node/types"
	sessiontypes "github.com/sentinel-official/hub/x/session/types"
	"github.com/spf13/cobra"

	"github.com/sentinel-official/cli-client/services/v2ray"
	v2raytypes "github.com/sentinel-official/cli-client/services/v2ray/types"
	"github.com/sentinel-official/cli-client/services/wireguard"
	wireguardtypes "github.com/sentinel-official/cli-client/services/wireguard/types"
	clienttypes "github.com/sentinel-official/cli-client/types"
	netutil "github.com/sentinel-official/cli-client/utils/net"
)

func fetchNodeInfo(remoteURL string, timeout time.Duration) (map[string]interface{}, error) {
	endpoint, err := url.JoinPath(remoteURL, "status")
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: timeout,
	}

	resp, err := httpClient.Get(endpoint)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var body clienttypes.Response
	if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	return body.Result.(map[string]interface{}), nil
}

func queryNode(qsc nodetypes.QueryServiceClient, address hubtypes.NodeAddress) (*nodetypes.Node, error) {
	var (
		result, err = qsc.QueryNode(
			context.Background(),
			nodetypes.NewQueryNodeRequest(address),
		)
	)

	if err != nil {
		return nil, err
	}

	return &result.Node, nil
}

func queryActiveSession(qsc sessiontypes.QueryServiceClient, address sdk.AccAddress) (*sessiontypes.Session, error) {
	var (
		result, err = qsc.QuerySessionsForAccount(
			context.Background(),
			sessiontypes.NewQuerySessionsForAccountRequest(
				address,
				&query.PageRequest{
					Limit:   1,
					Reverse: true,
				},
			),
		)
	)

	if err != nil {
		return nil, err
	}
	if len(result.Sessions) > 0 {
		if result.Sessions[0].Status == hubtypes.StatusInactivePending {
			return nil, nil
		} else {
			return &result.Sessions[0], nil
		}
	}

	return nil, nil
}

func ConnectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect [subscription] [address]",
		Short: "Connect to a node",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			address, err := hubtypes.NodeAddressFromBech32(args[1])
			if err != nil {
				return err
			}

			timeout, err := cmd.Flags().GetDuration(clienttypes.FlagTimeout)
			if err != nil {
				return err
			}

			ss, err := cmd.Flags().GetStringArray(clienttypes.FlagResolver)
			if err != nil {
				return err
			}

			var resolvers []net.IP
			for _, s := range ss {
				ip := net.ParseIP(s)
				if ip == nil {
					return fmt.Errorf("invalid resolver ip %s", s)
				}

				resolvers = append(resolvers, ip)
			}

			v2RayProxyPort, err := cmd.Flags().GetUint16(clienttypes.FlagV2RayProxyPort)
			if err != nil {
				return err
			}

			var (
				status         = clienttypes.NewStatus()
				statusFilePath = filepath.Join(ctx.HomeDir, "status.json")
			)

			if err = status.LoadFromPath(statusFilePath); err != nil {
				return err
			}

			var service clienttypes.Service
			if status.Type == 1 {
				var cfg wireguardtypes.Config
				if err = json.Unmarshal(status.Info, &cfg); err != nil {
					return err
				}

				service = wireguard.NewWireGuard(&cfg)
			} else if status.Type == 2 {
				var cfg v2raytypes.Config
				if err = json.Unmarshal(status.Info, &cfg); err != nil {
					return err
				}

				service = v2ray.NewV2Ray(&cfg)
			}

			if service != nil && service.IsUp() {
				if err = service.PreDown(); err != nil {
					return err
				}
				if err = service.Down(); err != nil {
					return err
				}
				if err = service.PostDown(); err != nil {
					return err
				}
			}

			nodeQueryClient := nodetypes.NewQueryServiceClient(ctx)

			node, err := queryNode(nodeQueryClient, address)
			if err != nil {
				return err
			}

			nodeInfo, err := fetchNodeInfo(node.RemoteURL, timeout)
			if err != nil {
				return err
			}

			var (
				nodeType           = uint64(nodeInfo["type"].(float64))
				messages           []sdk.Msg
				sessionQueryClient = sessiontypes.NewQueryServiceClient(ctx)
			)

			session, err := queryActiveSession(sessionQueryClient, ctx.FromAddress)
			if err != nil {
				return err
			}

			// Add a MsgEndRequest if session is active
			if session != nil {
				messages = append(
					messages,
					sessiontypes.NewMsgEndRequest(
						ctx.FromAddress,
						session.ID,
						0,
					),
				)
			}

			messages = append(
				messages,
				sessiontypes.NewMsgStartRequest(
					ctx.FromAddress,
					id,
					address,
				),
			)

			if err = tx.GenerateOrBroadcastTxCLI(ctx, cmd.Flags(), messages...); err != nil {
				return err
			}

			session, err = queryActiveSession(sessionQueryClient, ctx.FromAddress)
			if err != nil {
				return err
			}

			if session == nil {
				return errors.New("no active session found")
			}

			var (
				key          string
				wgPrivateKey *wireguardtypes.Key
				uid          []byte
			)

			if nodeType == 1 {
				wgPrivateKey, err = wireguardtypes.NewPrivateKey()
				if err != nil {
					return err
				}

				key = wgPrivateKey.Public().String()
			} else if nodeType == 2 {
				uid, err = uuid.GenerateRandomBytes(16)
				if err != nil {
					return err
				}

				key = base64.StdEncoding.EncodeToString(append([]byte{0x01}, uid...))
			} else {
				return fmt.Errorf("invalid node type %d", nodeType)
			}

			signature, _, err := ctx.Keyring.Sign(ctx.From, sdk.Uint64ToBigEndian(session.ID))
			if err != nil {
				return err
			}

			req, err := json.Marshal(
				map[string]interface{}{
					"key":       key,
					"signature": signature,
				},
			)
			if err != nil {
				return err
			}

			endpoint, err := url.JoinPath(node.RemoteURL, fmt.Sprintf("/accounts/%s/sessions/%d", ctx.FromAddress, session.ID))
			if err != nil {
				return err
			}

			var (
				body       clienttypes.Response
				httpClient = http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							InsecureSkipVerify: true,
						},
					},
					Timeout: timeout,
				}
			)

			resp, err := httpClient.Post(endpoint, jsonrpc.ContentType, bytes.NewBuffer(req))
			if err != nil {
				return err
			}

			defer resp.Body.Close()

			if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
				return err
			}
			if body.Error != nil {
				return errors.New(body.Error.Message)
			}

			result, err := base64.StdEncoding.DecodeString(body.Result.(string))
			if err != nil {
				return err
			}

			if nodeType == 1 {
				if len(result) != 58 {
					return fmt.Errorf("incorrect result size %d", len(result))
				}

				var (
					ipv4Address         = net.IP(result[0:4])
					ipv6Address         = net.IP(result[4:20])
					endpointHost        = net.IP(result[20:24])
					endpointPort        = binary.BigEndian.Uint16(result[24:26])
					endpointWGPublicKey = wireguardtypes.NewKey(result[26:58])
				)

				listenPort, err := netutil.GetFreeUDPPort()
				if err != nil {
					return err
				}

				cfg := &wireguardtypes.Config{
					Name: wireguardtypes.DefaultInterface,
					Interface: wireguardtypes.Interface{
						Addresses: []wireguardtypes.IPNet{
							{IP: ipv4Address, Net: 32},
							{IP: ipv6Address, Net: 128},
						},
						ListenPort: listenPort,
						PrivateKey: *wgPrivateKey,
						DNS: append(
							[]net.IP{net.ParseIP("10.8.0.1")},
							resolvers...,
						),
					},
					Peers: []wireguardtypes.Peer{
						{
							PublicKey: *endpointWGPublicKey,
							AllowedIPs: []wireguardtypes.IPNet{
								{IP: net.ParseIP("0.0.0.0")},
								{IP: net.ParseIP("::")},
							},
							Endpoint: wireguardtypes.Endpoint{
								Host: endpointHost.String(),
								Port: endpointPort,
							},
							PersistentKeepalive: 15,
						},
					},
				}

				service = wireguard.NewWireGuard(cfg)
			} else if nodeType == 2 {
				if len(result) != 8 {
					return fmt.Errorf("incorrect result size %d", len(result))
				}

				var (
					vMessAddress  = net.IP(result[0:4])
					vMessPort     = binary.BigEndian.Uint16(result[4:6])
					vMessSecurity = func() string {
						switch result[7] {
						case 0x01:
							return "tls"
						default:
							return ""
						}
					}()
					vMessTransport = func() string {
						switch result[6] {
						case 0x01:
							return "tcp"
						case 0x02:
							return "mkcp"
						case 0x03:
							return "websocket"
						case 0x04:
							return "http"
						case 0x05:
							return "domainsocket"
						case 0x06:
							return "quic"
						case 0x07:
							return "gun"
						case 0x08:
							return "grpc"
						default:
							return ""
						}
					}()
				)

				uidStr, err := uuid.FormatUUID(uid)
				if err != nil {
					return err
				}

				apiPort, err := netutil.GetFreeTCPPort()
				if err != nil {
					return err
				}

				cfg := &v2raytypes.Config{
					API: &v2raytypes.APIConfig{
						Port: apiPort,
					},
					Proxy: &v2raytypes.ProxyConfig{
						Port: v2RayProxyPort,
					},
					VMess: &v2raytypes.VMessConfig{
						Address:   vMessAddress.String(),
						ID:        uidStr,
						Port:      vMessPort,
						Security:  vMessSecurity,
						Transport: vMessTransport,
					},
				}

				service = v2ray.NewV2Ray(cfg)
			} else {
				return fmt.Errorf("invalid node type %d", nodeType)
			}

			if err = service.PreUp(); err != nil {
				return err
			}
			if err = service.Up(); err != nil {
				return err
			}
			if err = service.PostUp(); err != nil {
				return err
			}

			status = clienttypes.NewStatus().
				WithFrom(ctx.GetFromName()).
				WithID(id).
				WithInfo(service.Info()).
				WithTo(address.String()).
				WithType(nodeType)

			if err = status.SaveToPath(statusFilePath); err != nil {
				return err
			}

			return nil
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	cmd.Flags().String(flags.FlagChainID, "sentinelhub-2", "the network chain identity")
	cmd.Flags().StringArray(clienttypes.FlagResolver, []string{"1.0.0.1", "1.1.1.1"}, "provide additional DNS servers")
	cmd.Flags().Duration(clienttypes.FlagTimeout, 15*time.Second, "time limit for requests made by the HTTP client")
	cmd.Flags().Uint16(clienttypes.FlagV2RayProxyPort, 1080, "port number fot the V2Ray SOCKS proxy")

	return cmd
}
