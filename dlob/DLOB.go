package dlob

import (
	"driftgo/common"
	"driftgo/constants"
	"driftgo/dlob/types"
	types4 "driftgo/drift/types"
	"driftgo/events"
	"driftgo/lib/drift"
	"driftgo/math"
	oracles "driftgo/oracles/types"
	types2 "driftgo/types"
	types3 "driftgo/userMap/types"
	"driftgo/utils"
	"github.com/gagliardetto/solana-go"
	"math/big"
	"sync"
)

//	type NodeToFill struct {
//		Node       *DLOBNode
//		MakerNodes []*DLOBNode
//	}
//
//	type NodeToTrigger struct {
//		Node *DLOBNode
//	}
type MarketNodeList map[types.DLOBNodeSubType]*NodeList

//	type MarketNodeList struct {
//		Ask *NodeList
//		Bid *NodeList
//		Above *NodeList
//		Below *NodeList
//	}
type MarketNodeLists map[types.DLOBNodeType]MarketNodeList

var SUPPORTED_ORDER_TYPES = []string{
	"market",
	"limit",
	"triggerMarket",
	"triggerLimit",
	"oracle",
}

type UserOrderAndSlot struct {
	Order *drift.Order
	Slot  uint64
}
type DLOB struct {
	types.IDLOB
	OpenOrders                   map[drift.MarketType]map[string]bool
	OrderLists                   map[drift.MarketType]map[uint16]MarketNodeLists
	UserOrderMap                 map[string]map[uint32]*UserOrderAndSlot
	UserSlotMap                  map[string]uint64
	MaxSlotForRestingLimitOrders uint64
	Initialized                  bool
	mxState                      *sync.RWMutex
}

func NewDLOB() *DLOB {
	dlob := &DLOB{
		OpenOrders:                   make(map[drift.MarketType]map[string]bool),
		OrderLists:                   make(map[drift.MarketType]map[uint16]MarketNodeLists),
		UserOrderMap:                 make(map[string]map[uint32]*UserOrderAndSlot),
		UserSlotMap:                  make(map[string]uint64),
		MaxSlotForRestingLimitOrders: 0,
		Initialized:                  false,
		mxState:                      new(sync.RWMutex),
	}
	dlob.Init()
	return dlob
}

func (p *DLOB) Init() {
	p.OpenOrders[drift.MarketType_Perp] = make(map[string]bool)
	p.OpenOrders[drift.MarketType_Spot] = make(map[string]bool)
	p.OrderLists[drift.MarketType_Perp] = make(map[uint16]MarketNodeLists)
	p.OrderLists[drift.MarketType_Spot] = make(map[uint16]MarketNodeLists)
}

func (p *DLOB) Clear() {
	defer p.mxState.Unlock()
	p.mxState.Lock()
	for marketType, _ := range p.OpenOrders {
		p.OpenOrders[marketType] = make(map[string]bool)
	}
	p.OpenOrders = make(map[drift.MarketType]map[string]bool)

	for marketType, _ := range p.OrderLists {
		for marketIndex, _ := range p.OrderLists[marketType] {
			for side, _ := range p.OrderLists[marketType][marketIndex] {
				p.OrderLists[marketType][marketIndex][side] = nil
			}
		}
	}
	p.OrderLists = make(map[drift.MarketType]map[uint16]MarketNodeLists)

	p.MaxSlotForRestingLimitOrders = 0
	p.UserOrderMap = make(map[string]map[uint32]*UserOrderAndSlot)

	p.Init()
}

func (p *DLOB) addUserOrderMap(userAccountKey string, order *drift.Order, slot uint64) bool {
	//orderKey := getOrderKey(order)
	_, exists := p.UserOrderMap[userAccountKey]
	if !exists {
		p.UserOrderMap[userAccountKey] = make(map[uint32]*UserOrderAndSlot)
	}
	userOrder, exists := p.UserOrderMap[userAccountKey][order.OrderId]
	if !exists || userOrder.Slot < slot {
		if exists {
			p.UserOrderMap[userAccountKey][order.OrderId].Order = order
			p.UserOrderMap[userAccountKey][order.OrderId].Slot = slot
		} else {
			p.UserOrderMap[userAccountKey][order.OrderId] = &UserOrderAndSlot{
				Order: order,
				Slot:  slot,
			}
		}
		return true
	}
	return false
}

func (p *DLOB) deleteUserOrderMap(userAccountKey string, order *drift.Order) {
	_, exists := p.UserOrderMap[userAccountKey]
	if !exists {
		return
	}
	delete(p.UserOrderMap[userAccountKey], order.OrderId)
}

func (p *DLOB) updateUserOrderMap(userAccountKey string, order *drift.Order, slot uint64) bool {
	_, exists := p.UserOrderMap[userAccountKey]
	if !exists {
		return false
	}
	_, orderExists := p.UserOrderMap[userAccountKey][order.OrderId]
	if !orderExists {
		return false
	}
	if p.UserOrderMap[userAccountKey][order.OrderId].Slot < slot {
		p.UserOrderMap[userAccountKey][order.OrderId].Order = order
		p.UserOrderMap[userAccountKey][order.OrderId].Slot = slot
		return true
	}
	return false
}
func (p *DLOB) InitFromUserMap(
	userMap types3.IUserMap,
	slot uint64,
) bool {
	if p.Initialized {
		return false
	}

	// initialize the dlob with the user map
	var orders []*types.BulkOrder
	userMap.Values().Each(func(user types4.IUser, key int) bool {
		userAccount := user.GetUserAccount()
		userAccountPubkey := user.GetUserAccountPublicKey()
		p.UserSlotMap[userAccountPubkey.String()] = slot
		//userAccountPubkeyString := userAccountPubkey.String()
		for idx := 0; idx < len(userAccount.Orders); idx++ {
			order := &userAccount.Orders[idx]
			if order.OrderId == 0 {
				continue
			}
			if order.Status == drift.OrderStatus_Init {
				continue
			}
			if !(order.OrderType == drift.OrderType_Market ||
				order.OrderType == drift.OrderType_Limit ||
				order.OrderType == drift.OrderType_TriggerMarket ||
				order.OrderType == drift.OrderType_TriggerLimit ||
				order.OrderType == drift.OrderType_Oracle) {
				continue
			}
			orders = append(orders, &types.BulkOrder{
				Order:       order,
				UserAccount: userAccountPubkey,
				Slot:        slot,
			})
		}
		return false
	})
	if len(orders) > 0 {
		p.InsertOrderBulk(orders)
	}
	p.Initialized = true
	return true
}

func (p *DLOB) InitFromOrders(dlobOrders DLOBOrders, slot uint64) bool {
	if p.Initialized {
		return false
	}

	for _, dlobOrder := range dlobOrders {
		p.InsertOrder(dlobOrder.Order, dlobOrder.User.String(), slot)
	}
	p.Initialized = true
	return true
}

func (p *DLOB) HandleOrderRecord(record *drift.OrderRecord, slot uint64) {
	p.InsertOrder(&record.Order, record.User.String(), slot)
}

func (p *DLOB) HandleOrderActionRecord(record *drift.OrderActionRecord, slot uint64) {
	if record.Action == drift.OrderAction_Place || record.Action == drift.OrderAction_Expire {
		return
	}
	if record.Action == drift.OrderAction_Trigger {
		if record.TakerOrderId != nil && record.Taker != nil {
			takerOrder, _ := p.GetOrder(*record.TakerOrderId, *record.Taker)
			if takerOrder != nil {
				p.Trigger(takerOrder, *record.Taker, slot)
			}
		}
		if record.MakerOrderId != nil && record.Maker != nil {
			makerOrder, _ := p.GetOrder(*record.MakerOrderId, *record.Maker)
			if makerOrder != nil {
				p.Trigger(makerOrder, *record.Maker, slot)
			}
		}
	} else if record.Action == drift.OrderAction_Fill {
		if record.TakerOrderId != nil && record.Taker != nil {
			takerOrder, _ := p.GetOrder(*record.TakerOrderId, *record.Taker)
			if takerOrder != nil {
				p.UpdateOrder(
					takerOrder,
					*record.Taker,
					slot,
					utils.BN(*record.TakerOrderCumulativeBaseAssetAmountFilled),
				)
			}
		}
		if record.MakerOrderId != nil && record.Maker != nil {
			makerOrder, _ := p.GetOrder(*record.MakerOrderId, *record.Maker)
			if makerOrder != nil {
				p.UpdateOrder(
					makerOrder,
					*record.Maker,
					slot,
					utils.BN(*record.MakerOrderCumulativeBaseAssetAmountFilled),
				)
			}
		}
	} else if record.Action == drift.OrderAction_Cancel {
		if record.TakerOrderId != nil && record.Taker != nil {
			takerOrder, _ := p.GetOrder(*record.TakerOrderId, *record.Taker)
			if takerOrder != nil {
				p.Delete(takerOrder, *record.Taker, slot)
			}
		}
		if record.MakerOrderId != nil && record.Maker != nil {
			makerOrder, _ := p.GetOrder(*record.MakerOrderId, *record.Maker)
			if makerOrder != nil {
				p.Delete(makerOrder, *record.Maker, slot)
			}
		}
	}
}

