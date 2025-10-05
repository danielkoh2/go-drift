package priorityFee

import (
	"context"
	"driftgo/lib/drift"
	"driftgo/utils"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"sync"
	"time"
)

type DriftMarketInfo struct {
	MarketType  drift.MarketType
	MarketIndex uint16
}

type PriorityFeeSubscriber struct {
	connection            *rpc.Client
	frequencyMs           uint64
	addresses             solana.PublicKeySlice
	driftMarkets          []DriftMarketInfo
	customStrategy        IPriorityFeeStrategy
	averageStrategy       *AverageOverSlotsStrategy
	maxStrategy           *MaxOverSlotsStrategy
	priorityFeeMethod     PriorityFeeMethod
	lookbackDistance      uint64
	maxFeeMicroLamports   uint64
	priorityFeeMultiplier float64

	driftPriorityFeeEndpoint string

	latestPriorityFee        uint64
	lastCustomStrategyResult uint64
	lastAvgStrategyResult    uint64
	lastMaxStrategyResult    uint64
	lastSlotSeen             uint64
	wait                     sync.WaitGroup
	cancel                   func()
	percentile               uint
	callback                 func(*PriorityFeeSubscriber)
}

func CreatePriorityFeeSubscriber(config PriorityFeeSubscriberConfig) *PriorityFeeSubscriber {
	var customStrategy IPriorityFeeStrategy
	if config.CustomStrategy != nil {
		customStrategy = config.CustomStrategy
	} else {
		customStrategy = &AverageStrategy{}
	}
	var slotsToCheck = uint64(50)
	if config.SlotsToCheck > 0 {
		slotsToCheck = config.SlotsToCheck
	}
	subscriber := &PriorityFeeSubscriber{
		connection:            config.Connection,
		frequencyMs:           config.FrequencyMs,
		addresses:             config.Addresses,
		driftMarkets:          config.DriftMarkets,
		customStrategy:        customStrategy,
		lookbackDistance:      slotsToCheck,
		priorityFeeMultiplier: utils.TT(config.PriorityFeeMultiplier > 0.0, config.PriorityFeeMultiplier, 1.0),
		priorityFeeMethod:     config.PriorityFeeMethod,
		maxFeeMicroLamports:   config.MaxFeeMicroLamports,
		percentile:            config.Percentile,
		callback:              config.Callback,
	}
	if config.PriorityFeeMethod == PriorityFeeMethodDrift {
		subscriber.driftPriorityFeeEndpoint = config.DriftPriorityFeeEndpoint
	}
	if config.PriorityFeeMethod == PriorityFeeMethodSolana {
		if config.Connection == nil {
			panic("connection must be provided to use SOLANA priority fee API")
		}
	}
	return subscriber
}

func (p *PriorityFeeSubscriber) Subscribe() {
	p.load()
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go func(ctx context.Context) {
		p.wait.Add(1)
		ticker := time.NewTicker(time.Millisecond * time.Duration(p.frequencyMs))
		for {
			select {
			case <-ctx.Done():
				goto END
			case <-ticker.C:
				p.load()
			}
		}
	END:
		p.wait.Done()
	}(ctx)
}

func (p *PriorityFeeSubscriber) Unsubscribe() {
	if p.cancel != nil {
		p.wait.Add(1)
		p.cancel()
		p.cancel = nil
		p.wait.Wait()
	}
}

func (p *PriorityFeeSubscriber) loadForSolana() {
	samples := FetchSolanaPriorityFee(
		p.connection,
		p.lookbackDistance,
		p.addresses,
		p.percentile,
	)
	if len(samples) > 0 {
		p.latestPriorityFee = samples[0].PrioritizationFee
		p.lastSlotSeen = samples[0].Slot

		p.lastAvgStrategyResult = p.averageStrategy.Calculate(samples)
		p.lastMaxStrategyResult = p.maxStrategy.Calculate(samples)
		if p.customStrategy != nil {
			p.lastCustomStrategyResult = p.customStrategy.Calculate(samples)
		}
	}
}

func (p *PriorityFeeSubscriber) loadForDrift() {
	if len(p.driftMarkets) == 0 {
		return
	}

	var marketTypes []string
	var marketIndexs []uint16
	for _, marketInfo := range p.driftMarkets {
		marketTypes = append(marketTypes, marketInfo.MarketType.String())
		marketIndexs = append(marketIndexs, marketInfo.MarketIndex)
	}
	feeSamples := FetchDriftPriorityFee(
		p.driftPriorityFeeEndpoint,
		marketTypes,
		marketIndexs,
	)
	if len(feeSamples) > 0 {
		p.lastAvgStrategyResult = feeSamples[0].Medium
		p.lastMaxStrategyResult = feeSamples[0].UnsafeMax
		if p.customStrategy != nil {
			var samples []SolanaPriorityFeeResponse
			for _, s := range feeSamples {
				samples = append(samples, SolanaPriorityFeeResponse{Slot: 0, PrioritizationFee: s.Medium})
			}
			p.lastCustomStrategyResult = p.customStrategy.Calculate(samples)
		}
	}
}

func (p *PriorityFeeSubscriber) GetMaxPriorityFee() uint64 {
	return p.maxFeeMicroLamports
}

func (p *PriorityFeeSubscriber) GetCustomStrategyResult() uint64 {
	result := uint64(float64(p.lastCustomStrategyResult) * p.priorityFeeMultiplier)
	if p.maxFeeMicroLamports > 0 && result > p.maxFeeMicroLamports {
		return p.maxFeeMicroLamports
	}
	return result
}

func (p *PriorityFeeSubscriber) GetRawCustomStrategyResult() uint64 {
	return p.lastCustomStrategyResult
}

func (p *PriorityFeeSubscriber) GetAvgStrategyResult() uint64 {
	if p.maxFeeMicroLamports > 0 && p.lastAvgStrategyResult > p.maxFeeMicroLamports {
		return p.maxFeeMicroLamports
	}
	return p.lastAvgStrategyResult
}

func (p *PriorityFeeSubscriber) GetMaxStrategyResult() uint64 {
	if p.maxFeeMicroLamports > 0 && p.lastMaxStrategyResult > p.maxFeeMicroLamports {
		return p.maxFeeMicroLamports
	}
	return p.lastMaxStrategyResult
}

func (p *PriorityFeeSubscriber) load() {
	if p.priorityFeeMethod == PriorityFeeMethodSolana {
		p.loadForSolana()
	} else if p.priorityFeeMethod == PriorityFeeMethodDrift {
		p.loadForDrift()
	} else if p.priorityFeeMethod == PriorityFeeMethodHelius {
		panic("Helius load not implemented")
	} else {
		panic(fmt.Sprintf("%s load not implemented", p.priorityFeeMethod))
	}

	if p.callback != nil {
		(p.callback)(p)
	}
}

func (p *PriorityFeeSubscriber) UpdateAddresses(addresses []solana.PublicKey) {
	p.addresses = addresses
}
