package jitter

import (
	go_drift "driftgo"
	"driftgo/auctionSubscriber"
	"driftgo/drift/types"
	types3 "driftgo/jit_proxy/jitter/types"
	types2 "driftgo/jit_proxy/types"
	"driftgo/lib/drift"
	"driftgo/lib/jit_proxy"
	"driftgo/userMap"
	"driftgo/utils"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"strings"
	"sync"
)

type JitterShotgun struct {
	BaseJitter
}

func CreateJitterShotgun(
	driftClient types.IDriftClient,
	auctionSubscriber *auctionSubscriber.AuctionSubscriber,
	jitProxyClient types2.IJitProxyClient,
	userStatsMap *userMap.UserStatsMap,
) *JitterShotgun {
	return &JitterShotgun{
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
		}}
}

func (p *JitterShotgun) CreateTryFill(
	taker *drift.User,
	takerKey solana.PublicKey,
	takerStatsKey solana.PublicKey,
	order *drift.Order,
	orderSignature string,
	onComplete func(),
) {
	defer onComplete()
	takerStats := p.UserStatsMap.MustGet(
		taker.Authority.String(),
	)
	referrerInfo := takerStats.GetReferrerInfo()

	var tryFill func(int)
	tryFill = func(retry int) {
		params := p.GetPerpParams(order.MarketIndex)
		if params == nil {
			return
		}

		fmt.Printf("Trying to fill %s\n", orderSignature)
		txSig, err := p.jitProxyClient.Jit(&types2.JitTxParams{
			TakerKey:      takerKey,
			TakerStatsKey: takerStatsKey,
			Taker:         taker,
			TakerOrderId:  order.OrderId,
			MaxPosition:   params.MaxPosition,
			MinPosition:   params.MinPosition,
			Bid:           params.Bid,
			Ask:           params.Ask,
			PostOnly:      utils.TT(params.PostOnlyParams == nil, utils.NewPtr(jit_proxy.PostOnlyParamMustPostOnly), params.PostOnlyParams),
			PriceType:     params.PriceType,
			ReferrerInfo:  referrerInfo,
			SubAccountId:  params.SubAccountId,
		}, &go_drift.TxParams{
			BaseTxParams: go_drift.BaseTxParams{
				ComputeUnits:      p.ComputeUnits,
				ComputeUnitsPrice: p.ComputeUnitsPrice,
			},
		})
		if err != nil {
			fmt.Printf("Failed to send tx for %s\n", orderSignature)
			errMsg := err.Error()
			if strings.Contains(errMsg, "0x1770") || strings.Contains(errMsg, "0x1771") {
				fmt.Println("Order does not cross params yet, retrying")
			} else if strings.Contains(errMsg, "0x1779") {
				fmt.Println("Order could not fill")
			} else if strings.Contains(errMsg, "0x1793") {
				fmt.Println("Oracle invalid, retrying")
			} else {
				//time.Sleep(10 * time.Second)
				//break
				p.addWorkerPool(10_000, onComplete)
				return
			}
		} else {
			fmt.Printf("Successfully sent tx for %s txSig %s\n", orderSignature, txSig.TxSig.String())
			//time.Sleep(10 * time.Second)
			//break
			p.addWorkerPool(10_000, onComplete)
			return
		}

		if retry+1 < int(order.AuctionDuration) {
			p.addWorkerPool(10_000, func() {
				tryFill(retry + 1)
			})
		}
	}
	// assumes each preflight simulation takes ~1 slot
	tryFill(0)
}