func (p *DLOB) UpdateByUser(userAccountKey solana.PublicKey, userAccount *drift.User, slot uint64) {
	key := userAccountKey.String()
	userOrderMap, orderMapExists := p.UserOrderMap[key]
	deletedOrders := make(map[uint32]*types.BulkOrder)
	var newOrders []*types.BulkOrder
	var updatedOrders []*types.BulkOrder

	var existOrders []uint32
	p.UserSlotMap[key] = slot
	if orderMapExists && userOrderMap != nil {
		for orderId, order := range userOrderMap {
			if order.Slot < slot {
				deletedOrders[orderId] = &types.BulkOrder{
					Order:       order.Order,
					UserAccount: userAccountKey,
					Slot:        slot,
				}
			}
		}
	}
	for idx := 0; idx < len(userAccount.Orders); idx++ {
		order := &userAccount.Orders[idx]
		if order.Status == drift.OrderStatus_Init {
			continue
		}
		if !(order.OrderType == drift.OrderType_Market ||
			order.OrderType == drift.OrderType_Limit ||
			order.OrderType == drift.OrderType_TriggerMarket ||
			order.OrderType == drift.OrderType_TriggerLimit ||
			order.OrderType == drift.OrderType_Oracle) {
			continue
		}
		existOrders = append(existOrders, order.OrderId)
		if orderMapExists && userOrderMap != nil {
			userOrder, exists := userOrderMap[order.OrderId]
			if exists {
				if userOrder.Slot < slot {
					if utils.BN(order.BaseAssetAmount).Cmp(utils.BN(order.BaseAssetAmountFilled)) == 0 {
						_, e := deletedOrders[order.OrderId]
						if !e {
							deletedOrders[order.OrderId] = &types.BulkOrder{
								Order:       order,
								UserAccount: userAccountKey,
								Slot:        slot,
							}
						}
						continue
					} else {
						updatedOrders = append(updatedOrders, &types.BulkOrder{
							Order:       order,
							UserAccount: userAccountKey,
							Slot:        slot,
						})
					}
				}
			} else {
				newOrders = append(newOrders, &types.BulkOrder{
					Order:       order,
					UserAccount: userAccountKey,
					Slot:        slot,
				})
			}
		} else {
			//p.InsertOrder(order, key, slot)
			newOrders = append(newOrders, &types.BulkOrder{
				Order:       order,
				UserAccount: userAccountKey,
				Slot:        slot,
			})
		}
		delete(deletedOrders, order.OrderId)
	}

	if !(len(newOrders) > 0 || len(updatedOrders) > 0 || len(deletedOrders) > 0) {
		//fmt.Printf("User order update (%s) skipped\n", userAccountKey.String())
		return
	}
	defer p.mxState.Unlock()
	p.mxState.Lock()
	//fmt.Printf("User order update (%s) : new %d, update %d, delete %d\n", userAccountKey.String(),
	//	len(newOrders),
	//	len(updatedOrders),
	//	len(deletedOrders),
	//)
	if len(newOrders) > 0 {
		p.InsertOrderBulk(newOrders)
	}
	if len(updatedOrders) > 0 || len(deletedOrders) > 0 {
		p.UpdateRestingLimitOrders(slot)
	}
	if len(updatedOrders) > 0 {
		p.UpdateOrderBulk(updatedOrders)
	}
	if len(deletedOrders) > 0 {
		p.DeleteBulk(utils.MapValues(deletedOrders))
	}
}

func (p *DLOB) GetUserSlot(key string) uint64 {
	slot, exists := p.UserSlotMap[key]
	if exists {
		return slot
	}
	return 0
}

func (p *DLOB) HandleOrderEvents(orderEvents []*events.WrappedEvent) {
	var newOrders []*types.BulkOrder
	var updatedOrders []*types.BulkOrder
	var triggeredOrders []*types.BulkOrder
	var deletedOrders []*types.BulkOrder

	var lastSlot uint64

	p.mxState.RLock()
	for _, event := range orderEvents {
		lastSlot = max(lastSlot, event.Slot)
		if event.EventType == events.EventTypeOrderRecord {
			record := event.Data.(*drift.OrderRecord)
			if record.Order.Status == drift.OrderStatus_Init {
				continue
			}
			if !(record.Order.OrderType == drift.OrderType_Market ||
				record.Order.OrderType == drift.OrderType_Limit ||
				record.Order.OrderType == drift.OrderType_TriggerMarket ||
				record.Order.OrderType == drift.OrderType_TriggerLimit ||
				record.Order.OrderType == drift.OrderType_Oracle) {
				continue
			}
			userOrder, orderSlot := p.GetOrder(record.Order.OrderId, record.User)
			if userOrder != nil {
				if orderSlot < event.Slot {
					updatedOrders = append(updatedOrders, &types.BulkOrder{
						Order:       &record.Order,
						UserAccount: record.User,
						Slot:        event.Slot,
					})
				}
			} else {
				newOrders = append(newOrders, &types.BulkOrder{
					Order:       &record.Order,
					UserAccount: record.User,
					Slot:        event.Slot,
				})
			}
		} else if event.EventType == events.EventTypeOrderActionRecord {
			record := event.Data.(*drift.OrderActionRecord)
			if record.Action == drift.OrderAction_Place || record.Action == drift.OrderAction_Expire {
				continue
			}
			if record.Action == drift.OrderAction_Trigger {
				if record.TakerOrderId != nil && record.Taker != nil {
					takerOrder, orderSlot := p.GetOrder(*record.TakerOrderId, *record.Taker)
					if takerOrder != nil && orderSlot <= event.Slot && takerOrder.Status != drift.OrderStatus_Init && math.IsTriggered(takerOrder) {
						triggeredOrders = append(triggeredOrders, &types.BulkOrder{
							Order:       takerOrder,
							UserAccount: *record.Taker,
							Slot:        event.Slot,
						})
					}
				}
				if record.MakerOrderId != nil && record.Maker != nil {
					makerOrder, orderSlot := p.GetOrder(*record.MakerOrderId, *record.Maker)
					if makerOrder != nil && orderSlot <= event.Slot && makerOrder.Status != drift.OrderStatus_Init && math.IsTriggered(makerOrder) {
						triggeredOrders = append(triggeredOrders, &types.BulkOrder{
							Order:       makerOrder,
							UserAccount: *record.Maker,
							Slot:        event.Slot,
						})
					}
				}
			} else if record.Action == drift.OrderAction_Fill {
				filledAmount := utils.BN(0)
				if record.TakerOrderCumulativeBaseAssetAmountFilled != nil {
					filledAmount = utils.BN(*record.TakerOrderCumulativeBaseAssetAmountFilled)
				}
				if record.TakerOrderId != nil && record.Taker != nil {
					takerOrder, orderSlot := p.GetOrder(*record.TakerOrderId, *record.Taker)
					if takerOrder != nil && orderSlot < event.Slot {
						if utils.BN(takerOrder.BaseAssetAmount).Cmp(filledAmount) == 0 {
							deletedOrders = append(deletedOrders, &types.BulkOrder{
								Order:            takerOrder,
								UserAccount:      *record.Taker,
								Slot:             event.Slot,
								BaseAmountFilled: nil,
							})
						} else {
							updatedOrders = append(updatedOrders, &types.BulkOrder{
								Order:            takerOrder,
								UserAccount:      *record.Taker,
								Slot:             event.Slot,
								BaseAmountFilled: filledAmount,
							})
						}
					}
				}
				if record.MakerOrderId != nil && record.Maker != nil {
					makerOrder, orderSlot := p.GetOrder(*record.MakerOrderId, *record.Maker)
					if makerOrder != nil && orderSlot < event.Slot {
						if utils.BN(makerOrder.BaseAssetAmount).Cmp(filledAmount) == 0 {
							deletedOrders = append(deletedOrders, &types.BulkOrder{
								Order:            makerOrder,
								UserAccount:      *record.Maker,
								Slot:             event.Slot,
								BaseAmountFilled: nil,
							})
						} else {
							updatedOrders = append(updatedOrders, &types.BulkOrder{
								Order:            makerOrder,
								UserAccount:      *record.Maker,
								Slot:             event.Slot,
								BaseAmountFilled: utils.BN(*record.MakerOrderCumulativeBaseAssetAmountFilled),
							})
						}
					}
				}
			} else if record.Action == drift.OrderAction_Cancel {
				if record.TakerOrderId != nil && record.Taker != nil {
					takerOrder, _ := p.GetOrder(*record.TakerOrderId, *record.Taker)
					if takerOrder != nil {
						deletedOrders = append(deletedOrders, &types.BulkOrder{
							Order:       takerOrder,
							UserAccount: *record.Taker,
							Slot:        event.Slot,
						})
					}
				}
				if record.MakerOrderId != nil && record.Maker != nil {
					makerOrder, _ := p.GetOrder(*record.MakerOrderId, *record.Maker)
					if makerOrder != nil {
						deletedOrders = append(deletedOrders, &types.BulkOrder{
							Order:       makerOrder,
							UserAccount: *record.Maker,
							Slot:        event.Slot,
						})
					}
				}
			}
		}
	}
	p.mxState.RUnlock()
	if len(newOrders) > 0 || len(updatedOrders) > 0 || len(triggeredOrders) > 0 || len(deletedOrders) > 0 {
		//fmt.Printf("Order Event handled : new %d, update %d, trigger %d, delete %d\n",
		//	len(newOrders),
		//	len(updatedOrders),
		//	len(triggeredOrders),
		//	len(deletedOrders),
		//)
		p.HandleBulkOrders(lastSlot, newOrders, updatedOrders, triggeredOrders, deletedOrders)
	}
}

func (p *DLOB) HandleBulkOrders(slot uint64, orders ...[]*types.BulkOrder) {
	var newOrders []*types.BulkOrder
	var updatedOrders []*types.BulkOrder
	var triggeredOrders []*types.BulkOrder
	var deletedOrders []*types.BulkOrder
	if len(orders) > 0 {
		newOrders = orders[0]
	}
	if len(orders) > 1 {
		updatedOrders = orders[1]
	}
	if len(orders) > 2 {
		triggeredOrders = orders[2]
	}
	if len(orders) > 3 {
		deletedOrders = orders[3]
	}
	if !(len(newOrders) > 0 || len(updatedOrders) > 0 || len(deletedOrders) > 0 || len(triggeredOrders) > 0) {
		return
	}
	defer p.mxState.Unlock()
	p.mxState.Lock()
	if len(newOrders) > 0 {
		p.InsertOrderBulk(newOrders)
	}
	if len(updatedOrders) > 0 || len(deletedOrders) > 0 || len(triggeredOrders) > 0 {
		p.UpdateRestingLimitOrders(slot)
	}
	if len(updatedOrders) > 0 {
		p.UpdateOrderBulk(updatedOrders)
	}
	if len(triggeredOrders) > 0 {
		p.TriggerBulk(triggeredOrders)
	}
	if len(deletedOrders) > 0 {
		p.DeleteBulk(deletedOrders)
	}
}
func (p *DLOB) InsertOrder(
	order *drift.Order,
	userAccount string,
	slot uint64,
	onInsert ...func(),
) {
	if order.Status == drift.OrderStatus_Init {
		return
	}
	if !(order.OrderType == drift.OrderType_Market ||
		order.OrderType == drift.OrderType_Limit ||
		order.OrderType == drift.OrderType_TriggerMarket ||
		order.OrderType == drift.OrderType_TriggerLimit ||
		order.OrderType == drift.OrderType_Oracle) {
		return
	}

	defer p.mxState.Unlock()
	p.mxState.Lock()
	marketType := order.MarketType

	_, exists := p.OrderLists[marketType][order.MarketIndex]
	if !exists {
		p.AddOrderList(marketType, order.MarketIndex)
	}

	if order.Status == drift.OrderStatus_Open {
		p.OpenOrders[marketType][GetOrderSignature(order.OrderId, userAccount)] = true
	}
	p.GetListForOrder(order, slot).Insert(order, marketType, userAccount)
	p.addUserOrderMap(userAccount, order, slot)

	if len(onInsert) > 0 {
		onInsert[0]()
	}
}

