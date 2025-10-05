package types

import (
	go_drift "driftgo"
	"driftgo/lib/geyser"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

type IProvider interface {
	GetConnection(...string) *rpc.Client
	GetWsConnection(...string) *ws.Client
	GetGeyserConnection(...string) *geyser.Connection
	GetProgram() IProgram
	SetProgram(IProgram)
	GetOpts() *go_drift.ConfirmOptions
	GetWallet() go_drift.IWallet
}

type IProgram interface {
	GetProgramId() solana.PublicKey
	GetProvider() IProvider
	GetAccounts(t any, accountName string) IAccountClient
}

type IAccountNamespace interface {
	Client(any, string) IAccountClient
}

type IAccountClient interface {
	Decode([]byte) interface{}
	FetchNullableAndContext(solana.PublicKey, rpc.CommitmentType) (interface{}, *rpc.Context)
	Fetch(solana.PublicKey, rpc.CommitmentType) interface{}
	FetchAndContext(
		address solana.PublicKey,
		commitment rpc.CommitmentType,
	) (interface{}, *rpc.Context)
	FetchMultiple(
		addresses []solana.PublicKey,
		commitment rpc.CommitmentType,
	) []interface{}
	FetchMultipleAndContext(
		addresses []solana.PublicKey,
		commitment rpc.CommitmentType,
	) ([]interface{}, *rpc.Context)
	All(filters []rpc.RPCFilter) []interface{}
}
