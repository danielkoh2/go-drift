package jitter

import (
	"context"
	go_drift "driftgo"
	"driftgo/addresses"
	"driftgo/auctionSubscriber"
	"driftgo/drift/types"
	types3 "driftgo/jit_proxy/jitter/types"
	types2 "driftgo/jit_proxy/types"
	"driftgo/lib/drift"
	"driftgo/math"
	"driftgo/userMap"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
)

type PendingTask struct {
	timestamp int64
	callback  func()
}

type BaseJitter struct {
	types3.IBaseJitter
	AuctionSubscriber *auctionSubscriber.AuctionSubscriber
	DriftClient       types.IDriftClient
	jitProxyClient    types2.IJitProxyClient
	UserStatsMap      *userMap.UserStatsMap
	PerpParams        map[uint16]*types3.JitParams
	SpotParams        map[uint16]*types3.JitParams
	SeenOrders        map[string]bool
	OnGoingAuctions   map[string]bool
	UserFiller        types3.UserFilter
	ComputeUnits      uint64
	ComputeUnitsPrice uint64
	mxState           *sync.RWMutex

	pendingTasks []*PendingTask
	cancel       func()
	mxWorker     *sync.RWMutex
}

func (p *BaseJitter) Subscribe() {
	p.DriftClient.Subscribe()

	p.AuctionSubscriber.Subscribe()
	p.startIntervalLoop()

	go_drift.EventEmitter().On(
		"onAccountUpdate",
		func(data ...interface{}) {
			defer p.mxState.Unlock()
			p.mxState.Lock()
			taker := data[0].(*drift.User)
			takerKey := data[1].(solana.PublicKey)
			slot := data[2].(uint64)

			takerKeyString := takerKey.String()

			takerStatsKey := addresses.GetUserStatsAccountPublicKey(
				p.DriftClient.GetProgram().GetProgramId(),
				taker.Authority,
			)
			for idx := 0; idx < len(taker.Orders); idx++ {
				order := &taker.Orders[idx]
				if order.Status != drift.OrderStatus_Open {
					continue
				}
				if !math.HasAuctionPrice(order, slot) {
					continue
				}

				if p.UserFiller != nil {
					if p.UserFiller(taker, takerKeyString, order) {
						return
					}
				}

				orderSignature := p.GetOrderSignatures(
					takerKeyString,
					order.OrderId,
				)
				seenOrder, onGoingAuction := p.CheckOrder(orderSignature)
				if seenOrder {
					continue
				}
				p.SetSeenOrder(orderSignature)

				if onGoingAuction {
					continue
				}

				if order.MarketType == drift.MarketType_Perp {
					perpParams := p.GetPerpParams(order.MarketIndex)
					if perpParams == nil {
						return
					}

					perpMarketAccount := p.DriftClient.GetPerpMarketAccount(
						order.MarketIndex,
					)
					if order.BaseAssetAmount-order.BaseAssetAmountFilled <= perpMarketAccount.Amm.MinOrderSize {
						return
					}
					p.SetOnGoingAuction(orderSignature)
					p.CreateTryFill(
						taker,
						takerKey,
						takerStatsKey,
						order,
						orderSignature,
						func() {
							p.DeleteOnGoingAuction(orderSignature)
						},
					)
				} else {
					spotParams := p.GetSpotParams(order.MarketIndex)
					if spotParams == nil {
						return
					}

					spotMarketAccount := p.DriftClient.GetSpotMarketAccount(
						order.MarketIndex,
					)
					if order.BaseAssetAmount-order.BaseAssetAmountFilled <= spotMarketAccount.MinOrderSize {
						return
					}
					p.SetOnGoingAuction(orderSignature)
					p.CreateTryFill(
						taker,
						takerKey,
						takerStatsKey,
						order,
						orderSignature,
						func() {
							p.DeleteOnGoingAuction(orderSignature)
						},
					)
				}
			}
		},
	)
}

func (p *BaseJitter) Unsubscribe() {
	p.AuctionSubscriber.Unsubscribe()
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

func (p *BaseJitter) startIntervalLoop() {
	ctx, cancel := context.WithCancel(context.Background())
	go func(ctx context.Context) {
		ticker := time.NewTicker(time.Millisecond * 10)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.runWorkerPool()
			}
		}
	}(ctx)
	p.cancel = cancel
}

func (p *BaseJitter) addWorkerPool(timestamp int64, callback func()) {
	defer p.mxWorker.Unlock()
	p.mxWorker.Lock()
	p.pendingTasks = append(p.pendingTasks, &PendingTask{timestamp: time.Now().UnixMilli() + timestamp, callback: callback})
}

func (p *BaseJitter) runWorkerPool() {
	defer p.mxWorker.Unlock()
	p.mxWorker.Lock()
	p.pendingTasks = slices.DeleteFunc(p.pendingTasks, func(pendingTask *PendingTask) bool {
		if pendingTask.timestamp <= time.Now().UnixMilli() {
			go pendingTask.callback()
			return true
		}
		return false
	})
}

func (p *BaseJitter) CheckOrder(orderSignature string) (bool, bool) {
	defer p.mxState.RUnlock()
	p.mxState.RLock()
	_, seenOrder := p.SeenOrders[orderSignature]
	_, onGoingAuction := p.OnGoingAuctions[orderSignature]
	return seenOrder, onGoingAuction
}

func (p *BaseJitter) SetSeenOrder(orderSignature string) {
	defer p.mxState.Unlock()
	p.mxState.Lock()
	p.SeenOrders[orderSignature] = true
}

func (p *BaseJitter) SetOnGoingAuction(orderSignature string) {
	defer p.mxState.Unlock()
	p.mxState.Lock()
	p.OnGoingAuctions[orderSignature] = true
}

func (p *BaseJitter) DeleteOnGoingAuction(orderSignature string) {
	defer p.mxState.Unlock()
	p.mxState.Lock()
	delete(p.OnGoingAuctions, orderSignature)
	delete(p.SeenOrders, orderSignature)
}

func (p *BaseJitter) GetOrderSignatures(takerKey string, orderId uint32) string {
	return fmt.Sprintf("%s-%d", takerKey, orderId)
}

func (p *BaseJitter) UpdatePerpParams(marketIndex uint16, params *types3.JitParams) {
	defer p.mxState.Unlock()
	p.mxState.Lock()
	p.PerpParams[marketIndex] = params
}

func (p *BaseJitter) UpdateSpotParams(marketIndex uint16, params *types3.JitParams) {
	defer p.mxState.Unlock()
	p.mxState.Lock()
	p.SpotParams[marketIndex] = params
}

func (p *BaseJitter) GetPerpParams(marketIndex uint16) *types3.JitParams {
	defer p.mxState.RUnlock()
	p.mxState.RLock()
	params, exists := p.PerpParams[marketIndex]
	if !exists {
		return nil
	}
	return params
}

func (p *BaseJitter) GetSpotParams(marketIndex uint16) *types3.JitParams {
	defer p.mxState.RUnlock()
	p.mxState.RLock()
	params, exists := p.SpotParams[marketIndex]
	if !exists {
		return nil
	}
	return params

}
