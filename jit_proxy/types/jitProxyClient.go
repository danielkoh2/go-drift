package types

import (
	go_drift "driftgo"
	"driftgo/lib/drift"
	"driftgo/lib/jit_proxy"
	"driftgo/tx"
	"github.com/gagliardetto/solana-go"
	"math/big"
)

type JitTxParams struct {
	TakerKey      solana.PublicKey
	TakerStatsKey solana.PublicKey
	Taker         *drift.User
	TakerOrderId  uint32
	MaxPosition   *big.Int
	MinPosition   *big.Int
	Bid           *big.Int
	Ask           *big.Int
	PostOnly      *jit_proxy.PostOnlyParam
	PriceType     *jit_proxy.PriceType
	ReferrerInfo  *go_drift.ReferrerInfo
	SubAccountId  *uint16
}
type IJitProxyClient interface {
	Jit(*JitTxParams, *go_drift.TxParams) (*tx.TxSigAndSlot, error)
	GetJitIx(*JitTxParams) (solana.Instruction, error)
	GetCheckOrderConstraintIx(uint16, []jit_proxy.OrderConstraint) (solana.Instruction, error)
	ArbPerp([]*go_drift.MakerInfo, uint16, *go_drift.TxParams) (*tx.TxSigAndSlot, error)
	GetArbPerpIx([]*go_drift.MakerInfo, uint16, *go_drift.ReferrerInfo) (solana.Instruction, error)
}
