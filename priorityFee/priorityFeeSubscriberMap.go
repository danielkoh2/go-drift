package priorityFee

import (
	"context"
	"driftgo/utils"
	"time"
)

type PriorityFeeSubscriberMap struct {
	frequencyMs int64
	cancel      func()

	driftMarkets             []DriftMarketInfo
	driftPriorityFeeEndpoint string
	feesMap                  map[string]map[uint16]DriftPriorityFeeLevels
}

func CreatePriorityFeeSubscriberMap(config PriorityFeeSubscriberMapConfig) *PriorityFeeSubscriberMap {
	p := &PriorityFeeSubscriberMap{
		frequencyMs: utils.TTM[int64](config.FrequencyMs == nil, DEFAULT_PRIORITY_FEE_MAP_FREQUENCY_MS, func() int64 {
			return *config.FrequencyMs
		}),
		driftMarkets:             config.DriftMarkets,
		driftPriorityFeeEndpoint: config.DriftPriorityFeeEndpoint,
		feesMap:                  make(map[string]map[uint16]DriftPriorityFeeLevels),
	}
	p.feesMap["perp"] = make(map[uint16]DriftPriorityFeeLevels)
	p.feesMap["spot"] = make(map[uint16]DriftPriorityFeeLevels)
	return p
}

func (p *PriorityFeeSubscriberMap) updateFeesMap(response DriftPriorityFeeResponse) {
	for _, fee := range response {
		p.feesMap[fee.MarketType][fee.MarketIndex] = fee
	}
}

func (p *PriorityFeeSubscriberMap) Subscribe() {
	if p.cancel != nil {
		return
	}
	p.load()
	ctx, cancel := context.WithCancel(context.Background())
	go func(ctx context.Context) {
		ticker := time.NewTicker(100 * time.Millisecond)
		last := time.Now().UnixMilli()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if time.Now().UnixMilli()-last > p.frequencyMs {
					p.load()
					last = time.Now().UnixMilli()
				}
			}
		}
	}(ctx)
	p.cancel = cancel
}

func (p *PriorityFeeSubscriberMap) Unsubscribe() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

func (p *PriorityFeeSubscriberMap) load() {
	if p.driftMarkets == nil || len(p.driftMarkets) == 0 {
		return
	}

	fees := FetchDriftPriorityFee(
		p.driftPriorityFeeEndpoint,
		utils.ValuesFunc(p.driftMarkets, func(m DriftMarketInfo) string {
			return m.MarketType.String()
		}),
		utils.ValuesFunc(p.driftMarkets, func(m DriftMarketInfo) uint16 {
			return m.MarketIndex
		}),
	)
	p.updateFeesMap(fees)
}

func (p *PriorityFeeSubscriberMap) UpdateMarketTypeAndIndex(driftMarkets []DriftMarketInfo) {
	p.driftMarkets = driftMarkets
}

func (p *PriorityFeeSubscriberMap) GetPriorityFees(
	marketType string,
	marketIndex uint16,
) *DriftPriorityFeeLevels {
	v, exists := p.feesMap[marketType][marketIndex]
	if !exists {
		return nil
	}
	return &v
}
