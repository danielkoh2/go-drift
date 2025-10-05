package accounts

import (
	go_drift "driftgo"
	"driftgo/addresses"
	"driftgo/anchor/types"
	utils2 "driftgo/config/utils"
	"driftgo/lib/drift"
	oracles "driftgo/oracles/types"
	"driftgo/utils"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/go-errors/errors"
	"sync"
)

type GeyserDriftClientAccountSubscriber struct {
	DriftClientAccountSubscriber
	program           types.IProgram
	commitment        *rpc.CommitmentType
	perpMarketIndexes []uint16
	spotMarketIndexes []uint16
	oracleInfos       []oracles.OracleInfo
	oracleClientCache *oracles.OracleClientCache

	resubTimeoutMs                 *int64
	shouldFindAllMarketsAndOracles bool

	stateAccountSubscriber IAccountSubscriber[drift.State]
	perpMarketSubscriber   IMultiAccountSubscriber[drift.PerpMarket]
	perpOracleMap          map[uint16]solana.PublicKey

	spotMarketSubscriber IMultiAccountSubscriber[drift.SpotMarket]
	spotOracleMap        map[uint16]solana.PublicKey

	oracleSubscriber IMultiAccountSubscriber[oracles.OraclePriceData]
	//oracleSubscribers map[string]IAccountSubscriber[oracles.OraclePriceData]
	oracleSourceMap map[string]drift.OracleSource
	isSubscribing   bool
}

func CreateGeyserDriftClientAccountSubscriber(
	program types.IProgram,
	perpMarketIndexes []uint16,
	spotMarketIndexes []uint16,
	oracleInfos []oracles.OracleInfo,
	shouldFindAllMarketsAndOracles bool,
	resubTimeoutMs *int64,
	commitment *rpc.CommitmentType,
) *GeyserDriftClientAccountSubscriber {

	subscriber := &GeyserDriftClientAccountSubscriber{
		DriftClientAccountSubscriber: DriftClientAccountSubscriber{
			EventEmitter: go_drift.EventEmitter(),
			IsSubscribed: false,
		},
		program:                        program,
		commitment:                     utils.TT(commitment != nil, commitment, &program.GetProvider().GetOpts().Commitment),
		perpMarketIndexes:              perpMarketIndexes,
		spotMarketIndexes:              spotMarketIndexes,
		oracleInfos:                    oracleInfos,
		shouldFindAllMarketsAndOracles: shouldFindAllMarketsAndOracles,
		oracleClientCache:              oracles.CreateOracleClientCache(),
		perpOracleMap:                  make(map[uint16]solana.PublicKey),
		spotOracleMap:                  make(map[uint16]solana.PublicKey),
		oracleSourceMap:                make(map[string]drift.OracleSource),
		resubTimeoutMs:                 resubTimeoutMs,
	}
	return subscriber
}

func (p *GeyserDriftClientAccountSubscriber) Subscribed() bool {
	return p.IsSubscribed
}

func (p *GeyserDriftClientAccountSubscriber) Subscribe() bool {
	if p.IsSubscribed {
		return true
	}

	if p.isSubscribing {
		return false
	}
	p.isSubscribing = true
	if p.shouldFindAllMarketsAndOracles {
		p.perpMarketIndexes, p.spotMarketIndexes, p.oracleInfos = utils2.FindAllMarketAndOracles(p.program)
	}

	statePublicKey := addresses.GetDriftStateAccountPublicKey(p.program.GetProgramId())

	// create and activate drift state account subscription
	p.stateAccountSubscriber = CreateGeyserAccountSubscriber[drift.State](
		"state",
		p.program,
		statePublicKey,
		nil,
		p.resubTimeoutMs,
		p.commitment,
	)

	p.stateAccountSubscriber.Subscribe(func(data *drift.State) {
		p.EventEmitter.Emit("stateAccountUpdate", data)
		p.EventEmitter.Emit("update", nil)
	})

	// subscribe to market accounts
	p.subscribeToPerpMarketAccounts()

	// subscribe to spot market accounts
	p.subscribeToSpotMarketAccounts()

	// subscribe to oracles
	p.subscribeToOracles()

	p.EventEmitter.Emit("update", nil)

	p.setPerpOracleMap()
	p.setSpotOracleMap()

	p.isSubscribing = false
	p.IsSubscribed = true
	return true
}

