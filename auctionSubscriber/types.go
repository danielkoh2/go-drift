package auctionSubscriber

import (
	go_drift "driftgo"
	"driftgo/drift/types"
	"driftgo/lib/drift"
	"github.com/gagliardetto/solana-go"
)

type AuctionSubscriberConfig struct {
	DriftClient    types.IDriftClient
	Opts           go_drift.ConfirmOptions
	ResubTimeoutMs int64
}

type AuctionSubscriberEvents interface {
	onAccountUpdate(
		account *drift.User,
		pubkey solana.PublicKey,
		slot uint64,
	)
}
