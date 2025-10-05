package phoenix

import (
	"context"
	"driftgo/accounts"
	"driftgo/anchor/types"
	"driftgo/common"
	"driftgo/constants"
	dloblib "driftgo/dlob/types"
	"driftgo/lib/geyser"
	"driftgo/lib/phoenix"
	"driftgo/lib/phoenix/utils"
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	utils2 "driftgo/utils"
	"fmt"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"math"
	"math/big"
)

type PhoenixContainer struct {
	dloblib.L2OrderBookGenerator
	marketAddress solana.PublicKey
	market        *phoenix.Market
	subscriber    *PhoenixMultiSubscriber
}

func CreatePhoenixContainer(marketAddress solana.PublicKey, subscriber *PhoenixMultiSubscriber) *PhoenixContainer {
	p := &PhoenixContainer{
		marketAddress: marketAddress,
		market:        nil,
		subscriber:    subscriber,
	}
	p.GetL2Bids = func() *common.Generator[*dloblib.L2Level, int] {
		return p.GetL2Levels("bids")
	}
	p.GetL2Asks = func() *common.Generator[*dloblib.L2Level, int] {
		return p.GetL2Levels("asks")
	}
	return p
}

func (p *PhoenixContainer) reload(buffer []byte) {
	p.market.Reload(buffer)
}

func (p *PhoenixContainer) GetBestBid() *big.Int {
	ladder := utils.GetMarketUiLadder(
		p.market,
		utils.DEFAULT_L2_LADDER_DEPTH,
		p.subscriber.lastSlot,
		p.subscriber.lastUnixTimestamp,
	)
	if len(ladder.Bids) == 0 {
		return nil
	}
	bestBid := ladder.Bids[0]
	return utils2.BN(bestBid.Price * constants.PRICE_PRECISION.Uint64())
}

func (p *PhoenixContainer) GetBestAsk() *big.Int {
	ladder := utils.GetMarketUiLadder(
		p.market,
		utils.DEFAULT_L2_LADDER_DEPTH,
		p.subscriber.lastSlot,
		p.subscriber.lastUnixTimestamp,
	)
	if len(ladder.Asks) == 0 {
		return nil
	}
	bestAsk := ladder.Asks[0]
	return utils2.BN(bestAsk.Price * constants.PRICE_PRECISION.Uint64())
}

func (p *PhoenixContainer) GetL2Levels(side string) *common.Generator[*dloblib.L2Level, int] {
	basePrecision := uint64(math.Pow(
		10,
		float64(p.market.Data.Header.BaseParams.Decimals),
	))

	pricePrecision := constants.PRICE_PRECISION.Uint64()
	ladder := utils.GetMarketUiLadder(p.market, 20, p.subscriber.lastSlot, p.subscriber.lastUnixTimestamp)

	var levels *[]phoenix.UiLadderLevel
	levels = utils2.TT(side == "bids", &ladder.Bids, &ladder.Asks)
	return common.NewGenerator(func(yield common.YieldFn[*dloblib.L2Level, int]) {
		if levels == nil {
			return
		}
		for i := 0; i < len(*levels); i++ {
			price := (*levels)[i].Price
			quantity := (*levels)[i].Quantity
			size := quantity * basePrecision
			updatedPrice := price * pricePrecision
			yield(&dloblib.L2Level{
				Price: utils2.BN(updatedPrice),
				Size:  utils2.BN(size),
				Sources: map[dloblib.LiquiditySource]*big.Int{
					dloblib.LiquiditySourcePhoenix: utils2.BN(size),
				},
			}, i)
		}
	})
}

type PhoenixMultiSubscriber struct {
	program           types.IProgram
	connectionId      string
	connection        *rpc.Client
	geyserConnection  *geyser.Connection
	programId         solana.PublicKey
	containers        map[string]*PhoenixContainer
	accountLoader     *accounts.BulkAccountLoader
	subscribed        bool
	subscription      *geyser.Subscription
	lastSlot          uint64
	lastUnixTimestamp int64
}