//func (p *GeyserDriftClientAccountSubscriber) subscribeToPerpMarketAccount(marketIndex uint16) bool {
//	perpMarketPublicKey := addresses.GetPerpMarketPublicKey(p.program.GetProgramId(), marketIndex)
//	subscriber := CreateGeyserAccountSubscriber[drift.PerpMarket](
//		"perpMarket",
//		p.program,
//		perpMarketPublicKey,
//		nil,
//		p.resubTimeoutMs,
//		p.commitment,
//	)
//	subscriber.Subscribe(func(data *drift.PerpMarket) {
//		p.EventEmitter.Emit("perpMarketAccountUpdate", perpMarketPublicKey, data)
//		p.EventEmitter.Emit("update", nil)
//	})
//	p.perpMarketAccountSubscribers[marketIndex] = subscriber
//	return true
//}

func (p *GeyserDriftClientAccountSubscriber) subscribeToPerpMarketAccounts() bool {
	pubkeys := utils.ValuesFunc[uint16, solana.PublicKey](p.perpMarketIndexes, func(marketIndex uint16) solana.PublicKey {
		return addresses.GetPerpMarketPublicKey(p.program.GetProgramId(), marketIndex)
	})

	subscriber := CreateGeyserMultiAccountSubscriber[drift.PerpMarket](
		"perpMarket",
		p.program,
		pubkeys,
		nil,
		p.resubTimeoutMs,
		p.commitment,
	)
	subscriber.Subscribe(func(pubkey string, data *drift.PerpMarket) {
		p.EventEmitter.Emit("perpMarketAccountUpdate", pubkey, data)
		p.EventEmitter.Emit("update", nil)
	})
	p.perpMarketSubscriber = subscriber
	//for _, marketIndex := range p.perpMarketIndexes {
	//	p.subscribeToPerpMarketAccount(marketIndex)
	//
	//}
	return true
}

//	func (p *GeyserDriftClientAccountSubscriber) subscribeToSpotMarketAccount(marketIndex uint16) bool {
//		spotMarketPublicKey := addresses.GetSpotMarketPublicKey(p.program.GetProgramId(), marketIndex)
//		subscriber := CreateGeyserAccountSubscriber[drift.SpotMarket](
//			"spotMarket",
//			p.program,
//			spotMarketPublicKey,
//			nil,
//			p.resubTimeoutMs,
//			p.commitment,
//		)
//		subscriber.Subscribe(func(data *drift.SpotMarket) {
//			p.EventEmitter.Emit("spotMarketAccountUpdate", spotMarketPublicKey, data)
//			p.EventEmitter.Emit("update", nil)
//		})
//		p.spotMarketAccountSubscribers[marketIndex] = subscriber
//		return true
//	}
func (p *GeyserDriftClientAccountSubscriber) subscribeToSpotMarketAccounts() bool {
	//for _, marketIndex := range p.spotMarketIndexes {
	//	p.subscribeToSpotMarketAccount(marketIndex)
	//}
	pubkeys := utils.ValuesFunc[uint16, solana.PublicKey](p.spotMarketIndexes, func(marketIndex uint16) solana.PublicKey {
		return addresses.GetSpotMarketPublicKey(p.program.GetProgramId(), marketIndex)
	})
	subscriber := CreateGeyserMultiAccountSubscriber[drift.SpotMarket](
		"perpMarket",
		p.program,
		pubkeys,
		nil,
		p.resubTimeoutMs,
		p.commitment,
	)
	subscriber.Subscribe(func(pubkey string, data *drift.SpotMarket) {
		p.EventEmitter.Emit("spotMarketAccountUpdate", pubkey, data)
		p.EventEmitter.Emit("update", nil)
	})
	p.spotMarketSubscriber = subscriber
	return true
}

