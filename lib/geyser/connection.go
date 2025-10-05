package geyser

import (
	"context"
	"crypto/x509"
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"time"
)

var KACP = keepalive.ClientParameters{
	Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
	Timeout:             time.Second,      // wait 1 second for ping ack before considering the connection dead
	PermitWithoutStream: true,             // send pings even without active streams
}

type ConnectionConfig struct {
	Identity string
	Token    string
	Endpoint string
	Insecure bool
}

type Connection struct {
	Connection    *grpc.ClientConn
	Config        *ConnectionConfig
	Subscriptions []*Subscription
}

func NewConnection() *Connection {
	return &Connection{}
}

func (p *Connection) Connect(config ConnectionConfig) error {
	var opts []grpc.DialOption

	if config.Insecure { // Secure connection
		pool, _ := x509.SystemCertPool()
		tlsCredentials := credentials.NewClientTLSFromCert(pool, "")
		opts = append(opts, grpc.WithTransportCredentials(tlsCredentials), grpc.WithKeepaliveParams(KACP))

	} else { // Insecure connection
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	connection, err := grpc.Dial(config.Endpoint, opts...)
	if err != nil {
		return err
	}
	p.Config = &config
	p.Connection = connection
	return nil
}

func (p *Connection) Reconnect() error {
	if p.Config == nil {
		return errors.New("geyser: no connection configured")
	}
	return p.Connect(*p.Config)
}

func (p *Connection) GetState() string {
	if p.Connection == nil {
		return "INVALID_STATE"
	}
	return p.Connection.GetState().String()
}

func (p *Connection) IsConnected() bool {
	if p.Connection == nil {
		return false
	}
	state := p.GetState()
	return state == "READY" || state == "CONNECTING"
}

func (p *Connection) Stream() (solana_geyser_grpc.Geyser_SubscribeClient, solana_geyser_grpc.GeyserClient, func(), error) {
	ctx, cancel := context.WithCancel(context.Background())

	client := solana_geyser_grpc.NewGeyserClient(p.Connection)

	if p.Config.Token != "" {
		ctx = metadata.NewOutgoingContext(
			ctx,
			metadata.New(map[string]string{"x-token": p.Config.Token}),
		)
	}
	stream, err := client.Subscribe(ctx)
	if err != nil {
		cancel()
		return nil, nil, nil, err
	}
	return stream, client, cancel, nil
}

func (p *Connection) Subscription() (*Subscription, error) {
	stream, client, cancel, err := p.Stream()
	if err != nil {
		return nil, err
	}
	return &Subscription{Stream: stream, Client: client, Cancel: cancel}, nil
}

func (p *Connection) Close() {
	if p.Connection != nil {
		_ = p.Connection.Close()
		p.Connection = nil
	}
}