func (p *DLOB) InsertOrderBulk(
	orders []*types.BulkOrder,
) {
	for _, order := range orders {

		p.addUserOrderMap(order.UserAccount.String(), order.Order, order.Slot)

		marketType := order.Order.MarketType

		_, exists := p.OrderLists[marketType][order.Order.MarketIndex]
		if !exists {
			p.AddOrderList(marketType, order.Order.MarketIndex)
		}

		if order.Order.Status == drift.OrderStatus_Open {
			p.OpenOrders[marketType][GetOrderSignature(order.Order.OrderId, order.UserAccount.String())] = true
		}
		p.GetListForOrder(order.Order, order.Slot).Insert(order.Order, marketType, order.UserAccount.String())
	}
}

func (p *DLOB) AddOrderList(marketType drift.MarketType, marketIndex uint16) map[types.DLOBNodeType]MarketNodeList {
	//_, exists := p.OrderLists[marketType]
	//if !exists {
	//	panic(fmt.Sprintf("order list not set : %d - %d", marketType, marketIndex))
	//}
	p.OrderLists[marketType][marketIndex] = make(map[types.DLOBNodeType]MarketNodeList)
	p.OrderLists[marketType][marketIndex][types.NodeTypeRestingLimit] = make(map[types.DLOBNodeSubType]*NodeList)
	p.OrderLists[marketType][marketIndex][types.NodeTypeFloatingLimit] = make(map[types.DLOBNodeSubType]*NodeList)
	p.OrderLists[marketType][marketIndex][types.NodeTypeTakingLimit] = make(map[types.DLOBNodeSubType]*NodeList)
	p.OrderLists[marketType][marketIndex][types.NodeTypeMarket] = make(map[types.DLOBNodeSubType]*NodeList)
	p.OrderLists[marketType][marketIndex][types.NodeTypeTrigger] = make(map[types.DLOBNodeSubType]*NodeList)

	p.OrderLists[marketType][marketIndex][types.NodeTypeRestingLimit][types.NodeSubTypeAsk] = CreateNodeList(types.NodeTypeRestingLimit, SortDirectionAsc)
	p.OrderLists[marketType][marketIndex][types.NodeTypeRestingLimit][types.NodeSubTypeBid] = CreateNodeList(types.NodeTypeRestingLimit, SortDirectionDesc)

	p.OrderLists[marketType][marketIndex][types.NodeTypeFloatingLimit][types.NodeSubTypeAsk] = CreateNodeList(types.NodeTypeRestingLimit, SortDirectionAsc)
	p.OrderLists[marketType][marketIndex][types.NodeTypeFloatingLimit][types.NodeSubTypeBid] = CreateNodeList(types.NodeTypeRestingLimit, SortDirectionDesc)

	p.OrderLists[marketType][marketIndex][types.NodeTypeTakingLimit][types.NodeSubTypeAsk] = CreateNodeList(types.NodeTypeRestingLimit, SortDirectionAsc)
	p.OrderLists[marketType][marketIndex][types.NodeTypeTakingLimit][types.NodeSubTypeBid] = CreateNodeList(types.NodeTypeRestingLimit, SortDirectionAsc)

	p.OrderLists[marketType][marketIndex][types.NodeTypeMarket][types.NodeSubTypeAsk] = CreateNodeList(types.NodeTypeRestingLimit, SortDirectionAsc)
	p.OrderLists[marketType][marketIndex][types.NodeTypeMarket][types.NodeSubTypeBid] = CreateNodeList(types.NodeTypeRestingLimit, SortDirectionAsc)

	p.OrderLists[marketType][marketIndex][types.NodeTypeTrigger][types.NodeSubTypeAbove] = CreateNodeList(types.NodeTypeRestingLimit, SortDirectionAsc)
	p.OrderLists[marketType][marketIndex][types.NodeTypeTrigger][types.NodeSubTypeBelow] = CreateNodeList(types.NodeTypeRestingLimit, SortDirectionDesc)
	return p.OrderLists[marketType][marketIndex]
}

func (p *DLOB) UpdateOrder(
	order *drift.Order,
	userAccount solana.PublicKey,
	slot uint64,
	cumulativeBaseAssetAmountFilled *big.Int,
	onUpdate ...func(),
) {
	defer p.mxState.Unlock()
	p.mxState.Lock()

	p.updateUserOrderMap(userAccount.String(), order, slot)

	p.UpdateRestingLimitOrders(slot)

	if utils.BN(order.BaseAssetAmount).Cmp(cumulativeBaseAssetAmountFilled) == 0 {
		p.Delete(order, userAccount, slot)
	}

	if utils.BN(order.BaseAssetAmountFilled).Cmp(cumulativeBaseAssetAmountFilled) == 0 {
		return
	}
	p.GetListForOrder(order, slot).Update(order, userAccount.String())

	if len(onUpdate) > 0 {
		onUpdate[0]()
	}
}

func (p *DLOB) UpdateOrderBulk(
	orders []*types.BulkOrder,
) {
	for _, order := range orders {
		if !p.updateUserOrderMap(order.UserAccount.String(), order.Order, order.Slot) {
			continue
		}
		if order.BaseAmountFilled != nil && utils.BN(order.Order.BaseAssetAmountFilled).Cmp(order.BaseAmountFilled) == 0 {
			continue
		}
		p.GetListForOrder(order.Order, order.Slot).Update(order.Order, order.UserAccount.String())
	}
}

func (p *DLOB) Trigger(
	order *drift.Order,
	userAccount solana.PublicKey,
	slot uint64,
	onTrigger ...func(),
) {
	if order.Status == drift.OrderStatus_Init {
		return
	}

	defer p.mxState.Unlock()
	p.mxState.Lock()

	p.updateUserOrderMap(userAccount.String(), order, slot)
	p.UpdateRestingLimitOrders(slot)

	if math.IsTriggered(order) {
		return
	}
	var triggerList *NodeList
	if order.TriggerCondition == drift.OrderTriggerCondition_Above {
		triggerList = p.OrderLists[order.MarketType][order.MarketIndex][types.NodeTypeTrigger][types.NodeSubTypeAbove]
	} else {
		triggerList = p.OrderLists[order.MarketType][order.MarketIndex][types.NodeTypeTrigger][types.NodeSubTypeBelow]
	}
	triggerList.Remove(order, userAccount.String())

	orderList := p.GetListForOrder(order, slot)
	if orderList != nil {
		orderList.Insert(order, order.MarketType, userAccount.String())
	}
	if len(onTrigger) > 0 {
		onTrigger[0]()
	}
}

func (p *DLOB) TriggerBulk(
	orders []*types.BulkOrder,
) {
	for _, order := range orders {
		p.updateUserOrderMap(order.UserAccount.String(), order.Order, order.Slot)
		var triggerList *NodeList
		if order.Order.TriggerCondition == drift.OrderTriggerCondition_Above {
			triggerList = p.OrderLists[order.Order.MarketType][order.Order.MarketIndex][types.NodeTypeTrigger][types.NodeSubTypeAbove]
		} else {
			triggerList = p.OrderLists[order.Order.MarketType][order.Order.MarketIndex][types.NodeTypeTrigger][types.NodeSubTypeBelow]
		}
		triggerList.Remove(order.Order, order.UserAccount.String())
		orderList := p.GetListForOrder(order.Order, order.Slot)
		if orderList != nil {
			orderList.Insert(order.Order, order.Order.MarketType, order.UserAccount.String())
		}
	}
}
func (p *DLOB) Delete(
	order *drift.Order,
	userAccount solana.PublicKey,
	slot uint64,
	onDelete ...func(),
) {
	if order.Status == drift.OrderStatus_Init {
		return
	}

	defer p.mxState.Unlock()
	p.mxState.Lock()
	p.deleteUserOrderMap(userAccount.String(), order)
	p.UpdateRestingLimitOrders(slot)

	orderList := p.GetListForOrder(order, slot)
	if orderList != nil {
		orderList.Remove(order, userAccount.String())
	}
	if len(onDelete) > 0 {
		onDelete[0]()
	}

}

func (p *DLOB) DeleteBulk(
	orders []*types.BulkOrder,
) {
	for _, order := range orders {
		p.deleteUserOrderMap(order.UserAccount.String(), order.Order)
		orderList := p.GetListForOrder(order.Order, order.Slot)
		if orderList != nil {
			orderList.Remove(order.Order, order.UserAccount.String())
		}
	}
}

func GetNodeType(order *drift.Order, slot uint64) (types.DLOBNodeType, types.DLOBNodeSubType) {
	isInactiveTriggerOrder := math.MustBeTriggered(order) && !math.IsTriggered(order)

	var nodeType types.DLOBNodeType
	if isInactiveTriggerOrder {
		nodeType = types.NodeTypeTrigger
	} else if order.OrderType == drift.OrderType_Market ||
		order.OrderType == drift.OrderType_TriggerMarket ||
		order.OrderType == drift.OrderType_Oracle {
		nodeType = types.NodeTypeMarket
	} else if order.OraclePriceOffset != 0 {
		nodeType = types.NodeTypeFloatingLimit
	} else {
		nodeType = types.NodeTypeRestingLimit
		if !math.IsRestingLimitOrder(order, slot) {
			nodeType = types.NodeTypeTakingLimit
		}
	}

	var subType types.DLOBNodeSubType
	if isInactiveTriggerOrder {
		subType = types.NodeSubTypeAbove
		if order.TriggerCondition == drift.OrderTriggerCondition_Below {
			subType = types.NodeSubTypeBelow
		}
	} else {
		subType = types.NodeSubTypeAsk
		if order.Direction == drift.PositionDirection_Long {
			subType = types.NodeSubTypeBid
		}
	}
	return nodeType, subType
}