func (p *GeyserDriftClientAccountSubscriber) subscribeToOracles() bool {
	//for _, oracleInfo := range p.oracleInfos {
	//	p.subscribeToOracle(oracleInfo)
	//}
	p.oracleSourceMap = make(map[string]drift.OracleSource)
	pubkeys := utils.ValuesFunc[oracles.OracleInfo, solana.PublicKey](p.oracleInfos, func(oracleInfo oracles.OracleInfo) solana.PublicKey {
		p.oracleSourceMap[oracleInfo.PublicKey.String()] = oracleInfo.Source
		return oracleInfo.PublicKey
	})
	subscriber := CreateGeyserMultiAccountSubscriber[oracles.OraclePriceData](
		"oracle",
		p.program,
		pubkeys,
		func(key solana.PublicKey, data []byte) *oracles.OraclePriceData {
			defer func() {
				if e := recover(); e != nil {
					fmt.Println("Decode oracle data error: "+key.String(), e)
					fmt.Println(errors.Wrap(e, 2).ErrorStack())
				}
			}()
			var client oracles.IOracleClient
			fmt.Println("77777777777777OraclePrice : source=", p.oracleSourceMap[key.String()], "pubkey=", key.String())
			source, exists := p.oracleSourceMap[key.String()]
			if exists {
				client = p.oracleClientCache.Get(source, p.program.GetProvider().GetConnection(), p.program)
			} else {
				client = p.oracleClientCache.Get(drift.OracleSource_Pyth, p.program.GetProvider().GetConnection(), p.program)
			}
			oracleData := client.GetOraclePriceDataFromBuffer(data)
			//if oracleData.Price == nil || oracleData.Price.Cmp(constants.ZERO) == 0 {
			//	fmt.Println("OraclePrice is zero : source=", source, ",pubkey=", key.String())
			//}
			//fmt.Println("OraclePrice : source=", source, ",pubkey=", key.String(), ",price=", oracleData.Price.String())
			return oracleData
		},
		p.resubTimeoutMs,
		p.commitment,
	)
	//fmt.Println("Drift subscription : oracle starting")
	subscriber.Subscribe(func(pubkey string, data *oracles.OraclePriceData) {
		p.EventEmitter.Emit("oraclePriceUpdate", pubkey, data)
		p.EventEmitter.Emit("update", nil)
	})
	//spew.Dump(subscriber.GetDataAndSlots())
	p.oracleSubscriber = subscriber
	return true
}

//func (p *GeyserDriftClientAccountSubscriber) subscribeToOracle(oracleInfo oracles.OracleInfo) bool {
//	client := p.oracleClientCache.Get(oracleInfo.Source, p.program.GetProvider().GetConnection())
//	subscriber := CreateGeyserAccountSubscriber[oracles.OraclePriceData](
//		"oracle",
//		p.program,
//		oracleInfo.PublicKey,
//		func(data []byte) *oracles.OraclePriceData {
//			return client.GetOraclePriceDataFromBuffer(data)
//		},
//		p.resubTimeoutMs,
//		p.commitment,
//	)
//	p.oracleSubscribers[oracleInfo.PublicKey.String()] = subscriber
//	subscriber.Subscribe(func(data *oracles.OraclePriceData) {
//		p.EventEmitter.Emit("oraclePriceUpdate", oracleInfo.PublicKey, data)
//		p.EventEmitter.Emit("update", nil)
//	})
//	return true
//}

func (p *GeyserDriftClientAccountSubscriber) UnsubscribeFromMarketAccounts() {
	p.perpMarketSubscriber.Unsubscribe()
	//for _, subscriber := range p.perpMarketAccountSubscribers {
	//	subscriber.Unsubscribe()
	//}
}

func (p *GeyserDriftClientAccountSubscriber) UnsubscribeFromSpotMarketAccounts() {
	p.spotMarketSubscriber.Unsubscribe()
	//for _, subscriber := range p.spotMarketAccountSubscribers {
	//	subscriber.Unsubscribe()
	//}
}

func (p *GeyserDriftClientAccountSubscriber) UnsubscribeFromOracles() {
	p.oracleSubscriber.Unsubscribe()
	//for _, subscriber := range p.oracleSubscribers {
	//	subscriber.Unsubscribe()
	//}
}

