package serum

import (
	"driftgo/anchor/types"
	"github.com/gagliardetto/solana-go"
)

type SerumMarketSubscriberConfig struct {
	Program       types.IProgram
	ConnectionId  string
	ProgramId     solana.PublicKey
	MarketAddress solana.PublicKey
}
