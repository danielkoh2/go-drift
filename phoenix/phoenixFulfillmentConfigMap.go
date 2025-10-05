package phoenix

import (
	"driftgo/drift/types"
	"driftgo/lib/drift"
	"github.com/gagliardetto/solana-go"
)

type PhoenixFulfillmentConfigMap struct {
	driftClient      types.IDriftClient
	configAccountMap map[uint16]*drift.PhoenixV1FulfillmentConfig
}

func NewPhoenixFulfillmentConfigMap(driftClient types.IDriftClient) *PhoenixFulfillmentConfigMap {
	return &PhoenixFulfillmentConfigMap{
		driftClient:      driftClient,
		configAccountMap: make(map[uint16]*drift.PhoenixV1FulfillmentConfig),
	}
}

func (p *PhoenixFulfillmentConfigMap) Add(
	marketIndex uint16,
	phoenixMarketAddress solana.PublicKey,
) {
	account := p.driftClient.GetPhoenixV1FulfillmentConfig(
		phoenixMarketAddress,
	)
	p.configAccountMap[marketIndex] = account
}

func (p *PhoenixFulfillmentConfigMap) Get(
	marketIndex uint16,
) *drift.PhoenixV1FulfillmentConfig {
	return p.configAccountMap[marketIndex]
}