func CreatePhoenixMultiSubscriber(config PhoenixMarketSubscriberConfig) *PhoenixMultiSubscriber {
	return &PhoenixMultiSubscriber{
		program:           config.Program,
		connectionId:      config.ConnectionId,
		connection:        config.Program.GetProvider().GetConnection(config.ConnectionId),
		geyserConnection:  config.Program.GetProvider().GetGeyserConnection(config.ConnectionId),
		programId:         config.ProgramId,
		containers:        make(map[string]*PhoenixContainer),
		accountLoader:     nil,
		subscribed:        false,
		subscription:      nil,
		lastSlot:          0,
		lastUnixTimestamp: 0,
	}
}

func (p *PhoenixMultiSubscriber) AddMarket(marketAddress solana.PublicKey) {
	marketAddessKey := marketAddress.String()
	_, exists := p.containers[marketAddessKey]
	if exists {
		return
	}
	p.containers[marketAddessKey] = CreatePhoenixContainer(marketAddress, p)
}

func (p *PhoenixMultiSubscriber) loadClock() {
	var clock phoenix.ClockData
	accountInfo, err := p.connection.GetAccountInfo(context.TODO(), solana.SysVarClockPubkey)
	if err == nil {
		err = bin.NewBinDecoder(accountInfo.GetBinary()).Decode(&clock)
	}
	if err == nil {
		p.lastUnixTimestamp = clock.UnixTimestamp
	}
}

func (p *PhoenixMultiSubscriber) loadMarkets() {
	marketAddresses := utils2.ValuesFunc[string, solana.PublicKey](utils2.MapKeys(p.containers), func(key string) solana.PublicKey {
		return solana.MPK(key)
	})
	accountInfos, err := p.connection.GetMultipleAccounts(context.TODO(), marketAddresses...)
	if err != nil {
		return
	}

	for idx, accountInfo := range accountInfos.Value {
		marketAddress := marketAddresses[idx]
		p.containers[marketAddress.String()].market = phoenix.LoadMarketFromBuffer(marketAddress, accountInfo.Data.GetBinary())
	}

}

func (p *PhoenixMultiSubscriber) Subscribe() {
	if p.subscribed {
		return
	}
	p.loadClock()
	p.loadMarkets()
	subscription, err := p.geyserConnection.Subscription()
	if err != nil {
		return
	}
	accountKeys := []string{solana.SysVarClockPubkey.String()}
	accountKeys = append(accountKeys, utils2.MapKeys(p.containers)...)
	err = subscription.
		Request(
			geyser.
				NewSubscriptionRequestBuilder().
				Accounts(
					accountKeys...,
				).
				Build(),
		)
	if err != nil {
		return
	}
	subscription.Subscribe(&geyser.SubscribeEvents{
		Update: func(data interface{}) {
			account := data.(*solana_geyser_grpc.SubscribeUpdate).GetAccount()
			if account == nil {
				return
			}
			pubkey := solana.PublicKeyFromBytes(account.GetAccount().GetPubkey())
			if pubkey.Equals(solana.SysVarClockPubkey) {
				p.lastSlot = account.GetSlot()
				var clockData phoenix.ClockData
				err = bin.NewBinDecoder(account.GetAccount().GetData()).Decode(&clockData)
				if err == nil {
					p.lastUnixTimestamp = clockData.UnixTimestamp
				} else {
					fmt.Println("Failed to reload clock data")
				}
			} else {
				container, exists := p.containers[pubkey.String()]
				if exists {
					container.reload(account.GetAccount().GetData())
				}
			}
		},
		Error: nil,
		Eof:   nil,
	})
	p.subscription = subscription
	p.subscribed = true
}

func (p *PhoenixMultiSubscriber) Get(marketAddress solana.PublicKey) *PhoenixContainer {
	container, exists := p.containers[marketAddress.String()]
	if !exists {
		return nil
	}
	return container
}

func (p *PhoenixMultiSubscriber) Unsubscribe() {
	if !p.subscribed {
		return
	}
	if p.subscription != nil {
		p.subscription.Unsubscribe()
	}
	p.subscription = nil
	p.subscribed = false
}