func (p *GeyserDriftClientAccountSubscriber) Fetch() {
	if !p.IsSubscribed {
		return
	}

	var wait sync.WaitGroup
	wait.Add(3)
	go func() {
		p.stateAccountSubscriber.Fetch()
		wait.Done()
	}()

	go func() {
		p.perpMarketSubscriber.Fetch()
		//for _, subscriber := range p.perpMarketAccountSubscribers {
		//	subscriber.Fetch()
		//}
		wait.Done()
	}()

	go func() {
		p.spotMarketSubscriber.Fetch()
		//for _, subscriber := range p.spotMarketAccountSubscribers {
		//	subscriber.Fetch()
		//}
		wait.Done()
	}()
	wait.Wait()
}

func (p *GeyserDriftClientAccountSubscriber) Unsubscribe() {
	if !p.IsSubscribed {
		return
	}
	p.stateAccountSubscriber.Unsubscribe()
	p.UnsubscribeFromMarketAccounts()
	p.UnsubscribeFromSpotMarketAccounts()
	p.UnsubscribeFromOracles()

	p.IsSubscribed = false
}

func (p *GeyserDriftClientAccountSubscriber) AddSpotMarket(marketIndex uint16) bool {
	subscriptionSuccess := p.spotMarketSubscriber.Add(addresses.GetSpotMarketPublicKey(p.program.GetProgramId(), marketIndex))
	if subscriptionSuccess {
		p.setSpotOracleMap()
	}
	return subscriptionSuccess
	//_, exists := p.spotMarketAccountSubscribers[marketIndex]
	//if exists {
	//	return true
	//}
	//return p.subscribeToSpotMarketAccount(marketIndex)
}

func (p *GeyserDriftClientAccountSubscriber) AddPerpMarket(marketIndex uint16) bool {

	subscriptionSuccess := p.perpMarketSubscriber.Add(addresses.GetPerpMarketPublicKey(p.program.GetProgramId(), marketIndex))
	if subscriptionSuccess {
		p.setPerpOracleMap()
	}
	return subscriptionSuccess
	//_, exists := p.perpMarketAccountSubscribers[marketIndex]
	//if exists {
	//	return true
	//}
	//return p.subscribeToPerpMarketAccount(marketIndex)
}

func (p *GeyserDriftClientAccountSubscriber) AddOracle(oracleInfo oracles.OracleInfo) bool {

	_, exists := p.oracleSourceMap[oracleInfo.PublicKey.String()]
	if exists {
		return false
	}
	p.oracleSourceMap[oracleInfo.PublicKey.String()] = oracleInfo.Source
	return p.oracleSubscriber.Add(oracleInfo.PublicKey)
}

func (p *GeyserDriftClientAccountSubscriber) setPerpOracleMap() {
	perpMarkets := p.GetMarketAccountsAndSlots()
	for _, perpMarket := range perpMarkets {
		if perpMarket == nil {
			continue
		}
		perpMarketAccount := perpMarket.Data
		perpMarketIndex := perpMarketAccount.MarketIndex
		oracle := perpMarketAccount.Amm.Oracle

		oracleSubscribed := p.oracleSubscriber.GetDataAndSlot(oracle.String()) != nil
		if !oracleSubscribed {
			p.AddOracle(oracles.OracleInfo{
				PublicKey: oracle,
				Source:    perpMarketAccount.Amm.OracleSource,
			})
		}
		p.perpOracleMap[perpMarketIndex] = oracle
	}
}

func (p *GeyserDriftClientAccountSubscriber) setSpotOracleMap() {
	spotMarkets := p.GetSpotMarketAccountsAndSlots()
	for _, spotMarket := range spotMarkets {
		if spotMarket == nil {
			continue
		}
		spotMarketAccount := spotMarket.Data
		spotMarketIndex := spotMarketAccount.MarketIndex
		oracle := spotMarketAccount.Oracle

		oracleSubscribed := p.oracleSubscriber.GetDataAndSlot(oracle.String()) != nil
		if !oracleSubscribed {
			p.AddOracle(oracles.OracleInfo{
				PublicKey: oracle,
				Source:    spotMarketAccount.OracleSource,
			})
		}
		p.spotOracleMap[spotMarketIndex] = oracle
	}

}

func (p *GeyserDriftClientAccountSubscriber) AssetIsSubscribed() {
	if !p.IsSubscribed {
		panic("You must call `subscribe` before using this function")
	}
}

