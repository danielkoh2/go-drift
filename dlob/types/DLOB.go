package types

import (
	"driftgo/common"
	"driftgo/events"
	"driftgo/lib/drift"
	oracles "driftgo/oracles/types"
	types2 "driftgo/types"
	"github.com/gagliardetto/solana-go"
	"math/big"
)

type DLOBFilterFcn func(node IDLOBNode) bool

type IDLOB interface {
	GetRestingLimitBids(
		marketIndex uint16,
		slot uint64,
		marketType drift.MarketType,
		oraclePriceData *oracles.OraclePriceData,
		filterFcn DLOBFilterFcn,
	) *common.Generator[IDLOBNode, int]

	GetRestingLimitAsks(
		marketIndex uint16,
		slot uint64,
		marketType drift.MarketType,
		oraclePriceData *oracles.OraclePriceData,
		filterFcn DLOBFilterFcn,
	) *common.Generator[IDLOBNode, int]
	FindNodesToFill(
		marketIndex uint16,
		fallbackBid *big.Int,
		fallbackAsk *big.Int,
		slot uint64,
		ts int64,
		marketType drift.MarketType,
		oraclePriceData *oracles.OraclePriceData,
		stateAccount *drift.State,
		marketAccount *types2.MarketAccount,
	) []*NodeToFill
	FindNodesToTrigger(
		marketIndex uint16,
		slot uint64,
		oraclePrice *big.Int,
		marketType drift.MarketType,
		stateAccount *drift.State,
	) []*NodeToTrigger
	HandleOrderEvents(events []*events.WrappedEvent)

	HandleOrderRecord(record *drift.OrderRecord, slot uint64)

	HandleOrderActionRecord(record *drift.OrderActionRecord, slot uint64)

	UpdateByUser(solana.PublicKey, *drift.User, uint64)

	GetBestAsk(uint16, uint64, drift.MarketType, *oracles.OraclePriceData) *big.Int

	GetBestBid(uint16, uint64, drift.MarketType, *oracles.OraclePriceData) *big.Int

	GetBestMakers(uint16, drift.MarketType, drift.PositionDirection, uint64, *oracles.OraclePriceData, int) []solana.PublicKey

	EstimateFillExactBaseAmountInForSide(
		*big.Int,
		*oracles.OraclePriceData,
		uint64,
		*common.Generator[IDLOBNode, int],
	) *big.Int
	EstimateFillWithExactBaseAmount(
		uint16,
		drift.MarketType,
		*big.Int,
		drift.PositionDirection,
		uint64,
		*oracles.OraclePriceData,
	) *big.Int
}
