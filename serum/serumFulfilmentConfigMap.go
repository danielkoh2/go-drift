package serum

import (
	"driftgo/drift/types"
	"driftgo/lib/drift"
	"github.com/gagliardetto/solana-go"
)

type SerumFulfillmentConfigMap struct {
	driftClient      types.IDriftClient
	configAccountMap map[uint16]*drift.SerumV3FulfillmentConfig
}

func NewSerumFulfillmentConfigMap(driftClient types.IDriftClient) *SerumFulfillmentConfigMap {
	return &SerumFulfillmentConfigMap{
		driftClient:      driftClient,
		configAccountMap: make(map[uint16]*drift.SerumV3FulfillmentConfig),
	}
}

func (p *SerumFulfillmentConfigMap) Add(
	marketIndex uint16,
	serumMarketAddress solana.PublicKey,
) {
	account := p.driftClient.GetSerumV3FulfillmentConfig(
		serumMarketAddress,
	)
	p.configAccountMap[marketIndex] = account
}

func (p *SerumFulfillmentConfigMap) Get(marketIndex uint16) *drift.SerumV3FulfillmentConfig {
	account, exists := p.configAccountMap[marketIndex]
	if !exists {
		return nil
	}
	return account
}
