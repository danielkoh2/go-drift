package dlob

import (
	"driftgo/common"
	"driftgo/dlob/types"
	"driftgo/lib/drift"
	oracles "driftgo/oracles/types"
	"fmt"
	"math/big"
)

type SortDirection int

func GetOrderSignature(
	orderId uint32,
	userAccount string,
) string {
	return fmt.Sprintf("%s-%d", userAccount, orderId)
}

const (
	SortDirectionAsc SortDirection = iota
	SortDirectionDesc
)

type IDLOBNodeGenerator[T any] interface {
	GetGenerator()
}

type NodeList struct {
	IDLOBNodeGenerator[*OrderNode]
	nodeType      types.DLOBNodeType
	sortDirection SortDirection
	head          *OrderNode
	length        int
	nodeMap       map[string]*OrderNode
}

func CreateNodeList(nodeType types.DLOBNodeType, sortDirection SortDirection) *NodeList {
	return &NodeList{
		nodeType:      nodeType,
		sortDirection: sortDirection,
		head:          nil,
		length:        0,
		nodeMap:       make(map[string]*OrderNode),
	}
}

func (p *NodeList) GetLength() int {
	return p.length
}

func (p *NodeList) Clear() {
	p.head = nil
	p.length = 0
	p.nodeMap = make(map[string]*OrderNode)
}

func (p *NodeList) Insert(
	order *drift.Order,
	marketType drift.MarketType,
	userAccount string,
) {
	if order.Status == drift.OrderStatus_Init {
		return
	}
	orderSignature := GetOrderSignature(order.OrderId, userAccount)
	_, exists := p.nodeMap[orderSignature]
	if exists {
		return
	}
	newNode := CreateNode(p.nodeType, order, userAccount)
	p.nodeMap[orderSignature] = newNode
	p.length++

	if p.head == nil {
		p.head = newNode
		return
	}

	if p.PrependNode(p.head, newNode) {
		p.head.previous = newNode
		newNode.next = p.head
		p.head = newNode
		return
	}
	currentNode := p.head
	for currentNode.next != nil && !p.PrependNode(currentNode.next, newNode) {
		currentNode = currentNode.next
	}

	newNode.next = currentNode.next
	if currentNode.next != nil {
		newNode.next.previous = newNode
	}
	currentNode.next = newNode
	newNode.previous = currentNode
}

func (p *NodeList) PrependNode(
	currentNode *OrderNode,
	newNode *OrderNode,
) bool {
	currentOrder := currentNode.Order
	newOrder := newNode.Order

	currentOrderSortPrice := currentNode.sortValue
	newOrderSortPrice := newNode.sortValue

	if newOrderSortPrice == currentOrderSortPrice {
		return newOrder.Slot < currentOrder.Slot
	}

	if p.sortDirection == SortDirectionAsc {
		return newOrderSortPrice < currentOrderSortPrice
	} else {
		return newOrderSortPrice > currentOrderSortPrice
	}
}

func (p *NodeList) Update(
	order *drift.Order,
	userAccount string,
) {
	orderId := GetOrderSignature(order.OrderId, userAccount)
	node, exists := p.nodeMap[orderId]
	if exists {
		if node.Order != nil && node.Order.BaseAssetAmountFilled != order.BaseAssetAmountFilled {
			node.ResetTryFill()
		}
		node.Order = order
		node.HaveFilled = false
	}
}

func (p *NodeList) ResetTryFill(
	order *drift.Order,
	userAccount string,
) {
	orderId := GetOrderSignature(order.OrderId, userAccount)
	node, exists := p.nodeMap[orderId]
	if exists {
		node.ResetTryFill()
	}
}

func (p *NodeList) TryFill(
	order *drift.Order,
	userAccount string,
) {
	orderId := GetOrderSignature(order.OrderId, userAccount)
	node, exists := p.nodeMap[orderId]
	if exists {
		node.TryFill()
	}
}
func (p *NodeList) Remove(
	order *drift.Order,
	userAccount string,
) {
	orderId := GetOrderSignature(order.OrderId, userAccount)
	node, exists := p.nodeMap[orderId]
	if exists {
		if node.next != nil {
			node.next.previous = node.previous
		}
		if node.previous != nil {
			node.previous.next = node.next
		}

		if p.head != nil && node.Order.OrderId == p.head.Order.OrderId {
			p.head = node.next
		}

		node.previous = nil
		node.next = nil

		delete(p.nodeMap, orderId)

		p.length--
	}
}

func (p *NodeList) GetGenerator() *common.Generator[types.IDLOBNode, int] {
	return common.NewGenerator(func(yield common.YieldFn[types.IDLOBNode, int]) {
		var idx = 0
		for node := p.head; node != nil; node = node.next {
			yield(node, idx)
			idx++
		}
	})
}

func (p *NodeList) Has(order *drift.Order, userAccount string) bool {
	_, exists := p.nodeMap[GetOrderSignature(order.OrderId, userAccount)]
	return exists
}

func (p *NodeList) Get(orderSignature string) *OrderNode {
	v, exists := p.nodeMap[orderSignature]
	if exists {
		return v
	}
	return nil
}

func (p *VammNode) GetPrice(oraclePriceData *oracles.OraclePriceData, slot uint64) *big.Int {
	return p.price
}

func (p *VammNode) IsVammNode() bool {
	return true
}

func (p *VammNode) IsBaseFilled() bool {
	return false
}

func (p *VammNode) TryFill() {

}

func (p *VammNode) ResetTryFill() {

}

func (p *VammNode) GetTryFill() int64 {
	return 0
}

func GetVammNodeGenerator(price *big.Int) *common.Generator[types.IDLOBNode, int] {
	if price == nil {
		return nil
	}
	return common.NewGenerator(func(yield common.YieldFn[types.IDLOBNode, int]) {
		node := VammNode{
			DLOBNode: DLOBNode{
				Order:       nil,
				HaveFilled:  false,
				UserAccount: "",
			},
		}
		yield(&node, 0)
	})
}
