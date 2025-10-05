package anchor

import (
	go_drift "driftgo"
	"driftgo/anchor/types"
	"driftgo/connection"
	"driftgo/lib/geyser"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

type AnchorProvider struct {
	types.IProvider
	Wallet            go_drift.IWallet
	PublicKey         solana.PublicKey
	Opts              go_drift.ConfirmOptions
	ConnectionManager *connection.Manager
	Program           types.IProgram
}

func CreateAnchorProvider(
	wallet go_drift.IWallet,
	publicKey solana.PublicKey,
	opts go_drift.ConfirmOptions,
	connectionManager *connection.Manager,
) *AnchorProvider {
	return &AnchorProvider{
		Wallet:            wallet,
		PublicKey:         publicKey,
		Opts:              opts,
		ConnectionManager: connectionManager,
	}
}
func (p *AnchorProvider) GetWallet() go_drift.IWallet {
	return p.Wallet
}
func (p *AnchorProvider) GetConnection(id ...string) *rpc.Client {
	connection := p.ConnectionManager.GetRpc(id...)
	return connection
}

func (p *AnchorProvider) GetWsConnection(id ...string) *ws.Client {
	connection := p.ConnectionManager.GetWs(id...)
	return connection
}

func (p *AnchorProvider) GetGeyserConnection(id ...string) *geyser.Connection {
	connection := p.ConnectionManager.GetGeyser(id...)
	return connection
}

func (p *AnchorProvider) GetOpts() *go_drift.ConfirmOptions {
	return &p.Opts
}

func (p *AnchorProvider) GetProgram() types.IProgram {
	return p.Program
}

func (p *AnchorProvider) SetProgram(program types.IProgram) {
	p.Program = program
}