func (p *DLOB) GetListForOrder(
	order *drift.Order,
	slot uint64,
) *NodeList {
	nodeType, subType := GetNodeType(order, slot)
	_, exists := p.OrderLists[order.MarketType]
	if !exists {
		return nil
	}
	return p.OrderLists[order.MarketType][order.MarketIndex][nodeType][subType]
}

func (p *DLOB) UpdateRestingLimitOrders(slot uint64) {
	if slot <= p.MaxSlotForRestingLimitOrders {
		return
	}

	p.MaxSlotForRestingLimitOrders = slot

	p.updateRestingLimitOrdersForMarketType(slot, drift.MarketType_Perp)

	p.updateRestingLimitOrdersForMarketType(slot, drift.MarketType_Spot)
}

func (p *DLOB) updateRestingLimitOrdersForMarketType(
	slot uint64,
	marketType drift.MarketType,
) {
	for _, nodeLists := range p.OrderLists[marketType] {
		var nodesToUpdate []types.NodeToUpdate
		asks := nodeLists[types.NodeTypeTakingLimit][types.NodeSubTypeAsk].GetGenerator()
		asks.Each(func(node types.IDLOBNode, key int) bool {
			if !math.IsRestingLimitOrder(node.GetOrder(), slot) {
				return false
			}
			nodesToUpdate = append(nodesToUpdate, types.NodeToUpdate{
				Side: types.NodeSubTypeAsk,
				Node: node,
			})
			return false
		})
		bids := nodeLists[types.NodeTypeTakingLimit][types.NodeSubTypeBid].GetGenerator()
		bids.Each(func(node types.IDLOBNode, key int) bool {
			if !math.IsRestingLimitOrder(node.GetOrder(), slot) {
				return false
			}
			nodesToUpdate = append(nodesToUpdate, types.NodeToUpdate{
				Side: types.NodeSubTypeBid,
				Node: node,
			})
			return false
		})

		for _, nodeToUpdate := range nodesToUpdate {
			nodeLists[types.NodeTypeTakingLimit][nodeToUpdate.Side].Remove(
				nodeToUpdate.Node.GetOrder(),
				nodeToUpdate.Node.GetUserAccount(),
			)
			nodeLists[types.NodeTypeRestingLimit][nodeToUpdate.Side].Insert(
				nodeToUpdate.Node.GetOrder(),
				marketType,
				nodeToUpdate.Node.GetUserAccount(),
			)
		}
	}
}

func (p *DLOB) GetOrder(orderId uint32, userAccount solana.PublicKey) (*drift.Order, uint64) {
	userOrders, exists := p.UserOrderMap[userAccount.String()]
	if !exists {
		return nil, 0
	}
	userOrder, exists := userOrders[orderId]
	if !exists {
		return nil, 0
	}
	return userOrder.Order, userOrder.Slot
	//orderSignature := GetOrderSignature(orderId, userAccount.String())
	//var order *drift.Order
	//p.getNodeLists().Each(func(nodeList *NodeList, key int) bool {
	//	node := nodeList.Get(orderSignature)
	//	if node != nil {
	//		order = node.Order
	//		return true
	//	}
	//	return false
	//})
	//return order
}

func (p *DLOB) FindNodesToFill(
	marketIndex uint16,
	fallbackBid *big.Int,
	fallbackAsk *big.Int,
	slot uint64,
	ts int64,
	marketType drift.MarketType,
	oraclePriceData *oracles.OraclePriceData,
	stateAccount *drift.State,
	marketAccount *types2.MarketAccount,
) []*types.NodeToFill {
	if math.FillPaused(stateAccount, marketAccount) {
		return []*types.NodeToFill{}
	}
	defer p.mxState.RUnlock()
	p.mxState.RLock()
	isAmmPaused := math.AmmPaused(stateAccount, marketAccount)

	minAuctionDuration := 0
	if marketType == drift.MarketType_Perp {
		minAuctionDuration = int(stateAccount.MinPerpAuctionDuration)
	}

	makerRebateNumerator, makerRebateDenominator := p.GetMakerRebate(marketType, stateAccount, marketAccount)

	var nodeToFills []*types.NodeToFill

	restingLimitOrderNodesToFill := p.FindRestingLimitOrderNodesToFill(
		marketIndex,
		slot,
		marketType,
		oraclePriceData,
		isAmmPaused,
		minAuctionDuration,
		makerRebateNumerator,
		makerRebateDenominator,
		fallbackAsk,
		fallbackBid,
	)

	takingOrderNodesToFill := p.FindTakingNodesToFill(
		marketIndex,
		slot,
		marketType,
		oraclePriceData,
		isAmmPaused,
		minAuctionDuration,
		fallbackAsk,
		fallbackBid,
	)

	// get expired market nodes
	expiredNodesToFill := p.FindExpiredNodesToFill(
		marketIndex,
		ts,
		marketType,
	)

	nodeToFills = append(p.MergeNodesToFill(
		restingLimitOrderNodesToFill,
		takingOrderNodesToFill,
	), expiredNodesToFill...)
	return nodeToFills
}

func (p *DLOB) GetMakerRebate(
	marketType drift.MarketType,
	stateAccount *drift.State,
	marketAccount *types2.MarketAccount,
) (int64, int64) {
	var makerRebateNumerator int64 = 0
	var makerRebateDenominator int64 = 0
	if marketType == drift.MarketType_Perp {
		makerRebateNumerator = int64(stateAccount.PerpFeeStructure.FeeTiers[0].MakerRebateNumerator)
		makerRebateDenominator = int64(stateAccount.PerpFeeStructure.FeeTiers[0].MakerRebateDenominator)
	} else {
		makerRebateNumerator = int64(stateAccount.SpotFeeStructure.FeeTiers[0].MakerRebateNumerator)
		makerRebateDenominator = int64(stateAccount.SpotFeeStructure.FeeTiers[0].MakerRebateDenominator)
	}
	var feeAdjustment int64
	if marketAccount.PerpMarketAccount != nil {
		feeAdjustment = int64(marketAccount.PerpMarketAccount.FeeAdjustment)
	} else {
		feeAdjustment = 0
	}
	if feeAdjustment != 0 {
		makerRebateNumerator += (makerRebateNumerator * feeAdjustment) / 100
	}
	return makerRebateNumerator, makerRebateDenominator
}

func (p *DLOB) MergeNodesToFill(
	restingLimitOrderNodesToFill []*types.NodeToFill,
	takingOrderNodesToFill []*types.NodeToFill,
) []*types.NodeToFill {
	mergedNodesToFill := make(map[string]*types.NodeToFill)

	var mergeNodesToFillHelper = func(nodesToFillArray []*types.NodeToFill) {
		for _, nodeToFill := range nodesToFillArray {
			nodeSignature := GetOrderSignature(nodeToFill.Node.GetOrder().OrderId, nodeToFill.Node.GetUserAccount())

			_, exists := mergedNodesToFill[nodeSignature]
			if !exists {
				mergedNodesToFill[nodeSignature] = &types.NodeToFill{Node: nodeToFill.Node, MakerNodes: make([]types.IDLOBNode, 0)}
			}

			if len(nodeToFill.MakerNodes) > 0 {
				mergedNodesToFill[nodeSignature].MakerNodes = append(mergedNodesToFill[nodeSignature].MakerNodes, nodeToFill.MakerNodes...)
			}
		}
	}
	mergeNodesToFillHelper(restingLimitOrderNodesToFill)
	mergeNodesToFillHelper(takingOrderNodesToFill)
	return utils.MapValues(mergedNodesToFill)
}

func (p *DLOB) FindRestingLimitOrderNodesToFill(
	marketIndex uint16,
	slot uint64,
	marketType drift.MarketType,
	oraclePriceData *oracles.OraclePriceData,
	isAmmPaused bool,
	minAuctionDuration int,
	makerRebateNumerator int64,
	makerRebateDenominator int64,
	fallbackAsk *big.Int,
	fallbackBid *big.Int,
) []*types.NodeToFill {
	var nodesToFill []*types.NodeToFill

	crossingNodes := p.FindCrossingRestingLimitOrders(
		marketIndex,
		slot,
		marketType,
		oraclePriceData,
	)
	if len(crossingNodes) > 0 {
		nodesToFill = append(nodesToFill, crossingNodes...)
	}

	if fallbackBid != nil && !isAmmPaused {
		askGenerator := p.GetRestingLimitAsks(
			marketIndex,
			slot,
			marketType,
			oraclePriceData,
			nil,
		)

		fallbackBidWithBuffer := utils.SubX(
			fallbackBid,
			utils.DivX(
				utils.MulX(
					fallbackBid,
					utils.BN(makerRebateNumerator),
				),
				utils.BN(makerRebateDenominator),
			),
		)
		//
		asksCrossingFallback := p.FindNodesCrossingFallbackLiquidity(
			marketType,
			slot,
			oraclePriceData,
			askGenerator,
			func(askPrice *big.Int) bool {
				return askPrice.Cmp(fallbackBidWithBuffer) <= 0
			},
			minAuctionDuration,
		)

		if len(asksCrossingFallback) > 0 {
			nodesToFill = append(nodesToFill, asksCrossingFallback...)
		}
	}

	if fallbackAsk != nil && !isAmmPaused {
		bidGenerator := p.GetRestingLimitBids(
			marketIndex,
			slot,
			marketType,
			oraclePriceData,
			nil,
		)

		fallbackAskWithBuffer := utils.SubX(
			fallbackAsk,
			utils.DivX(
				utils.MulX(
					fallbackAsk,
					utils.BN(makerRebateNumerator),
				),
				utils.BN(makerRebateDenominator),
			),
		)
		//

		bidsCrossingFallback := p.FindNodesCrossingFallbackLiquidity(
			marketType,
			slot,
			oraclePriceData,
			bidGenerator,
			func(bidPrice *big.Int) bool {
				return bidPrice.Cmp(fallbackAskWithBuffer) >= 0
			},
			minAuctionDuration,
		)

		if len(bidsCrossingFallback) > 0 {
			nodesToFill = append(nodesToFill, bidsCrossingFallback...)
		}
	}

	return nodesToFill
}

