package types

import (
	"driftgo/lib/drift"
	oracles "driftgo/oracles/types"
	"math/big"
)

type DLOBNodeType int

const (
	NodeTypeTakingLimit DLOBNodeType = iota
	NodeTypeRestingLimit
	NodeTypeFloatingLimit
	NodeTypeMarket
	NodeTypeTrigger
)

func (value DLOBNodeType) String() string {
	switch value {
	case NodeTypeTrigger:
		return "trg"
	case NodeTypeMarket:
		return "mark"
	case NodeTypeFloatingLimit:
		return "fltLmt"
	case NodeTypeRestingLimit:
		return "rstLmt"
	case NodeTypeTakingLimit:
		return "takLmt"
	default:
		return "unknown"
	}
}

type DLOBNodeSubType int

const (
	NodeSubTypeAsk DLOBNodeSubType = iota
	NodeSubTypeBid
	NodeSubTypeAbove
	NodeSubTypeBelow
)

func (value DLOBNodeSubType) String() string {
	switch value {
	case NodeSubTypeAbove:
		return "above"
	case NodeSubTypeBelow:
		return "below"
	case NodeSubTypeBid:
		return "bid"
	case NodeSubTypeAsk:
		return "ask"
	default:
		return "unknownSubType"
	}
}

type IDLOBNode interface {
	GetPrice(oraclePriceData *oracles.OraclePriceData, slot uint64) *big.Int
	IsVammNode() bool
	IsBaseFilled() bool
	GetOrder() *drift.Order
	GetUserAccount() string
	IsHaveFilled() bool
	IsHaveTrigger() bool
	SetTrigger(bool)
	TryFill()
	ResetTryFill()
	GetTryFill() int64
}

type NodeToFill struct {
	Node       IDLOBNode
	MakerNodes []IDLOBNode
}

type NodeToTrigger struct {
	Node IDLOBNode
}

type OrderBookCallback func()

type NodeToUpdate struct {
	Side DLOBNodeSubType
	Node IDLOBNode
}
