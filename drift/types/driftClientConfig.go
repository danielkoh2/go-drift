package types

import (
	drift "driftgo"
	"driftgo/connection"
	"driftgo/drift/config"
	oracles "driftgo/oracles/types"
	"driftgo/tx"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type DriftClientConfig struct {
	Connection             *connection.Manager
	Wallet                 drift.IWallet
	Env                    config.DriftEnv
	ProgramId              solana.PublicKey
	AccountSubscription    DriftClientSubscriptionConfig
	Opts                   drift.ConfirmOptions
	TxSender               tx.ITxSender
	SubAccountIds          []uint16
	ActiveSubAccountId     uint16
	PerpMarketIndexes      []uint16
	SpotMarketIndexes      []uint16
	MarketLookupTable      solana.PublicKey
	OracleInfos            []oracles.OracleInfo
	UserStats              bool
	Authority              solana.PublicKey
	IncludeDelegates       bool
	AuthoritySubAccountMap map[string][]uint16
	SkipLoadUsers          bool
	TxParams               *drift.TxParams
}

type DriftClientSubscriptionConfig struct {
	ResubTimeoutMs *int64
	Commitment     *rpc.CommitmentType
}
