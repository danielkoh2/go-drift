package jitter

import (
	go_drift "driftgo"
	"driftgo/auctionSubscriber"
	"driftgo/blockhashSubscriber"
	"driftgo/constants"
	"driftgo/drift/types"
	types3 "driftgo/jit_proxy/jitter/types"
	types2 "driftgo/jit_proxy/types"
	"driftgo/lib/drift"
	"driftgo/lib/jit_proxy"
	"driftgo/math"
	oracles "driftgo/oracles/types"
	"driftgo/userMap"
	"driftgo/utils"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/gagliardetto/solana-go"
)

type AuctionAndOrderDetails struct {
	SlotsTilCross     uint64
	WillCross         bool
	Bid               int64
	Ask               int64
	AuctionStartPrice int64
	AuctionEndPrice   int64
	StepSize          int64
	OraclePrice       *oracles.OraclePriceData
}

type JitterSniper struct {
	BaseJitter
	chainSubscriber *blockhashSubscriber.BlockHashSubscriber
}

func CreateJitterSniper(
	auctionSubscriber *auctionSubscriber.AuctionSubscriber,
	chainSubscriber *blockhashSubscriber.BlockHashSubscriber,
	jitProxyClient types2.IJitProxyClient,
	driftClient types.IDriftClient,
	userStatsMap *userMap.UserStatsMap,
) *JitterSniper {
	return &JitterSniper{
		chainSubscriber: chainSubscriber,
		BaseJitter: BaseJitter{
			AuctionSubscriber: auctionSubscriber,
			DriftClient:       driftClient,
			jitProxyClient:    jitProxyClient,
			UserStatsMap:      userStatsMap,
			PerpParams:        make(map[uint16]*types3.JitParams),
			SpotParams:        make(map[uint16]*types3.JitParams),
			SeenOrders:        make(map[string]bool),
			OnGoingAuctions:   make(map[string]bool),
			mxState:           new(sync.RWMutex),
			mxWorker:          new(sync.RWMutex),
		}}
}

