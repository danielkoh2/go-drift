package types

import (
	"driftgo/lib/drift"
	"driftgo/lib/jit_proxy"
	"github.com/gagliardetto/solana-go"
	"math/big"
)

type UserFilter func(userAccount *drift.User, userKey string, order *drift.Order) bool
type JitParams struct {
	Bid            *big.Int
	Ask            *big.Int
	MinPosition    *big.Int
	MaxPosition    *big.Int
	PriceType      *jit_proxy.PriceType
	SubAccountId   *uint16
	PostOnlyParams *jit_proxy.PostOnlyParam
}

type IBaseJitter interface {
	Subscribe()
	Unsubscribe()
	CreateTryFill(taker *drift.User, takerKey solana.PublicKey, takerStatsKey solana.PublicKey, order *drift.Order, orderSignature string, onComplete func())
	DeleteOnGoingAuction(orderSignature string)
	GetOrderSignatures(takerKey string, orderId uint32) string
	UpdatePerpParams(marketIndex uint16, params *JitParams)
	UpdateSpotParams(marketIndex uint16, params *JitParams)
	SetUserFilter(userFilter UserFilter)
	SetComputeUnits(computeUnits uint32)
	SetComputeUnitsPrice(computeUnitsPrice uint32)
}