func (p *DLOB) FindTakingNodesToFill(
	marketIndex uint16,
	slot uint64,
	marketType drift.MarketType,
	oraclePriceData *oracles.OraclePriceData,
	isAmmPaused bool,
	minAuctionDuration int,
	fallbackAsk *big.Int,
	fallbackBid *big.Int,
) []*types.NodeToFill {
	var nodesToFill []*types.NodeToFill

	takingOrderGenerator := p.GetTakingAsks(
		marketIndex,
		marketType,
		slot,
		oraclePriceData,
	)

	takingAsksCrossingBids := p.FindTakingNodesCrossingMakerNodes(
		marketIndex,
		slot,
		marketType,
		oraclePriceData,
		takingOrderGenerator,
		func(marketIndex uint16,
			slot uint64,
			marketType drift.MarketType,
			oraclePriceData *oracles.OraclePriceData,
			filterFcn types.DLOBFilterFcn,
		) *common.Generator[types.IDLOBNode, int] {
			return p.GetRestingLimitBids(marketIndex, slot, marketType, oraclePriceData, filterFcn)
		},
		func(takerPrice *big.Int, makerPrice *big.Int) bool {
			if marketType == drift.MarketType_Spot {
				if takerPrice == nil {
					return false
				}

				if fallbackBid != nil && makerPrice.Cmp(fallbackBid) < 0 {
					return false
				}
			}
			return takerPrice == nil || takerPrice.Cmp(makerPrice) <= 0
		},
	)

	nodesToFill = append(nodesToFill, takingAsksCrossingBids...)

	if fallbackBid != nil && !isAmmPaused {
		takingOrderGenerator = p.GetTakingAsks(
			marketIndex,
			marketType,
			slot,
			oraclePriceData,
		)
		takingAsksCrossingFallback := p.FindNodesCrossingFallbackLiquidity(
			marketType,
			slot,
			oraclePriceData,
			takingOrderGenerator,
			func(takerPrice *big.Int) bool {
				return takerPrice == nil || takerPrice.Cmp(fallbackBid) <= 0
			},
			minAuctionDuration,
		)

		nodesToFill = append(nodesToFill, takingAsksCrossingFallback...)
	}

	takingOrderGenerator = p.GetTakingBids(
		marketIndex,
		marketType,
		slot,
		oraclePriceData,
	)

	takingBidsToFill := p.FindTakingNodesCrossingMakerNodes(
		marketIndex,
		slot,
		marketType,
		oraclePriceData,
		takingOrderGenerator,
		func(marketIndex uint16,
			slot uint64,
			marketType drift.MarketType,
			oraclePriceData *oracles.OraclePriceData,
			filterFcn types.DLOBFilterFcn,
		) *common.Generator[types.IDLOBNode, int] {
			return p.GetRestingLimitBids(marketIndex, slot, marketType, oraclePriceData, filterFcn)
		},
		func(takerPrice *big.Int, makerPrice *big.Int) bool {
			if marketType == drift.MarketType_Spot {
				if takerPrice == nil {
					return false
				}

				if fallbackAsk != nil && makerPrice.Cmp(fallbackAsk) > 0 {
					return false
				}
			}
			return takerPrice == nil || takerPrice.Cmp(makerPrice) >= 0
		},
	)

	if len(takingBidsToFill) > 0 {
		nodesToFill = append(nodesToFill, takingBidsToFill...)
	}

	if fallbackAsk != nil && !isAmmPaused {
		takingOrderGenerator = p.GetTakingBids(
			marketIndex,
			marketType,
			slot,
			oraclePriceData,
		)
		takingBidsCrossingFallback := p.FindNodesCrossingFallbackLiquidity(
			marketType,
			slot,
			oraclePriceData,
			takingOrderGenerator,
			func(takerPrice *big.Int) bool {
				return takerPrice == nil || takerPrice.Cmp(fallbackAsk) >= 0
			},
			minAuctionDuration,
		)
		if len(takingBidsCrossingFallback) > 0 {
			nodesToFill = append(nodesToFill, takingBidsCrossingFallback...)
		}
	}

	return nodesToFill
}

type MakerDLOBNodeGeneratorFn func(
	marketIndex uint16,
	slot uint64,
	marketType drift.MarketType,
	oraclePriceData *oracles.OraclePriceData,
	filterFcn types.DLOBFilterFcn,
) *common.Generator[types.IDLOBNode, int]

func (p *DLOB) FindTakingNodesCrossingMakerNodes(
	marketIndex uint16,
	slot uint64,
	marketType drift.MarketType,
	oraclePriceData *oracles.OraclePriceData,
	takerNodeGenerator *common.Generator[types.IDLOBNode, int],
	makerNodeGeneratorFn MakerDLOBNodeGeneratorFn,
	doesCross func(*big.Int, *big.Int) bool,
) []*types.NodeToFill {
	var nodesToFill []*types.NodeToFill

	takerNodeGenerator.Each(func(takerNode types.IDLOBNode, key int) bool {
		makerNodeGenerator := makerNodeGeneratorFn(marketIndex, slot, marketType, oraclePriceData, nil)

		makerNodeGenerator.Each(func(makerNode types.IDLOBNode, key int) bool {
			sameUser := takerNode.GetUserAccount() == makerNode.GetUserAccount()
			if sameUser {
				return false
			}
			makerPrice := makerNode.GetPrice(oraclePriceData, slot)
			takerPrice := takerNode.GetPrice(oraclePriceData, slot)

			ordersCross := doesCross(takerPrice, makerPrice)
			if !ordersCross {
				// market orders aren't sorted by price, they are sorted by time, so we need to traverse
				// through all of em
				return true
			}

			nodesToFill = append(nodesToFill, &types.NodeToFill{
				Node:       takerNode,
				MakerNodes: []types.IDLOBNode{makerNode},
			})

			makerOrder := makerNode.GetOrder()
			takerOrder := takerNode.GetOrder()

			makerBaseRemaining := makerOrder.BaseAssetAmount - makerOrder.BaseAssetAmountFilled
			takerBaseRemaining := takerOrder.BaseAssetAmount - takerOrder.BaseAssetAmountFilled

			baseFilled := min(makerBaseRemaining, takerBaseRemaining)

			newMakerOrder := *makerOrder
			newMakerOrder.BaseAssetAmountFilled = makerOrder.BaseAssetAmountFilled + baseFilled
			p.GetListForOrder(&newMakerOrder, slot).Update(
				&newMakerOrder,
				makerNode.GetUserAccount(),
			)

			newTakerOrder := *takerOrder
			newTakerOrder.BaseAssetAmountFilled = takerOrder.BaseAssetAmountFilled + baseFilled
			p.GetListForOrder(&newTakerOrder, slot).Update(
				&newTakerOrder,
				takerNode.GetUserAccount(),
			)

			if newTakerOrder.BaseAssetAmountFilled == takerOrder.BaseAssetAmount {
				return true
			}
			return false
		})
		return false
	})

	return nodesToFill
}

func (p *DLOB) FindNodesCrossingFallbackLiquidity(
	marketType drift.MarketType,
	slot uint64,
	oraclePriceData *oracles.OraclePriceData,
	nodeGenerator *common.Generator[types.IDLOBNode, int],
	doesCross func(*big.Int) bool,
	minAuctionDuration int,
) []*types.NodeToFill {
	var nodesToFill []*types.NodeToFill

	nodeGenerator.Each(func(node types.IDLOBNode, key int) bool {
		if marketType == drift.MarketType_Spot && node.GetOrder() != nil && node.GetOrder().PostOnly {
			return false
		}

		nodePrice := math.GetLimitPrice(node.GetOrder(), oraclePriceData, slot, nil)

		// order crosses if there is no limit price or it crosses fallback price
		crosses := doesCross(nodePrice)

		// fallback is available if auction is complete or it's a spot order
		fallbackAvailable := marketType == drift.MarketType_Spot ||
			math.IsFallbackAvailableLiquiditySource(node.GetOrder(), minAuctionDuration, slot)

		if crosses && fallbackAvailable {
			nodesToFill = append(nodesToFill, &types.NodeToFill{
				Node:       node,
				MakerNodes: []types.IDLOBNode{}, // filled by fallback
			})
		}
		return false
	})
	return nodesToFill
}

func (p *DLOB) FindExpiredNodesToFill(
	marketIndex uint16,
	ts int64,
	marketType drift.MarketType,
) []*types.NodeToFill {
	nodeLists := p.OrderLists[marketType][marketIndex]

	if nodeLists == nil {
		return []*types.NodeToFill{}
	}

	// All bids/asks that can expire
	// dont try to expire limit orders with tif as its inefficient use of blockspace
	bidGenerators := []*common.Generator[types.IDLOBNode, int]{
		nodeLists[types.NodeTypeTakingLimit][types.NodeSubTypeBid].GetGenerator(),
		nodeLists[types.NodeTypeRestingLimit][types.NodeSubTypeBid].GetGenerator(),
		nodeLists[types.NodeTypeFloatingLimit][types.NodeSubTypeBid].GetGenerator(),
		nodeLists[types.NodeTypeMarket][types.NodeSubTypeBid].GetGenerator(),
	}

	askGenerators := []*common.Generator[types.IDLOBNode, int]{
		nodeLists[types.NodeTypeTakingLimit][types.NodeSubTypeAsk].GetGenerator(),
		nodeLists[types.NodeTypeRestingLimit][types.NodeSubTypeAsk].GetGenerator(),
		nodeLists[types.NodeTypeFloatingLimit][types.NodeSubTypeAsk].GetGenerator(),
		nodeLists[types.NodeTypeMarket][types.NodeSubTypeAsk].GetGenerator(),
	}

	var nodesToFill []*types.NodeToFill

	for _, bidGenerator := range bidGenerators {
		bidGenerator.Each(func(bid types.IDLOBNode, key int) bool {
			if math.IsOrderExpired(bid.GetOrder(), ts, true) {
				nodesToFill = append(nodesToFill, &types.NodeToFill{
					Node:       bid,
					MakerNodes: []types.IDLOBNode{},
				})
			}
			return false
		})
	}

	for _, askGenerator := range askGenerators {
		askGenerator.Each(func(ask types.IDLOBNode, key int) bool {
			if math.IsOrderExpired(ask.GetOrder(), ts, true) {
				nodesToFill = append(nodesToFill, &types.NodeToFill{
					Node:       ask,
					MakerNodes: []types.IDLOBNode{},
				})
			}
			return false
		})
	}

	return nodesToFill
}

