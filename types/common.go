package types

import (
	"driftgo/lib/drift"
	"github.com/gagliardetto/solana-go/rpc"
)

type ConfirmOptions struct {
	rpc.TransactionOpts
	Commitment rpc.CommitmentType
}

type MarketAccount struct {
	PerpMarketAccount *drift.PerpMarket
	SpotMarketAccount *drift.SpotMarket
}