func (p *JitterSniper) CreateTryFill(
	taker *drift.User,
	takerKey solana.PublicKey,
	takerStatsKey solana.PublicKey,
	order *drift.Order,
	orderSignature string,
	onComplete func(),
) {
	params := p.GetPerpParams(order.MarketIndex)
	if params == nil {
		//p.DeleteOnGoingAuction(orderSignature)
		onComplete()
		return
	}

	takerStats := p.UserStatsMap.MustGet(
		taker.Authority.String(),
	)
	referrerInfo := takerStats.GetReferrerInfo()

	auctionAndOrderDetails := p.GetAuctionAndOrderDetails(order)

	// don't increase risk if we're past max positions
	if order.MarketType == drift.MarketType_Perp {
		currPerpPos := p.DriftClient.GetUser(nil, nil).GetPerpPosition(order.MarketIndex)
		if currPerpPos == nil {
			currPerpPos = p.DriftClient.GetUser(nil, nil).GetEmptyPosition(order.MarketIndex)
		}
		if currPerpPos.BaseAssetAmount > 0 && order.Direction == drift.PositionDirection_Short {
			if currPerpPos.BaseAssetAmount <= params.MinPosition.Int64() {
				fmt.Printf("Order would increase existing short (mkt %v-%d) too much", order.MarketType, order.MarketIndex)
				//p.DeleteOnGoingAuction(orderSignature)
				onComplete()
				return
			}
		} else if currPerpPos.BaseAssetAmount > 0 && order.Direction == drift.PositionDirection_Long {
			if currPerpPos.BaseAssetAmount >= params.MaxPosition.Int64() {
				fmt.Printf("Order would increase existing long (mkt %v-%d) too much", order.MarketType, order.MarketIndex)
				//p.DeleteOnGoingAuction(orderSignature)
				onComplete()
				return
			}
		}
	}

	fmt.Printf("Taker wants to %v, order slot is %d,\nMy market: %d@%d,\nAuction: %d -> %d, step size %d,\nCurrent slot: %d, Order slot: %d,\nWill cross?: %v,\nSlots to wait: %d. Target slot = %d\n",
		order.Direction, order.Slot,
		auctionAndOrderDetails.Bid,
		auctionAndOrderDetails.Ask,
		auctionAndOrderDetails.AuctionStartPrice,
		auctionAndOrderDetails.AuctionEndPrice,
		auctionAndOrderDetails.StepSize,
		p.chainSubscriber.GetSlot(),
		order.Slot,
		auctionAndOrderDetails.WillCross,
		auctionAndOrderDetails.SlotsTilCross,
		order.Slot+auctionAndOrderDetails.SlotsTilCross,
	)
	mutex := new(sync.RWMutex)

	targetSlot := utils.TT(
		auctionAndOrderDetails.WillCross,
		order.Slot+auctionAndOrderDetails.SlotsTilCross,
		order.Slot+uint64(order.AuctionDuration)+1,
	)
	currentDetails := auctionAndOrderDetails
	willCross := auctionAndOrderDetails.WillCross

	var eventName string
	if order.MarketType == drift.MarketType_Perp {
		eventName = "perpMarketAccountUpdate"
	} else {
		eventName = "spotMarketAccountUpdate"
	}

	var marketHandler, slotHandler string
	stopHandlers := func() {
		if marketHandler != "" {
			p.DriftClient.GetEventEmitter().Off(eventName, marketHandler)
		}
		if slotHandler != "" {
			p.chainSubscriber.GetEventEmitter().Off("newSlot", slotHandler)
		}
	}

	detectForSlotOrCrossOrExpiry := func(first bool) (int64, *AuctionAndOrderDetails) {
		currentSlot := p.chainSubscriber.GetSlot()
		if first {
			if currentSlot > targetSlot {
				slot := utils.TT(willCross, int64(currentSlot), -1)
				return slot, currentDetails
			}
		}
		if currentSlot >= targetSlot {
			slot := utils.TT(willCross, int64(currentSlot), -1)
			return slot, currentDetails
		}

		return -1, nil
	}
	var tryFill func(*types2.JitTxParams, *go_drift.TxParams, int)
	tryFill = func(params *types2.JitTxParams, txParams *go_drift.TxParams, retryCount int) {
		txSig, err := p.jitProxyClient.Jit(params, txParams)
		if err == nil {
			fmt.Printf("Filled %s txSig %s\n", orderSignature, txSig.TxSig.String())
			p.addWorkerPool(3000, onComplete)
			//time.Sleep(3 * time.Second)
			//break
			return
		} else {
			fmt.Printf("Failed to send tx for %s\n", orderSignature)
			errMsg := err.Error()
			if strings.Contains(errMsg, "0x1770") || strings.Contains(errMsg, "0x1771") {
				fmt.Println("Order does not cross params yet, retrying")
			} else if strings.Contains(errMsg, "0x1779") {
				fmt.Println("Order could not fill")
			} else if strings.Contains(errMsg, "0x1793") {
				fmt.Println("Oracle invalid, retrying")
			} else {
				p.addWorkerPool(3000, onComplete)
				//time.Sleep(3 * time.Second)
				//break
				return
			}
		}
		//time.Sleep(2 * time.Second)
		if retryCount+1 < 10 {
			p.addWorkerPool(2000, func() {
				tryFill(params, txParams, retryCount+1)
			})
		} else {
			p.addWorkerPool(2000, onComplete)
		}
	}
	processFill := func(slot uint64, updatedDetails *AuctionAndOrderDetails) {
		jitParams := utils.TTM[*types3.JitParams](
			order.MarketType == drift.MarketType_Perp,
			func() *types3.JitParams {
				return p.GetPerpParams(order.MarketIndex)
			},
			func() *types3.JitParams {
				return p.GetSpotParams(order.MarketIndex)
			},
		)
		bid := utils.TTM[int64](
			*jitParams.PriceType == jit_proxy.PriceTypeOracle,
			func() int64 {
				return math.ConvertToNumber(utils.AddX(auctionAndOrderDetails.OraclePrice.Price, jitParams.Bid))
			},
			func() int64 {
				return math.ConvertToNumber(jitParams.Bid)
			},
		)
		ask := utils.TTM[int64](
			*jitParams.PriceType == jit_proxy.PriceTypeOracle,
			func() int64 {
				return math.ConvertToNumber(utils.AddX(auctionAndOrderDetails.OraclePrice.Price, jitParams.Ask))
			},
			func() int64 {
				return math.ConvertToNumber(jitParams.Ask)
			},
		)
		auctionPrice := math.ConvertToNumber(
			math.GetAuctionPrice(order, uint64(slot), updatedDetails.OraclePrice.Price),
		)

		fmt.Printf("Expected auction price: %d\nActual auction price: %d\n-----------------\nLooking for slot %d\nGot slot %d\n",
			auctionAndOrderDetails.AuctionStartPrice+int64(auctionAndOrderDetails.SlotsTilCross)*auctionAndOrderDetails.StepSize,
			auctionPrice,
			order.Slot+auctionAndOrderDetails.SlotsTilCross,
			slot,
		)

		fmt.Printf("Trying to fill %s with:\nmarket: %d@%d\nauction price: %d\nsubmitting %d@%d\n",
			orderSignature,
			bid,
			ask,
			auctionPrice,
			math.ConvertToNumber(jitParams.Bid),
			math.ConvertToNumber(jitParams.Ask),
		)
		tryFill(&types2.JitTxParams{
			TakerKey:      takerKey,
			TakerStatsKey: takerStatsKey,
			Taker:         taker,
			TakerOrderId:  order.OrderId,
			MaxPosition:   jitParams.MaxPosition,
			MinPosition:   jitParams.MinPosition,
			Bid:           jitParams.Bid,
			Ask:           jitParams.Ask,
			PostOnly:      utils.TT(jitParams.PostOnlyParams == nil, utils.NewPtr(jit_proxy.PostOnlyParamMustPostOnly), jitParams.PostOnlyParams),
			PriceType:     jitParams.PriceType,
			ReferrerInfo:  referrerInfo,
			SubAccountId:  jitParams.SubAccountId,
		}, &go_drift.TxParams{
			BaseTxParams: go_drift.BaseTxParams{
				ComputeUnits:      p.ComputeUnits,
				ComputeUnitsPrice: p.ComputeUnitsPrice,
			},
		}, 0)
	}
	process := func(first bool) bool {
		mutex.RLock()
		slot, updatedDetails := detectForSlotOrCrossOrExpiry(first)
		mutex.RUnlock()
		if slot == -1 {
			fmt.Println("Auction expired without crossing")
			onComplete()
			stopHandlers()
			return true
		} else if slot == -2 {
			// not detected then retry on new event
			return false
		}

		stopHandlers()
		processFill(uint64(slot), updatedDetails)
		return true

	}
	if process(true) {
		return
	}
	marketHandler = p.DriftClient.GetEventEmitter().On(eventName, func(data ...interface{}) {
		var marketIndex uint16
		if order.MarketType == drift.MarketType_Perp {
			marketIndex = data[1].(*drift.PerpMarket).MarketIndex
		} else {
			marketIndex = data[1].(*drift.SpotMarket).MarketIndex
		}
		if order.MarketIndex == marketIndex {
			mutex.Lock()
			currentDetails = p.GetAuctionAndOrderDetails(order)
			willCross = currentDetails.WillCross
			if willCross {
				targetSlot = order.Slot + currentDetails.SlotsTilCross
			}
			mutex.Unlock()
		}
	})
	slotHandler = p.chainSubscriber.GetEventEmitter().On("newSlot", func(data ...interface{}) {
		process(false)
	})
}