func (p *DLOB) FindJitAuctionNodesToFill(
	marketIndex uint16,
	slot uint64,
	oraclePriceData *oracles.OraclePriceData,
	marketType drift.MarketType,
) []*types.NodeToFill {
	var nodesToFill []*types.NodeToFill
	// Then see if there are orders still in JIT auction
	marketBidGenerator := p.GetTakingBids(
		marketIndex,
		marketType,
		slot,
		oraclePriceData,
	)
	if marketBidGenerator != nil {
		marketBidGenerator.Each(func(marketBid types.IDLOBNode, key int) bool {
			nodesToFill = append(nodesToFill, &types.NodeToFill{
				Node:       marketBid,
				MakerNodes: []types.IDLOBNode{},
			})
			return false
		})
	}

	marketAskGenerator := p.GetTakingAsks(
		marketIndex,
		marketType,
		slot,
		oraclePriceData,
	)
	if marketAskGenerator != nil {
		marketAskGenerator.Each(func(marketAsk types.IDLOBNode, key int) bool {
			nodesToFill = append(nodesToFill, &types.NodeToFill{
				Node:       marketAsk,
				MakerNodes: []types.IDLOBNode{},
			})
			return false
		})
	}

	return nodesToFill
}

func (p *DLOB) GetTakingBids(
	marketIndex uint16,
	marketType drift.MarketType,
	slot uint64,
	oraclePriceData *oracles.OraclePriceData,
) *common.Generator[types.IDLOBNode, int] {
	orderLists, exists := p.OrderLists[marketType][marketIndex]
	if !exists || orderLists == nil {
		return nil
	}

	p.UpdateRestingLimitOrders(slot)

	generatorList := []*common.Generator[types.IDLOBNode, int]{
		orderLists[types.NodeTypeMarket][types.NodeSubTypeBid].GetGenerator(),
		orderLists[types.NodeTypeTakingLimit][types.NodeSubTypeBid].GetGenerator(),
	}

	return p.GetBestNode(
		generatorList,
		oraclePriceData,
		slot,
		func(bestNode types.IDLOBNode, currentNode types.IDLOBNode, slot uint64, oraclePriceData *oracles.OraclePriceData) bool {
			return bestNode.GetOrder().Slot < currentNode.GetOrder().Slot
		},
		nil,
	)
}

func (p *DLOB) GetTakingAsks(
	marketIndex uint16,
	marketType drift.MarketType,
	slot uint64,
	oraclePriceData *oracles.OraclePriceData,
) *common.Generator[types.IDLOBNode, int] {
	orderLists, exists := p.OrderLists[marketType][marketIndex]
	if !exists || orderLists == nil {
		return nil
	}

	p.UpdateRestingLimitOrders(slot)

	generatorList := []*common.Generator[types.IDLOBNode, int]{
		orderLists[types.NodeTypeMarket][types.NodeSubTypeAsk].GetGenerator(),
		orderLists[types.NodeTypeTakingLimit][types.NodeSubTypeAsk].GetGenerator(),
	}

	return p.GetBestNode(
		generatorList,
		oraclePriceData,
		slot,
		func(bestNode types.IDLOBNode, currentNode types.IDLOBNode, slot uint64, oraclePriceData *oracles.OraclePriceData) bool {
			return bestNode.GetOrder().Slot < currentNode.GetOrder().Slot
		},
		nil,
	)
}

type GeneratorItem struct {
	Next      types.IDLOBNode
	Done      bool
	Generator *common.Generator[types.IDLOBNode, int]
}

func (p *DLOB) GetBestNode(
	generatorList []*common.Generator[types.IDLOBNode, int],
	oraclePriceData *oracles.OraclePriceData,
	slot uint64,
	compareFn func(types.IDLOBNode, types.IDLOBNode, uint64, *oracles.OraclePriceData) bool,
	filterFcn types.DLOBFilterFcn,
) *common.Generator[types.IDLOBNode, int] {
	return common.NewGenerator(func(yield common.YieldFn[types.IDLOBNode, int]) {
		idx := 0
		var generators []*GeneratorItem
		for _, generator := range generatorList {
			nextNode, _, done := generator.Next()
			generators = append(generators, &GeneratorItem{
				Next:      nextNode,
				Done:      done,
				Generator: generator,
			})
		}
		sideExhausted := false

		//var bestGenerator *GeneratorItem = nil

		for sideExhausted == false {
			bestGenerator := utils.ArrayReduce(generators, func(bestGenerator *GeneratorItem, currentGenerator *GeneratorItem) *GeneratorItem {
				if currentGenerator.Done {
					return bestGenerator
				}
				if bestGenerator.Done {
					return currentGenerator
				}

				bestValue := bestGenerator.Next
				currentValue := currentGenerator.Next

				return utils.TT(compareFn(bestValue, currentValue, slot, oraclePriceData), bestGenerator, currentGenerator)
			})

			if bestGenerator.Done == false {
				// skip this node if it's already completely filled
				if bestGenerator.Next.IsBaseFilled() {
					bestGenerator.Next, _, bestGenerator.Done = bestGenerator.Generator.Next()
					continue
				}

				if filterFcn != nil && !filterFcn(bestGenerator.Next) {
					bestGenerator.Next, _, bestGenerator.Done = bestGenerator.Generator.Next()
					continue
				}

				yield(bestGenerator.Next, idx)
				idx++
				bestGenerator.Next, _, bestGenerator.Done = bestGenerator.Generator.Next()
			} else {
				sideExhausted = true
			}
		}

	})
}

func (p *DLOB) GetRestingLimitAsks(
	marketIndex uint16,
	slot uint64,
	marketType drift.MarketType,
	oraclePriceData *oracles.OraclePriceData,
	filterFcn types.DLOBFilterFcn,
) *common.Generator[types.IDLOBNode, int] {
	if marketType == drift.MarketType_Spot && oraclePriceData == nil {
		// error
	}

	p.UpdateRestingLimitOrders(slot)

	nodeLists, exists := p.OrderLists[marketType][marketIndex]
	if !exists || nodeLists == nil {
		p.AddOrderList(marketType, marketIndex)
		nodeLists, _ = p.OrderLists[marketType][marketIndex]
	}

	generatorList := []*common.Generator[types.IDLOBNode, int]{
		nodeLists[types.NodeTypeRestingLimit][types.NodeSubTypeAsk].GetGenerator(),
		nodeLists[types.NodeTypeFloatingLimit][types.NodeSubTypeAsk].GetGenerator(),
	}

	return p.GetBestNode(
		generatorList,
		oraclePriceData,
		slot,
		func(bestNode types.IDLOBNode, currentNode types.IDLOBNode, slot uint64, oraclePriceData *oracles.OraclePriceData) bool {
			return bestNode.GetPrice(oraclePriceData, slot).Cmp(
				currentNode.GetPrice(oraclePriceData, slot),
			) < 0
		},
		filterFcn,
	)
}

func (p *DLOB) GetRestingLimitBids(
	marketIndex uint16,
	slot uint64,
	marketType drift.MarketType,
	oraclePriceData *oracles.OraclePriceData,
	filterFcn types.DLOBFilterFcn,
) *common.Generator[types.IDLOBNode, int] {
	if marketType == drift.MarketType_Spot && oraclePriceData == nil {
		// error
	}

	p.UpdateRestingLimitOrders(slot)

	nodeLists, exists := p.OrderLists[marketType][marketIndex]
	if !exists || nodeLists == nil {
		p.AddOrderList(marketType, marketIndex)
		nodeLists, _ = p.OrderLists[marketType][marketIndex]
	}

	generatorList := []*common.Generator[types.IDLOBNode, int]{
		nodeLists[types.NodeTypeRestingLimit][types.NodeSubTypeBid].GetGenerator(),
		nodeLists[types.NodeTypeFloatingLimit][types.NodeSubTypeBid].GetGenerator(),
	}

	return p.GetBestNode(
		generatorList,
		oraclePriceData,
		slot,
		func(bestNode types.IDLOBNode, currentNode types.IDLOBNode, slot uint64, oraclePriceData *oracles.OraclePriceData) bool {
			return bestNode.GetPrice(oraclePriceData, slot).Cmp(
				currentNode.GetPrice(oraclePriceData, slot),
			) > 0
		},
		filterFcn,
	)
}

func (p *DLOB) GetAsks(
	marketIndex uint16,
	fallbackAsk *big.Int,
	slot uint64,
	marketType drift.MarketType,
	oraclePriceData *oracles.OraclePriceData,
) *common.Generator[types.IDLOBNode, int] {
	if marketType == drift.MarketType_Spot && oraclePriceData == nil {
		//error
	}

	generatorList := []*common.Generator[types.IDLOBNode, int]{
		p.GetTakingAsks(marketIndex, marketType, slot, oraclePriceData),
		p.GetRestingLimitAsks(marketIndex, slot, marketType, oraclePriceData, nil),
	}

	if marketType == drift.MarketType_Perp && fallbackAsk != nil {
		generatorList = append(generatorList, GetVammNodeGenerator(fallbackAsk))
	}
	return p.GetBestNode(
		generatorList,
		oraclePriceData,
		slot,
		func(bestNode types.IDLOBNode, currentNode types.IDLOBNode, slot uint64, oraclePriceData *oracles.OraclePriceData) bool {
			bestNodeTaking := bestNode.GetOrder() != nil && math.IsTakingOrder(bestNode.GetOrder(), slot)
			currentNodeTaking := currentNode.GetOrder() != nil && math.IsTakingOrder(currentNode.GetOrder(), slot)

			if bestNodeTaking && currentNodeTaking {
				return bestNode.GetOrder().Slot < currentNode.GetOrder().Slot
			}
			if bestNodeTaking {
				return true
			}
			if currentNodeTaking {
				return false
			}
			return bestNode.GetPrice(oraclePriceData, slot).Cmp(
				currentNode.GetPrice(oraclePriceData, slot),
			) < 0
		},
		nil,
	)
}

