package serum

import (
	"context"
	"driftgo/anchor/types"
	"driftgo/common"
	"driftgo/constants"
	dloblib "driftgo/dlob/types"
	"driftgo/lib/geyser"
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	"driftgo/lib/serum"
	utils2 "driftgo/utils"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"math"
	"math/big"
)

type SerumContainer struct {
	dloblib.L2OrderBookGenerator
	marketAddress solana.PublicKey
	market        *serum.Market
	asks          *serum.Orderbook
	lastAsksSlot  uint64
	bids          *serum.Orderbook
	lastBidsSlot  uint64

	subscriber *SerumMultiSubscriber
}

func CreateSerumContainer(marketAddress solana.PublicKey, subscriber *SerumMultiSubscriber) *SerumContainer {
	p := &SerumContainer{
		marketAddress: marketAddress,
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

func (p *SerumContainer) GetBestBid() *big.Int {
	if p.bids == nil {
		return nil
	}
	levels := p.bids.GetL2(1)
	if len(levels) > 0 {
		return utils2.MulX(levels[0].Price, constants.PRICE_PRECISION)
	}
	return nil
}

func (p *SerumContainer) GetBestAsk() *big.Int {
	if p.asks == nil {
		return nil
	}
	levels := p.asks.GetL2(1)
	if len(levels) > 0 {
		return utils2.MulX(levels[0].Price, constants.PRICE_PRECISION)
	}
	return nil
}

func (p *SerumContainer) GetL2Levels(side string) *common.Generator[*dloblib.L2Level, int] {
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

type SerumMultiSubscriber struct {
	program          types.IProgram
	connectionId     string
	connection       *rpc.Client
	geyserConnection *geyser.Connection
	programId        solana.PublicKey
	containers       map[string]*SerumContainer
	//accountLoader     *accounts.BulkAccountLoader
	subscribed   bool
	subscription *geyser.Subscription
}

func CreateSerumMultiSubscriber(config SerumMarketSubscriberConfig) *SerumMultiSubscriber {
	return &SerumMultiSubscriber{
		program:          config.Program,
		connectionId:     config.ConnectionId,
		connection:       config.Program.GetProvider().GetConnection(config.ConnectionId),
		geyserConnection: config.Program.GetProvider().GetGeyserConnection(config.ConnectionId),
		programId:        config.ProgramId,
		containers:       make(map[string]*SerumContainer),
		subscribed:       false,
		subscription:     nil,
	}
}
func (p *SerumMultiSubscriber) AddMarket(marketAddress solana.PublicKey) {
	marketAddessKey := marketAddress.String()
	_, exists := p.containers[marketAddessKey]
	if exists {
		return
	}
	p.containers[marketAddessKey] = CreateSerumContainer(marketAddress, p)
}

func (p *SerumMultiSubscriber) loadMarkets() {
	marketAddresses := utils2.ValuesFunc[string, solana.PublicKey](utils2.MapKeys(p.containers), func(key string) solana.PublicKey {
		return solana.MPK(key)
	})
	accountInfos, err := p.connection.GetMultipleAccounts(context.TODO(), marketAddresses...)
	if err != nil {
		return
	}

	for idx, accountInfo := range accountInfos.Value {
		marketAddress := marketAddresses[idx]
		p.containers[marketAddress.String()].market = serum.LoadMarketFromBuffer(marketAddress, p.programId, accountInfo.Data.GetBinary())
	}

}

func (p *SerumMultiSubscriber) loadAsksAndBids() {
	var keys []solana.PublicKey
	askAddressMap := make(map[string]string)
	bidAddressMap := make(map[string]string)
	for marketAddress, container := range p.containers {
		if container.market == nil {
			continue
		}
		askAddressMap[container.market.Data.Asks.String()] = marketAddress
		bidAddressMap[container.market.Data.Bids.String()] = marketAddress
		keys = append(keys, container.market.Data.Asks, container.market.Data.Bids)
	}

	accountInfos, err := p.connection.GetMultipleAccounts(context.TODO(), keys...)
	if err != nil {
		return
	}

	for idx, accountInfo := range accountInfos.Value {
		address := keys[idx]
		askMarketAddress, askExists := askAddressMap[address.String()]
		if askExists {
			orderbook := p.containers[askMarketAddress].market.LoadOrderbookFromBuffer(accountInfo.Data.GetBinary())
			if orderbook != nil {
				p.containers[askMarketAddress].asks = orderbook
				//fmt.Printf("Serum asks (%s) succeed to load \n", askMarketAddress)
			} else {
				fmt.Printf("Serum asks (%s) failed to load \n", askMarketAddress)
			}
		}
		bidMarketAddress, bidExists := askAddressMap[address.String()]
		if bidExists {
			orderbook := p.containers[bidMarketAddress].market.LoadOrderbookFromBuffer(accountInfo.Data.GetBinary())
			if orderbook != nil {
				p.containers[bidMarketAddress].bids = orderbook
				fmt.Printf("Serum bids (%s) succeed to load \n", bidMarketAddress)
			} else {
				fmt.Printf("Serum bids (%s) failed to load \n", bidMarketAddress)
			}
		}
	}

}

func (p *SerumMultiSubscriber) Subscribe() {
	if p.subscribed {
		return
	}
	p.loadMarkets()
	p.loadAsksAndBids()
	subscription, err := p.geyserConnection.Subscription()
	if err != nil {
		return
	}
	var accountKeys []string
	askAddressMap := make(map[string]string)
	bidAddressMap := make(map[string]string)
	//accountKeys = append(accountKeys, utils2.MapKeys(p.containers)...)
	for marketAddress, container := range p.containers {
		if container.market == nil {
			continue
		}
		askAddressMap[container.market.Data.Asks.String()] = marketAddress
		bidAddressMap[container.market.Data.Bids.String()] = marketAddress
		accountKeys = append(accountKeys, container.market.Data.Asks.String(), container.market.Data.Bids.String())
	}
	err = subscription.Request(geyser.
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
			address := solana.PublicKeyFromBytes(account.GetAccount().GetPubkey())
			askMarketAddress, askExists := askAddressMap[address.String()]
			if askExists {
				p.containers[askMarketAddress].asks = p.containers[askMarketAddress].market.LoadOrderbookFromBuffer(account.GetAccount().GetData())
			}
			bidMarketAddress, bidExists := askAddressMap[address.String()]
			if bidExists {
				p.containers[bidMarketAddress].bids = p.containers[bidMarketAddress].market.LoadOrderbookFromBuffer(account.GetAccount().GetData())
			}
		},
		Error: nil,
		Eof:   nil,
	})
	p.subscription = subscription
	p.subscribed = true
}

func (p *SerumMultiSubscriber) Get(marketAddress solana.PublicKey) *SerumContainer {
	container, exists := p.containers[marketAddress.String()]
	if !exists {
		return nil
	}
	return container
}

func (p *SerumMultiSubscriber) Unsubscribe() {
	if !p.subscribed {
		return
	}
	if p.subscription != nil {
		p.subscription.Unsubscribe()
	}
	p.subscription = nil
	p.subscribed = false
}
