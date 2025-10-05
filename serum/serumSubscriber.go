package serum

import (
	"driftgo/anchor/types"
	"driftgo/common"
	"driftgo/constants"
	dloblib "driftgo/dlob/types"
	"driftgo/lib/geyser"
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	"driftgo/lib/serum"
	utils2 "driftgo/utils"
	"fmt"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/go-errors/errors"
	"math"
	"math/big"
	"slices"
)

type SerumSubscriber struct {
	dloblib.L2OrderBookGenerator
	program       types.IProgram
	connectionId  string
	programId     solana.PublicKey
	marketAddress solana.PublicKey
	market        *serum.Market

	subscribed bool

	asksAddress  solana.PublicKey
	asks         *serum.Orderbook
	lastAsksSlot uint64

	bidsAddress  solana.PublicKey
	bids         *serum.Orderbook
	lastBidsSlot uint64

	subscription *geyser.Subscription
}

func CreateSerumSubscriber(config SerumMarketSubscriberConfig) *SerumSubscriber {
	p := &SerumSubscriber{
		program:       config.Program,
		connectionId:  config.ConnectionId,
		programId:     config.ProgramId,
		marketAddress: config.MarketAddress,
	}
	p.GetL2Bids = func() *common.Generator[*dloblib.L2Level, int] {
		return p.GetL2Levels("bids")
	}
	p.GetL2Asks = func() *common.Generator[*dloblib.L2Level, int] {
		return p.GetL2Levels("asks")
	}
	return p
}

func (p *SerumSubscriber) Subscribe() {
	if p.subscribed {
		return
	}
	rpcConnection := p.program.GetProvider().GetConnection(p.connectionId)
	p.market = serum.LoadMarketFromAddress(
		rpcConnection,
		p.marketAddress,
		p.programId,
	)
	if p.market == nil {
		return
	}

	p.asksAddress = p.market.Data.Asks
	p.asks = p.market.LoadAsks(rpcConnection)

	p.bidsAddress = p.market.Data.Bids
	p.bids = p.market.LoadBids(rpcConnection)

	geyserConnection := p.program.GetProvider().GetGeyserConnection(p.connectionId)
	subscription, _ := geyserConnection.Subscription()
	err := subscription.Request(
		geyser.NewSubscriptionRequestBuilder().
			Accounts(
				p.asksAddress.String(),
				p.bidsAddress.String(),
			).Build())
	if err != nil {
		return
	}
	p.subscription = subscription
	p.subscription.Subscribe(&geyser.SubscribeEvents{
		Update: func(data interface{}) {
			update := data.(*solana_geyser_grpc.SubscribeUpdate)
			accountInfo := update.GetAccount()
			slot := accountInfo.GetSlot()
			pubkey := solana.PublicKeyFromBytes(accountInfo.GetAccount().GetPubkey())
			var orderBook serum.Orderbook
			if !slices.Contains([]string{p.asksAddress.String(), p.bidsAddress.String()}, pubkey.String()) {
				return
			}
			err = bin.NewBinDecoder(update.GetAccount().GetAccount().GetData()).Decode(&orderBook)
			if err != nil {
				//fmt.Printf("Asks : %s, Bids : %s\n", p.asksAddress.String(), p.bidsAddress.String())
				fmt.Println("Serum orderbook failed: pubkey=", pubkey, ",err=", errors.Wrap(err, 2).ErrorStack())
				//panic("serum order book failed")
				return
			} else {
				//fmt.Println("Serum orderbook success: pubkey=", pubkey)
			}
			if pubkey.Equals(p.asksAddress) {
				p.lastAsksSlot = slot
				p.asks = &orderBook
			} else if pubkey.Equals(p.bidsAddress) {
				p.lastBidsSlot = slot
				p.bids = &orderBook
			}
		},
		Error: nil,
		Eof:   nil,
	})
	p.subscribed = true
}

func (p *SerumSubscriber) GetBestBid() *big.Int {
	if p.bids == nil {
		return nil
	}
	levels := p.bids.GetL2(1)
	if len(levels) > 0 {
		return utils2.MulX(levels[0].Price, constants.PRICE_PRECISION)
	}
	return nil
}

func (p *SerumSubscriber) GetBestAsk() *big.Int {
	if p.asks == nil {
		return nil
	}
	levels := p.asks.GetL2(1)
	if len(levels) > 0 {
		return utils2.MulX(levels[0].Price, constants.PRICE_PRECISION)
	}
	return nil
}

func (p *SerumSubscriber) GetL2Levels(side string) *common.Generator[*dloblib.L2Level, int] {
	basePrecision := uint64(math.Pow(
		10,
		float64(p.market.BaseSplTokenMultiplier()),
	))

	pricePrecision := constants.PRICE_PRECISION.Uint64()

	return common.NewGenerator(func(yield common.YieldFn[*dloblib.L2Level, int]) {
		orderBook := utils2.TT(side == "bids", p.bids, p.asks)
		if orderBook == nil {
			return
		}
		var idx = 0
		_ = orderBook.Items(side == "bids", func(item *serum.SlabLeafNode) error {
			price := item.GetPrice().Uint64() * pricePrecision
			quantity := utils2.BN(uint64(item.Quantity) * basePrecision)
			yield(&dloblib.L2Level{
				Price: utils2.BN(price),
				Size:  quantity,
				Sources: map[dloblib.LiquiditySource]*big.Int{
					dloblib.LiquiditySourceSerum: quantity,
				},
			}, idx)
			idx++

			return nil
		})
	})
}
func (p *SerumSubscriber) Unsubscribe() {
	if !p.subscribed {
		return
	}
	if p.subscription != nil {
		p.subscription.Cancel()
		p.subscription = nil
	}
	p.subscribed = false
}
