package connection

import (
	"driftgo/lib/geyser"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

type RpcConnectionConfig struct {
	Endpoint string
	Headers  map[string]string
}

type WsConnectionConfig struct {
	Endpoint string
	Headers  map[string][]string
}

type GeyserConnectionConfig struct {
	Endpoint string
	Token    string
	InSecure bool
}

type IConnectionManager interface {
	CreateRpcConnection(...string) (*rpc.Client, string)
	CreateWsConnection(...string) (*ws.Client, string)
	CreateGeyserConnection(...string) (*geyser.Connection, string)
	AddRpcConnection(RpcConnectionConfig) (string, error)
	AddWsConnection(WsConnectionConfig) (string, error)
	AddGeyserConnection(GeyserConnectionConfig) (string, error)
}