func (p *DLOB) GetBids(
	marketIndex uint16,
	fallbackBid *big.Int,
	slot uint64,
	marketType drift.MarketType,
	oraclePriceData *oracles.OraclePriceData,
) *common.Generator[types.IDLOBNode, int] {
	if marketType == drift.MarketType_Spot && oraclePriceData == nil {
		//error
	}

	generatorList := []*common.Generator[types.IDLOBNode, int]{
		p.GetTakingBids(marketIndex, marketType, slot, oraclePriceData),
		p.GetRestingLimitBids(marketIndex, slot, marketType, oraclePriceData, nil),
	}

	if marketType == drift.MarketType_Perp && fallbackBid != nil {
		generatorList = append(generatorList, GetVammNodeGenerator(fallbackBid))
	}
	return p.GetBestNode(
		generatorList,
		oraclePriceData,
		slot,
		func(bestNode types.IDLOBNode, currentNode types.IDLOBNode, slot uint64, oraclePriceData *oracles.OraclePriceData) bool {
			bestNodeTaking := bestNode.GetOrder() != nil && math.IsTakingOrder(bestNode.GetOrder(), slot)
			currentNodeTaking := currentNode.GetOrder() != nil && math.IsTakingOrder(currentNode.GetOrder(), slot)

			if bestNodeTaking && currentNodeTaking {
				return bestNode.GetOrder().Slot < currentNode.GetOrder().Slot
			}
			if bestNodeTaking {
				return true
			}
			if currentNodeTaking {
				return false
			}
			return bestNode.GetPrice(oraclePriceData, slot).Cmp(
				currentNode.GetPrice(oraclePriceData, slot),
			) > 0
		},
		nil,
	)
}

func (p *DLOB) FindCrossingRestingLimitOrders(
	marketIndex uint16,
	slot uint64,
	marketType drift.MarketType,
	oraclePriceData *oracles.OraclePriceData,
) []*types.NodeToFill {
	nodesToFill := []*types.NodeToFill{}

	p.GetRestingLimitAsks(
		marketIndex,
		slot,
		marketType,
		oraclePriceData,
		nil,
	).Each(func(askNode types.IDLOBNode, key int) bool {
		p.GetRestingLimitBids(
			marketIndex,
			slot,
			marketType,
			oraclePriceData,
			nil,
		).Each(func(bidNode types.IDLOBNode, key int) bool {
			bidPrice := bidNode.GetPrice(oraclePriceData, slot)
			askPrice := askNode.GetPrice(oraclePriceData, slot)

			// orders don't cross
			if bidPrice.Cmp(askPrice) < 0 {
				return true
			}

			bidOrder := bidNode.GetOrder()
			askOrder := askNode.GetOrder()

			// Can't match orders from the same user
			sameUser := bidNode.GetUserAccount() == askNode.GetUserAccount()
			if sameUser {
				return false
			}

			takerNode, makerNode := p.DetermineMakerAndTaker(askNode, bidNode)

			// unable to match maker and taker due to post only or slot
			if makerNode == nil || takerNode == nil {
				return false
			}

			bidBaseRemaining := bidOrder.BaseAssetAmount - bidOrder.BaseAssetAmountFilled
			askBaseRemaining := askOrder.BaseAssetAmount - askOrder.BaseAssetAmountFilled

			baseFilled := min(bidBaseRemaining, askBaseRemaining)

			newBidOrder := *bidOrder
			newBidOrder.BaseAssetAmountFilled = bidOrder.BaseAssetAmountFilled + baseFilled
			p.GetListForOrder(&newBidOrder, slot).Update(
				&newBidOrder,
				bidNode.GetUserAccount(),
			)

			// ask completely filled
			newAskOrder := *askOrder
			newAskOrder.BaseAssetAmountFilled = askOrder.BaseAssetAmountFilled + baseFilled
			p.GetListForOrder(&newAskOrder, slot).Update(
				&newAskOrder,
				askNode.GetUserAccount(),
			)

			nodesToFill = append(nodesToFill, &types.NodeToFill{
				Node:       takerNode,
				MakerNodes: []types.IDLOBNode{makerNode},
			})

			if newAskOrder.BaseAssetAmount == newAskOrder.BaseAssetAmountFilled {
				return true
			}
			return false
		})
		return false
	})
	return nodesToFill
}

func (p *DLOB) DetermineMakerAndTaker(
	askNode types.IDLOBNode,
	bidNode types.IDLOBNode,
) (types.IDLOBNode, types.IDLOBNode) {
	askSlot := askNode.GetOrder().Slot + uint64(askNode.GetOrder().AuctionDuration)
	bidSlot := bidNode.GetOrder().Slot + uint64(bidNode.GetOrder().AuctionDuration)

	if bidNode.GetOrder().PostOnly && askNode.GetOrder().PostOnly {
		return nil, nil
	} else if bidNode.GetOrder().PostOnly {
		return askNode, bidNode
	} else if askNode.GetOrder().PostOnly {
		return bidNode, askNode
	} else if askSlot <= bidSlot {
		return bidNode, askNode
	} else {
		return askNode, bidNode
	}

}

func (p *DLOB) GetBestAsk(
	marketIndex uint16,
	slot uint64,
	marketType drift.MarketType,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	bestAsk, _, _ := p.GetRestingLimitAsks(
		marketIndex,
		slot,
		marketType,
		oraclePriceData,
		nil,
	).Next()
	if bestAsk != nil {
		return bestAsk.GetPrice(oraclePriceData, slot)
	}
	return nil
}

func (p *DLOB) GetBestBid(
	marketIndex uint16,
	slot uint64,
	marketType drift.MarketType,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	bestBid, _, _ := p.GetRestingLimitBids(
		marketIndex,
		slot,
		marketType,
		oraclePriceData,
		nil,
	).Next()
	if bestBid != nil {
		return bestBid.GetPrice(oraclePriceData, slot)
	}
	return nil
}

func (p *DLOB) GetStopLosses(
	marketIndex uint16,
	marketType drift.MarketType,
	direction drift.PositionDirection,
) *common.Generator[types.IDLOBNode, int] {
	return common.NewGenerator[types.IDLOBNode, int](func(yield common.YieldFn[types.IDLOBNode, int]) {
		marketNodeLists, exists := p.OrderLists[marketType][marketIndex]
		if !exists {
			return
		}
		idx := 0
		if direction == drift.PositionDirection_Long && marketNodeLists[types.NodeTypeTrigger][types.NodeSubTypeBelow] != nil {
			nodeGenerator := marketNodeLists[types.NodeTypeTrigger][types.NodeSubTypeBelow].GetGenerator()
			nodeGenerator.Each(func(node types.IDLOBNode, key int) bool {
				if node.GetOrder().Direction == drift.PositionDirection_Short {
					yield(node, idx)
					idx++
				}
				return false
			})
		} else if direction == drift.PositionDirection_Short && marketNodeLists[types.NodeTypeTrigger][types.NodeSubTypeAbove] != nil {
			nodeGenerator := marketNodeLists[types.NodeTypeTrigger][types.NodeSubTypeAbove].GetGenerator()
			nodeGenerator.Each(func(node types.IDLOBNode, key int) bool {
				if node.GetOrder().Direction == drift.PositionDirection_Long {
					yield(node, idx)
					idx++
				}
				return false
			})
		}
	})
}

func (p *DLOB) GetStopLossMarkets(
	marketIndex uint16,
	marketType drift.MarketType,
	direction drift.PositionDirection,
) *common.Generator[types.IDLOBNode, int] {
	return common.NewGenerator[types.IDLOBNode, int](func(yield common.YieldFn[types.IDLOBNode, int]) {
		generator := p.GetStopLosses(marketIndex, marketType, direction)
		idx := 0
		generator.Each(func(node types.IDLOBNode, key int) bool {
			if node.GetOrder().OrderType == drift.OrderType_TriggerMarket {
				yield(node, idx)
				idx++
			}
			return false
		})
	})
}

func (p *DLOB) GetStopLossLimits(
	marketIndex uint16,
	marketType drift.MarketType,
	direction drift.PositionDirection,
) *common.Generator[types.IDLOBNode, int] {
	return common.NewGenerator[types.IDLOBNode, int](func(yield common.YieldFn[types.IDLOBNode, int]) {
		generator := p.GetStopLosses(marketIndex, marketType, direction)
		idx := 0
		generator.Each(func(node types.IDLOBNode, key int) bool {
			if node.GetOrder().OrderType == drift.OrderType_TriggerLimit {
				yield(node, idx)
				idx++
			}
			return false
		})
	})
}

func (p *DLOB) GetTakeProfits(
	marketIndex uint16,
	marketType drift.MarketType,
	direction drift.PositionDirection,
) *common.Generator[types.IDLOBNode, int] {
	return common.NewGenerator[types.IDLOBNode, int](func(yield common.YieldFn[types.IDLOBNode, int]) {
		marketNodeLists, exists := p.OrderLists[marketType][marketIndex]
		if !exists {
			return
		}
		idx := 0
		if direction == drift.PositionDirection_Long && marketNodeLists[types.NodeTypeTrigger][types.NodeSubTypeAbove] != nil {
			generator := marketNodeLists[types.NodeTypeTrigger][types.NodeSubTypeAbove].GetGenerator()
			generator.Each(func(node types.IDLOBNode, key int) bool {
				if node.GetOrder().Direction == drift.PositionDirection_Short {
					yield(node, idx)
					idx++
				}
				return false
			})
		} else if direction == drift.PositionDirection_Long && marketNodeLists[types.NodeTypeTrigger][types.NodeSubTypeBelow] != nil {
			generator := marketNodeLists[types.NodeTypeTrigger][types.NodeSubTypeBelow].GetGenerator()
			generator.Each(func(node types.IDLOBNode, key int) bool {
				if node.GetOrder().Direction == drift.PositionDirection_Long {
					yield(node, idx)
					idx++
				}
				return false
			})
		}
	})
}