func (p *JitterSniper) GetAuctionAndOrderDetails(order *drift.Order) *AuctionAndOrderDetails {
	// Find number of slots until the order is expected to be in cross
	params := utils.TTM[*types3.JitParams](
		order.MarketType == drift.MarketType_Perp,
		func() *types3.JitParams {
			params, exists := p.PerpParams[order.MarketIndex]
			if exists {
				return params
			}
			return nil
		},
		func() *types3.JitParams {
			params, exists := p.SpotParams[order.MarketIndex]
			if exists {
				return params
			}
			return nil
		},
	)
	oraclePrice := utils.TTM[*oracles.OraclePriceData](
		order.MarketType == drift.MarketType_Perp,
		func() *oracles.OraclePriceData {
			return p.DriftClient.GetOracleDataForPerpMarket(order.MarketIndex)
		},
		func() *oracles.OraclePriceData {
			return p.DriftClient.GetOracleDataForSpotMarket(order.MarketIndex)
		},
	)

	makerOrderDir := utils.TT(order.Direction == drift.PositionDirection_Long, "sell", "buy")
	auctionStartPrice := math.ConvertToNumber(
		utils.TTM[*big.Int](
			order.OrderType == drift.OrderType_Oracle,
			math.GetAuctionPriceForOracleOffsetAuction(
				order,
				order.Slot,
				oraclePrice.Price,
			),
			utils.BN(order.AuctionStartPrice),
		),
		constants.PRICE_PRECISION,
	)
	auctionEndPrice := math.ConvertToNumber(
		utils.TTM[*big.Int](
			order.OrderType == drift.OrderType_Oracle,
			math.GetAuctionPriceForOracleOffsetAuction(
				order,
				order.Slot+uint64(order.AuctionDuration)-1,
				oraclePrice.Price,
			),
			utils.BN(order.AuctionEndPrice),
		),
		constants.PRICE_PRECISION,
	)
	bid := utils.TTM[int64](
		*params.PriceType == jit_proxy.PriceTypeOracle,
		func() int64 {
			return math.ConvertToNumber(
				utils.AddX(oraclePrice.Price, params.Bid),
				constants.PRICE_PRECISION,
			)
		},
		func() int64 {
			return math.ConvertToNumber(
				params.Bid,
				constants.PRICE_PRECISION,
			)
		},
	)
	ask := utils.TTM[int64](
		*params.PriceType == jit_proxy.PriceTypeOracle,
		func() int64 {
			return math.ConvertToNumber(
				utils.AddX(oraclePrice.Price, params.Ask),
				constants.PRICE_PRECISION,
			)
		},
		func() int64 {
			return math.ConvertToNumber(
				params.Ask,
				constants.PRICE_PRECISION,
			)
		},
	)

	slotsTillCross := uint64(0)
	willCross := false
	stepSize := (auctionEndPrice - auctionStartPrice) / int64(order.AuctionDuration-1)
	for ; slotsTillCross < uint64(order.AuctionDuration); slotsTillCross++ {
		if makerOrderDir == "buy" {
			if math.ConvertToNumber(math.GetAuctionPrice(order, order.Slot+slotsTillCross, oraclePrice.Price)) <= bid {
				willCross = true
				break
			}
		} else {
			if math.ConvertToNumber(math.GetAuctionPrice(order, order.Slot+slotsTillCross, oraclePrice.Price)) >= ask {
				willCross = true
				break
			}
		}
	}

	// if it doesnt cross during auction, check if limit price crosses
	if !willCross {
		slotAfterAuction := order.Slot + uint64(order.AuctionDuration) + 1
		limitPrice := math.GetLimitPrice(order, oraclePrice, slotAfterAuction, nil)
		if limitPrice == nil {
			willCross = true
			slotsTillCross = uint64(order.AuctionDuration) + 1
		} else {
			limitPriceNum := math.ConvertToNumber(limitPrice)
			if makerOrderDir == "buy" || limitPriceNum <= bid {
				willCross = true
				slotsTillCross = uint64(order.AuctionDuration) + 1
			} else if makerOrderDir == "sell" || limitPriceNum >= ask {
				willCross = true
				slotsTillCross = uint64(order.AuctionDuration) + 1
			}
		}
	}
	return &AuctionAndOrderDetails{
		SlotsTilCross:     slotsTillCross,
		WillCross:         willCross,
		Bid:               bid,
		Ask:               ask,
		AuctionStartPrice: auctionStartPrice,
		AuctionEndPrice:   auctionEndPrice,
		StepSize:          stepSize,
		OraclePrice:       oraclePrice,
	}
}
