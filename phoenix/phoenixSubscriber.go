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
	"math"
	"math/big"
)

type PhoenixMarketSubscriberConfig struct {
	Program       types.IProgram
	ConnectionId  string
	ProgramId     solana.PublicKey
	MarketAddress solana.PublicKey
}

type PhoenixSubscriber struct {
	dloblib.L2OrderBookGenerator
	program       types.IProgram
	connectionId  string
	programId     solana.PublicKey
	marketAddress solana.PublicKey
	accountLoader *accounts.BulkAccountLoader
	market        *phoenix.Market

	subscribed        bool
	subscription      *geyser.Subscription
	lastSlot          uint64
	lastUnixTimestamp int64
}

func CreatePhoenixSubscriber(config PhoenixMarketSubscriberConfig) *PhoenixSubscriber {
	p := &PhoenixSubscriber{
		program:           config.Program,
		connectionId:      config.ConnectionId,
		programId:         config.ProgramId,
		marketAddress:     config.MarketAddress,
		accountLoader:     nil,
		market:            nil,
		subscribed:        false,
		lastSlot:          0,
		lastUnixTimestamp: 0,
	}
	p.GetL2Bids = func() *common.Generator[*dloblib.L2Level, int] {
		return p.GetL2Levels("bids")
	}
	p.GetL2Asks = func() *common.Generator[*dloblib.L2Level, int] {
		return p.GetL2Levels("asks")
	}
	return p
}

func (p *PhoenixSubscriber) Subscribe() {
	if p.subscribed {
		return
	}

	p.market = phoenix.LoadMarketFromAddress(
		p.program.GetProvider().GetConnection(),
		p.marketAddress,
	)
	if p.market == nil {
		return
	}
	var clock phoenix.ClockData
	accountInfo, err := p.program.GetProvider().GetConnection().GetAccountInfo(context.TODO(), solana.SysVarClockPubkey)
	if err == nil {
		err = bin.NewBinDecoder(accountInfo.GetBinary()).Decode(&clock)
	}
	if err == nil {
		p.lastUnixTimestamp = clock.UnixTimestamp
	}
	subscription, err := p.program.GetProvider().GetGeyserConnection().Subscription()
	if err != nil {
		return
	}
	err = subscription.
		Request(
			geyser.
				NewSubscriptionRequestBuilder().
				Accounts(
					solana.SysVarClockPubkey.String(),
					p.marketAddress.String(),
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
			if pubkey.Equals(p.marketAddress) {
				p.market = p.market.Reload(account.GetAccount().GetData())
			} else if pubkey.Equals(solana.SysVarClockPubkey) {
				p.lastSlot = account.GetSlot()
				var clockData phoenix.ClockData
				err = bin.NewBinDecoder(account.GetAccount().GetData()).Decode(&clockData)
				if err == nil {
					p.lastUnixTimestamp = clockData.UnixTimestamp
				} else {
					fmt.Println("Failed to reload clock data")
				}
			}
		},
		Error: nil,
		Eof:   nil,
	})
	p.subscription = subscription
	p.subscribed = true
}

func (p *PhoenixSubscriber) GetBestBid() *big.Int {
	ladder := utils.GetMarketUiLadder(
		p.market,
		utils.DEFAULT_L2_LADDER_DEPTH,
		p.lastSlot,
		p.lastUnixTimestamp,
	)
	if len(ladder.Bids) == 0 {
		return nil
	}
	bestBid := ladder.Bids[0]
	return utils2.BN(bestBid.Price * constants.PRICE_PRECISION.Uint64())
}

func (p *PhoenixSubscriber) GetBestAsk() *big.Int {
	ladder := utils.GetMarketUiLadder(
		p.market,
		utils.DEFAULT_L2_LADDER_DEPTH,
		p.lastSlot,
		p.lastUnixTimestamp,
	)
	if len(ladder.Asks) == 0 {
		return nil
	}
	bestAsk := ladder.Asks[0]
	return utils2.BN(bestAsk.Price * constants.PRICE_PRECISION.Uint64())
}

func (p *PhoenixSubscriber) GetL2Levels(side string) *common.Generator[*dloblib.L2Level, int] {
	basePrecision := uint64(math.Pow(
		10,
		float64(p.market.Data.Header.BaseParams.Decimals),
	))

	pricePrecision := constants.PRICE_PRECISION.Uint64()
	ladder := utils.GetMarketUiLadder(p.market, 20, p.lastSlot, p.lastUnixTimestamp)

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

func (p *PhoenixSubscriber) Unsubscribe() {
	if !p.subscribed {
		return
	}
	if p.subscription != nil {
		p.subscription.Unsubscribe()
	}
	p.subscription = nil
	p.subscribed = false
}