func (p *DLOB) GetTakeProfitMarkets(
	marketIndex uint16,
	marketType drift.MarketType,
	direction drift.PositionDirection,
) *common.Generator[types.IDLOBNode, int] {
	return common.NewGenerator[types.IDLOBNode, int](func(yield common.YieldFn[types.IDLOBNode, int]) {
		generator := p.GetTakeProfits(marketIndex, marketType, direction)
		idx := 0
		generator.Each(func(node types.IDLOBNode, key int) bool {
			if node.GetOrder().OrderType == drift.OrderType_TriggerMarket {
				yield(node, idx)
				idx++
			}
			return false
		})
	})
}

func (p *DLOB) GetTakeProfitLimits(
	marketIndex uint16,
	marketType drift.MarketType,
	direction drift.PositionDirection,
) *common.Generator[types.IDLOBNode, int] {
	return common.NewGenerator[types.IDLOBNode, int](func(yield common.YieldFn[types.IDLOBNode, int]) {
		generator := p.GetTakeProfits(marketIndex, marketType, direction)
		idx := 0
		generator.Each(func(node types.IDLOBNode, key int) bool {
			if node.GetOrder().OrderType == drift.OrderType_TriggerLimit {
				yield(node, idx)
				idx++
			}
			return false
		})
	})
}

func (p *DLOB) FindNodesToTrigger(
	marketIndex uint16,
	slot uint64,
	oraclePrice *big.Int,
	marketType drift.MarketType,
	stateAccount *drift.State,
) []*types.NodeToTrigger {
	if math.ExchangePaused(stateAccount) {
		return []*types.NodeToTrigger{}
	}
	defer p.mxState.RUnlock()
	p.mxState.RLock()

	nodesToTrigger := []*types.NodeToTrigger{}
	marketNodeLists, exists := p.OrderLists[marketType][marketIndex]

	var triggerAboveList *NodeList
	if exists && marketNodeLists != nil {
		triggerAboveList = marketNodeLists[types.NodeTypeTrigger][types.NodeSubTypeAbove]
	}
	if triggerAboveList != nil {
		triggerAboveList.GetGenerator().Each(func(node types.IDLOBNode, key int) bool {
			//fmt.Printf("Trigger above: oracle - %s, trigger - %d, market - %d\n", oraclePrice.String(), node.GetOrder().TriggerPrice, node.GetOrder().MarketIndex)
			if oraclePrice.Cmp(utils.BN(node.GetOrder().TriggerPrice)) > 0 {
				nodesToTrigger = append(nodesToTrigger, &types.NodeToTrigger{Node: node})
			} else {
				return true
			}
			return false
		})
	}

	var triggerBelowList *NodeList
	if exists && marketNodeLists != nil {
		triggerBelowList = marketNodeLists[types.NodeTypeTrigger][types.NodeSubTypeBelow]
	}
	if triggerBelowList != nil {
		triggerBelowList.GetGenerator().Each(func(node types.IDLOBNode, key int) bool {
			//fmt.Printf("Trigger below: oracle - %s, trigger - %d, market - %d\n", oraclePrice.String(), node.GetOrder().TriggerPrice, node.GetOrder().MarketIndex)
			if oraclePrice.Cmp(utils.BN(node.GetOrder().TriggerPrice)) < 0 {
				nodesToTrigger = append(nodesToTrigger, &types.NodeToTrigger{Node: node})
			} else {
				return true
			}
			return false
		})
	}

	return nodesToTrigger
}

func (p *DLOB) GetDLOBOrders() []*DLOBOrder {
	var dlobOrders []*DLOBOrder

	generator := p.getNodeLists()
	generator.Each(func(nodeList *NodeList, key int) bool {
		nodeGenerator := nodeList.GetGenerator()
		nodeGenerator.Each(func(node types.IDLOBNode, key int) bool {
			dlobOrders = append(dlobOrders, &DLOBOrder{
				User:  solana.MPK(node.GetUserAccount()),
				Order: node.GetOrder(),
			})
			return false
		})
		return false
	})
	return dlobOrders
}

func (p *DLOB) getNodeLists() *common.Generator[*NodeList, int] {
	return common.NewGenerator(func(yield common.YieldFn[*NodeList, int]) {
		var idx = 0
		for _, marketNodeLists := range p.OrderLists[drift.MarketType_Perp] {
			yield(marketNodeLists[types.NodeTypeRestingLimit][types.NodeSubTypeBid], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeRestingLimit][types.NodeSubTypeAsk], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeTakingLimit][types.NodeSubTypeBid], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeTakingLimit][types.NodeSubTypeAsk], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeMarket][types.NodeSubTypeBid], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeMarket][types.NodeSubTypeBid], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeFloatingLimit][types.NodeSubTypeBid], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeFloatingLimit][types.NodeSubTypeBid], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeTrigger][types.NodeSubTypeAbove], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeTrigger][types.NodeSubTypeBelow], idx)
			idx++
		}
		for _, marketNodeLists := range p.OrderLists[drift.MarketType_Spot] {
			yield(marketNodeLists[types.NodeTypeRestingLimit][types.NodeSubTypeBid], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeRestingLimit][types.NodeSubTypeAsk], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeTakingLimit][types.NodeSubTypeBid], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeTakingLimit][types.NodeSubTypeAsk], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeMarket][types.NodeSubTypeBid], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeMarket][types.NodeSubTypeBid], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeFloatingLimit][types.NodeSubTypeBid], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeFloatingLimit][types.NodeSubTypeBid], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeTrigger][types.NodeSubTypeAbove], idx)
			idx++
			yield(marketNodeLists[types.NodeTypeTrigger][types.NodeSubTypeBelow], idx)
			idx++
		}

	})
}

func (p *DLOB) EstimateFillExactBaseAmountInForSide(
	baseAmountIn *big.Int,
	oraclePriceData *oracles.OraclePriceData,
	slot uint64,
	dlobSide *common.Generator[types.IDLOBNode, int],
) *big.Int {
	runningSumQuote := utils.BN(0)
	runningSumBase := utils.BN(0)
	dlobSide.Each(func(side types.IDLOBNode, idx int) bool {
		price := side.GetPrice(oraclePriceData, slot) //side.order.quoteAssetAmount.div(side.order.baseAssetAmount);
		baseAmountRemaining := utils.BN(side.GetOrder().BaseAssetAmount - side.GetOrder().BaseAssetAmountFilled)
		if utils.AddX(runningSumBase, baseAmountRemaining).Cmp(baseAmountIn) > 0 {
			remainingBase := utils.SubX(baseAmountIn, runningSumBase)
			runningSumBase = utils.AddX(runningSumBase, remainingBase)
			runningSumQuote = utils.AddX(runningSumQuote, utils.MulX(remainingBase, price))
			return true
		} else {
			runningSumBase = utils.AddX(runningSumBase, baseAmountRemaining)
			runningSumQuote = utils.AddX(runningSumQuote, utils.MulX(baseAmountRemaining, price))
		}
		return false
	})

	return utils.DivX(
		utils.MulX(
			runningSumQuote,
			constants.QUOTE_PRECISION,
		),
		utils.MulX(constants.BASE_PRECISION, constants.PRICE_PRECISION),
	)
}

// EstimateFillWithExactBaseAmount
/**
 *
 * @param param.marketIndex the index of the market
 * @param param.marketType the type of the market
 * @param param.baseAmount the base amount in to estimate
 * @param param.orderDirection the direction of the trade
 * @param param.slot current slot for estimating dlob node price
 * @param param.oraclePriceData the oracle price data
 * @returns the estimated quote amount filled: QUOTE_PRECISION
 */
func (p *DLOB) EstimateFillWithExactBaseAmount(
	marketIndex uint16,
	marketType drift.MarketType,
	baseAmount *big.Int,
	orderDirection drift.PositionDirection,
	slot uint64,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	if orderDirection == drift.PositionDirection_Long {
		return p.EstimateFillExactBaseAmountInForSide(
			baseAmount,
			oraclePriceData,
			slot,
			p.GetRestingLimitAsks(
				marketIndex,
				slot,
				marketType,
				oraclePriceData,
				nil,
			),
		)
	} else if orderDirection == drift.PositionDirection_Short {
		return p.EstimateFillExactBaseAmountInForSide(
			baseAmount,
			oraclePriceData,
			slot,
			p.GetRestingLimitBids(
				marketIndex,
				slot,
				marketType,
				oraclePriceData,
				nil,
			),
		)
	}
	return nil
}

func (p *DLOB) GetBestMakers(
	marketIndex uint16,
	marketType drift.MarketType,
	direction drift.PositionDirection,
	slot uint64,
	oraclePriaceData *oracles.OraclePriceData,
	numMakers int,
) []solana.PublicKey {
	makers := make(map[string]solana.PublicKey)
	generator := utils.TTM[*common.Generator[types.IDLOBNode, int]](
		direction == drift.PositionDirection_Long,
		func() *common.Generator[types.IDLOBNode, int] {
			return p.GetRestingLimitBids(
				marketIndex,
				slot,
				marketType,
				oraclePriaceData,
				nil,
			)
		},
		func() *common.Generator[types.IDLOBNode, int] {
			return p.GetRestingLimitAsks(
				marketIndex,
				slot,
				marketType,
				oraclePriaceData,
				nil,
			)
		},
	)
	generator.Each(func(node types.IDLOBNode, idx int) bool {
		_, exists := makers[node.GetUserAccount()]
		if !exists {
			makers[node.GetUserAccount()] = solana.MPK(node.GetUserAccount())
		}
		if len(makers) == numMakers {
			return true
		}
		return false
	})
	return utils.MapValues(makers)
}
