package dlob

import (
	"driftgo/dlob/types"
	"driftgo/lib/drift"
	"driftgo/math"
	oracles "driftgo/oracles/types"
	"math/big"
)

type DLOBNode struct {
	types.IDLOBNode
	Order       *drift.Order
	HaveFilled  bool
	HaveTrigger bool
	UserAccount string
}

type OrderNode struct {
	DLOBNode
	nodeType types.DLOBNodeType
	next     *OrderNode
	previous *OrderNode

	sortValue    int64
	HaveTrigger  bool
	tryFillCount int64
}

type VammNode struct {
	DLOBNode
	price *big.Int
}

func (p *OrderNode) GetPrice(
	oraclePriceData *oracles.OraclePriceData,
	slot uint64,
) *big.Int {
	return math.GetLimitPrice(p.Order, oraclePriceData, slot, nil)
}

func (p *OrderNode) IsBaseFilled() bool {
	return p.Order.BaseAssetAmountFilled == p.Order.BaseAssetAmount
}

func (p *OrderNode) IsVammNode() bool {
	return false
}

func (p *OrderNode) GetOrder() *drift.Order {
	return p.Order
}

func (p *OrderNode) GetUserAccount() string {
	return p.UserAccount
}

func (p *OrderNode) IsHaveFilled() bool {
	return p.HaveFilled // || p.Order.BaseAssetAmountFilled >= p.Order.BaseAssetAmount
}

func (p *OrderNode) IsHaveTrigger() bool {
	return p.HaveTrigger
}

func (p *OrderNode) SetTrigger(trigger bool) {
	p.HaveTrigger = trigger
}

func (p *OrderNode) TryFill() {
	p.tryFillCount++
}

func (p *OrderNode) ResetTryFill() {
	p.tryFillCount = 0
}

func (p *OrderNode) GetTryFill() int64 {
	return p.tryFillCount
}

func CreateNode(nodeType types.DLOBNodeType, order *drift.Order, userAccount string) *OrderNode {
	orderNode := &OrderNode{
		DLOBNode: DLOBNode{
			Order:       order,
			HaveFilled:  false,
			UserAccount: userAccount,
		},
		nodeType:    nodeType,
		sortValue:   0,
		HaveTrigger: false,
	}
	orderNode.sortValue = orderNode.GetSortValue(order)
	return orderNode
}

func (p *OrderNode) GetSortValue(order *drift.Order) int64 {
	switch p.nodeType {
	case types.NodeTypeTakingLimit:
		return int64(order.Slot)
	case types.NodeTypeRestingLimit:
		return int64(order.Price)
	case types.NodeTypeFloatingLimit:
		return int64(order.OraclePriceOffset)
	case types.NodeTypeMarket:
		return int64(order.Slot)
	case types.NodeTypeTrigger:
		return int64(order.TriggerPrice)
	}
	return 0
}
