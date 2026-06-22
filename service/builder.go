package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sentinel-official/sentinel-go-sdk/amneziawg"
	"github.com/sentinel-official/sentinel-go-sdk/hysteria2"
	"github.com/sentinel-official/sentinel-go-sdk/node"
	"github.com/sentinel-official/sentinel-go-sdk/openvpn"
	"github.com/sentinel-official/sentinel-go-sdk/types"
	"github.com/sentinel-official/sentinel-go-sdk/v2ray"
	"github.com/sentinel-official/sentinel-go-sdk/wireguard"
	"github.com/sentinel-official/sentinel-go-sdk/xray"
)

// Builder holds all state required to initialize a client service.
type Builder struct {
	Client       *node.Client
	HomeDir      string
	ID           uint64
	Type         types.ServiceType
	V2RayCfg     *v2ray.ClientConfig
	WireGuardCfg *wireguard.ClientConfig
	XrayCfg      *xray.ClientConfig
	AmneziaWGCfg *amneziawg.ClientConfig
	Hysteria2Cfg *hysteria2.ClientConfig
	OpenVPNCfg   *openvpn.ClientConfig
}

// Build creates and initializes a ClientService by performing the handshake,
// applying configuration, and preparing it for use.
func (b *Builder) Build(ctx context.Context) (types.ClientService, error) {
	//nolint:exhaustive
	switch b.Type {
	case types.ServiceTypeV2Ray:
		return b.buildV2Ray(ctx)

	case types.ServiceTypeWireGuard:
		return b.buildWireGuard(ctx)

	case types.ServiceTypeXray:
		return b.buildXray(ctx)

	case types.ServiceTypeOpenVPN:
		return nil, fmt.Errorf("unspported service type %q", b.Type)

	case types.ServiceTypeUnspecified:
		return nil, fmt.Errorf("unspecified service type %q", b.Type)

	default:
		return nil, fmt.Errorf("unknown service type %q", b.Type)
	}
}

// buildV2Ray performs the V2Ray handshake and returns an initialized client service.
func (b *Builder) buildV2Ray(ctx context.Context) (types.ClientService, error) {
	// Create a handshake request with the V2Ray UUID.
	uuid := b.V2RayCfg.GetID()
	addReq := &v2ray.PeerRequest{UUID: uuid}

	// Perform the handshake with the node.
	resp, err := b.Client.InitHandshake(ctx, b.ID, addReq)
	if err != nil {
		return nil, fmt.Errorf("performing node handshake: %w", err)
	}

	// Decode the handshake response.
	var addResp v2ray.AddPeerResponse
	if err := json.Unmarshal(resp.Data, &addResp); err != nil {
		return nil, fmt.Errorf("unmarshaling add peer response: %w", err)
	}

	// Populate outbound configs from metadata.
	for _, addr := range resp.Addrs {
		for _, md := range addResp.Metadata {
			port := md.GetPort()
			if port == nil {
				continue
			}

			for p := port.OutFrom; p <= port.OutTo; p++ {
				b.V2RayCfg.Outbounds = append(
					b.V2RayCfg.Outbounds,
					&v2ray.OutboundClientConfig{
						Addr:              addr,
						Port:              p,
						ProxyProtocol:     md.ProxyProtocol.String(),
						TransportProtocol: md.TransportProtocol.String(),
						TransportSecurity: md.TransportSecurity.String(),
					},
				)
			}
		}
	}

	// Create V2Ray client and run PreUp.
	service := v2ray.NewClient("v2ray", b.HomeDir, b.V2RayCfg)
	if err := service.Init(true); err != nil {
		return nil, fmt.Errorf("running service init task: %w", err)
	}

	return service, nil
}

// buildWireGuard performs the WireGuard handshake and returns an initialized client service.
func (b *Builder) buildWireGuard(ctx context.Context) (types.ClientService, error) {
	// Create handshake request with public key.
	pk := b.WireGuardCfg.GetPrivateKey()
	addReq := &wireguard.PeerRequest{PublicKey: pk.Public()}

	// Perform the handshake with the node.
	resp, err := b.Client.InitHandshake(ctx, b.ID, addReq)
	if err != nil {
		return nil, fmt.Errorf("performing node handshake: %w", err)
	}

	// Decode handshake response.
	var addResp wireguard.AddPeerResponse
	if err := json.Unmarshal(resp.Data, &addResp); err != nil {
		return nil, fmt.Errorf("unmarshaling add peer response: %w", err)
	}

	// Apply handshake data to WireGuard config.
	b.WireGuardCfg.Addrs = addResp.GetAddrs()
	b.WireGuardCfg.Peer.Addr = resp.Addrs[0]
	b.WireGuardCfg.Peer.Port = addResp.Metadata[0].Port
	b.WireGuardCfg.Peer.PublicKey = addResp.Metadata[0].PublicKey.String()

	// Create WireGuard client and run PreUp.
	service := wireguard.NewClient("wireguard", b.HomeDir, b.WireGuardCfg)
	if err := service.Init(true); err != nil {
		return nil, fmt.Errorf("running service init task: %w", err)
	}

	return service, nil
}

// buildXray performs the Xray handshake and returns an initialized client service.
func (b *Builder) buildXray(ctx context.Context) (types.ClientService, error) {
	// Create a handshake request with the Xray UUID.
	uuid := b.XrayCfg.GetID()
	addReq := &xray.PeerRequest{UUID: uuid}

	// Perform the handshake with the node.
	resp, err := b.Client.InitHandshake(ctx, b.ID, addReq)
	if err != nil {
		return nil, fmt.Errorf("performing node handshake: %w", err)
	}

	// Decode the handshake response.
	var addResp xray.AddPeerResponse
	if err := json.Unmarshal(resp.Data, &addResp); err != nil {
		return nil, fmt.Errorf("unmarshaling add peer response: %w", err)
	}

	// Populate outbound configs from metadata.
	for _, addr := range resp.Addrs {
		for _, md := range addResp.Metadata {
			port := md.GetPort()
			if port == nil {
				continue
			}

			for p := port.OutFrom; p <= port.OutTo; p++ {
				b.XrayCfg.Outbounds = append(
					b.XrayCfg.Outbounds,
					&xray.OutboundClientConfig{
						Addr:               addr,
						Port:               p,
						ProxyProtocol:      md.ProxyProtocol.String(),
						TransportProtocol:  md.TransportProtocol.String(),
						TransportSecurity:  md.TransportSecurity.String(),
						Flow:               md.Flow.String(),
						TLSPin:             md.TLSPin,
						Method:             md.Method,
						ServerKey:          md.Key,
						RealityServerName:  md.RealityServerName,
						RealityShortId:     md.RealityShortId,
						RealityPublicKey:   md.RealityPublicKey,
						RealityFingerprint: md.RealityFingerprint,
					},
				)
			}
		}
	}

	// Create Xray client and run PreUp.
	service := xray.NewClient("xray", b.HomeDir, b.XrayCfg)
	if err := service.Init(true); err != nil {
		return nil, fmt.Errorf("running service init task: %w", err)
	}

	return service, nil
}