func (p *GeyserDriftClientAccountSubscriber) GetStateAccountAndSlot() *DataAndSlot[*drift.State] {
	p.AssetIsSubscribed()
	return p.stateAccountSubscriber.GetDataAndSlot()
}

func (p *GeyserDriftClientAccountSubscriber) GetMarketAccountAndSlot(
	marketIndex uint16,
) *DataAndSlot[*drift.PerpMarket] {
	p.AssetIsSubscribed()
	key := addresses.GetPerpMarketPublicKey(p.program.GetProgramId(), marketIndex)
	return p.perpMarketSubscriber.GetDataAndSlot(key.String())
	//return p.perpMarketAccountSubscribers[marketIndex].GetDataAndSlot()
}

func (p *GeyserDriftClientAccountSubscriber) GetMarketAccountsAndSlots() []*DataAndSlot[*drift.PerpMarket] {
	return utils.MapValues(p.perpMarketSubscriber.GetDataAndSlots())
	//var datas []*DataAndSlot[*drift.PerpMarket]
	//for _, subscriber := range p.perpMarketAccountSubscribers {
	//	datas = append(datas, subscriber.GetDataAndSlot())
	//}
	//return datas
}

func (p *GeyserDriftClientAccountSubscriber) GetSpotMarketAccountAndSlot(
	marketIndex uint16,
) *DataAndSlot[*drift.SpotMarket] {
	p.AssetIsSubscribed()
	key := addresses.GetSpotMarketPublicKey(p.program.GetProgramId(), marketIndex)
	return p.spotMarketSubscriber.GetDataAndSlot(key.String())
	//return p.spotMarketAccountSubscribers[marketIndex].GetDataAndSlot()
}

func (p *GeyserDriftClientAccountSubscriber) GetSpotMarketAccountsAndSlots() []*DataAndSlot[*drift.SpotMarket] {
	return utils.MapValues(p.spotMarketSubscriber.GetDataAndSlots())
	//var datas []*DataAndSlot[*drift.SpotMarket]
	//for _, subscriber := range p.spotMarketAccountSubscribers {
	//	datas = append(datas, subscriber.GetDataAndSlot())
	//}
	//return datas
}

func (p *GeyserDriftClientAccountSubscriber) GetOraclePriceDataAndSlot(
	oraclePublicKey solana.PublicKey,
) *DataAndSlot[*oracles.OraclePriceData] {
	p.AssetIsSubscribed()
	if oraclePublicKey.Equals(solana.PublicKey{}) {
		return &DataAndSlot[*oracles.OraclePriceData]{
			Data: &oracles.QUOTE_ORACLE_PRICE_DATA,
			Slot: 0,
		}
	}
	return p.oracleSubscriber.GetDataAndSlot(oraclePublicKey.String())
	//return p.oracleSubscribers[oraclePublicKey.String()].GetDataAndSlot()
}

func (p *GeyserDriftClientAccountSubscriber) GetOraclePriceDataAndSlotForPerpMarket(
	marketIndex uint16,
) *DataAndSlot[*oracles.OraclePriceData] {
	perpMarketAccount := p.GetMarketAccountAndSlot(marketIndex)
	oracle, exists := p.perpOracleMap[marketIndex]
	if perpMarketAccount == nil || !exists {
		return nil
	}
	if !perpMarketAccount.Data.Amm.Oracle.Equals(oracle) {
		// If the oracle has changed, we need to update the oracle map in background
		p.setPerpOracleMap()
	}
	return p.oracleSubscriber.GetDataAndSlot(oracle.String())
}
func (p *GeyserDriftClientAccountSubscriber) GetOraclePriceDataAndSlotForSpotMarket(
	marketIndex uint16,
) *DataAndSlot[*oracles.OraclePriceData] {
	spotMarketAccount := p.GetSpotMarketAccountAndSlot(marketIndex)
	oracle, exists := p.spotOracleMap[marketIndex]
	if spotMarketAccount == nil || !exists {
		return nil
	}
	if !spotMarketAccount.Data.Oracle.Equals(oracle) {
		// If the oracle has changed, we need to update the oracle map in background
		p.setSpotOracleMap()
	}
	return p.oracleSubscriber.GetDataAndSlot(oracle.String())

}
