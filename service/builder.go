package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/google/uuid"
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
	WireGuardCfg *wireguard.ClientConfig
	V2RayCfg     *v2ray.ClientConfig
	OpenVPNCfg   *openvpn.ClientConfig
	XrayCfg      *xray.ClientConfig
	AmneziaWGCfg *amneziawg.ClientConfig
	Hysteria2Cfg *hysteria2.ClientConfig
}

// Build creates and initializes a ClientService by performing the handshake,
// applying configuration, and preparing it for use.
func (b *Builder) Build(ctx context.Context) (types.ClientService, error) {
	switch b.Type {
	case types.ServiceTypeWireGuard:
		return b.buildWireGuard(ctx)

	case types.ServiceTypeV2Ray:
		return b.buildV2Ray(ctx)

	case types.ServiceTypeOpenVPN:
		return b.buildOpenVPN(ctx)

	case types.ServiceTypeXray:
		return b.buildXray(ctx)

	case types.ServiceTypeAmneziaWG:
		return b.buildAmneziaWG(ctx)

	case types.ServiceTypeHysteria2:
		return b.buildHysteria2(ctx)

	case types.ServiceTypeUnspecified:
		return nil, fmt.Errorf("unspecified service type %q", b.Type)

	default:
		return nil, fmt.Errorf("unknown service type %q", b.Type)
	}
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

// buildOpenVPN performs the OpenVPN handshake and returns an initialized client service.
func (b *Builder) buildOpenVPN(ctx context.Context) (types.ClientService, error) {
	// Create a handshake request with a generated UUID.
	addReq := &openvpn.PeerRequest{UUID: uuid.New()}

	// Perform the handshake with the node.
	resp, err := b.Client.InitHandshake(ctx, b.ID, addReq)
	if err != nil {
		return nil, fmt.Errorf("performing node handshake: %w", err)
	}

	// Decode the handshake response.
	var addResp openvpn.AddPeerResponse
	if err := json.Unmarshal(resp.Data, &addResp); err != nil {
		return nil, fmt.Errorf("unmarshaling add peer response: %w", err)
	}

	// Apply handshake data to OpenVPN config.
	md := addResp.Metadata[0]
	b.OpenVPNCfg.Addr = resp.Addrs[0]
	b.OpenVPNCfg.Port = md.Port
	b.OpenVPNCfg.Protocol = md.Protocol
	b.OpenVPNCfg.CA = md.CA
	b.OpenVPNCfg.TLS = md.TLS
	b.OpenVPNCfg.Cert = addResp.Cert
	b.OpenVPNCfg.Key = addResp.Key

	// Create OpenVPN client and run PreUp.
	service := openvpn.NewClient("openvpn", b.HomeDir, b.OpenVPNCfg)
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

// buildAmneziaWG performs the AmneziaWG handshake and returns an initialized client service.
func (b *Builder) buildAmneziaWG(ctx context.Context) (types.ClientService, error) {
	// Create handshake request with public key.
	pk := b.AmneziaWGCfg.GetPrivateKey()
	addReq := &amneziawg.PeerRequest{PublicKey: pk.Public()}

	// Perform the handshake with the node.
	resp, err := b.Client.InitHandshake(ctx, b.ID, addReq)
	if err != nil {
		return nil, fmt.Errorf("performing node handshake: %w", err)
	}

	// Decode handshake response.
	var addResp amneziawg.AddPeerResponse
	if err := json.Unmarshal(resp.Data, &addResp); err != nil {
		return nil, fmt.Errorf("unmarshaling add peer response: %w", err)
	}

	// Apply handshake data to AmneziaWG config.
	md := addResp.Metadata[0]
	b.AmneziaWGCfg.Addrs = addResp.GetAddrs()
	b.AmneziaWGCfg.Peer.Addr = resp.Addrs[0]
	b.AmneziaWGCfg.Peer.Port = md.Port
	b.AmneziaWGCfg.Peer.PublicKey = md.PublicKey.String()

	// Apply handshake-affecting obfuscation parameters from metadata.
	b.AmneziaWGCfg.Obfs.S1 = md.S1
	b.AmneziaWGCfg.Obfs.S2 = md.S2
	b.AmneziaWGCfg.Obfs.S3 = md.S3
	b.AmneziaWGCfg.Obfs.S4 = md.S4
	b.AmneziaWGCfg.Obfs.H1 = md.H1
	b.AmneziaWGCfg.Obfs.H2 = md.H2
	b.AmneziaWGCfg.Obfs.H3 = md.H3
	b.AmneziaWGCfg.Obfs.H4 = md.H4
	b.AmneziaWGCfg.Obfs.I1 = md.I1
	b.AmneziaWGCfg.Obfs.I2 = md.I2
	b.AmneziaWGCfg.Obfs.I3 = md.I3
	b.AmneziaWGCfg.Obfs.I4 = md.I4
	b.AmneziaWGCfg.Obfs.I5 = md.I5

	// Create AmneziaWG client and run PreUp.
	service := amneziawg.NewClient("amneziawg", b.HomeDir, b.AmneziaWGCfg)
	if err := service.Init(true); err != nil {
		return nil, fmt.Errorf("running service init task: %w", err)
	}

	return service, nil
}

// buildHysteria2 performs the Hysteria2 handshake and returns an initialized client service.
func (b *Builder) buildHysteria2(ctx context.Context) (types.ClientService, error) {
	// Create a handshake request with the Hysteria2 UUID.
	addReq := &hysteria2.PeerRequest{UUID: b.Hysteria2Cfg.Auth}

	// Perform the handshake with the node.
	resp, err := b.Client.InitHandshake(ctx, b.ID, addReq)
	if err != nil {
		return nil, fmt.Errorf("performing node handshake: %w", err)
	}

	// Decode the handshake response.
	var addResp hysteria2.AddPeerResponse
	if err := json.Unmarshal(resp.Data, &addResp); err != nil {
		return nil, fmt.Errorf("unmarshaling add peer response: %w", err)
	}

	// Apply handshake data to Hysteria2 config.
	md := addResp.Metadata[0]
	b.Hysteria2Cfg.ServerAddr = net.JoinHostPort(resp.Addrs[0], strconv.Itoa(int(md.Port)))
	b.Hysteria2Cfg.TLSPin = md.TLSPin
	b.Hysteria2Cfg.ObfsPassword = md.ObfsPassword

	// Create Hysteria2 client and run PreUp.
	service := hysteria2.NewClient("hysteria2", b.HomeDir, b.Hysteria2Cfg)
	if err := service.Init(true); err != nil {
		return nil, fmt.Errorf("running service init task: %w", err)
	}

	return service, nil
}
