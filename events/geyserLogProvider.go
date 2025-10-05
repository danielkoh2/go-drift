package events

import (
	"context"
	"driftgo/lib/geyser"
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"time"
)

type GeyserLogProvider struct {
	ILogProvider
	connection        *geyser.Connection
	address           solana.PublicKey
	commitment        rpc.CommitmentType
	resubTimeoutMs    *int64
	subscription      *geyser.Subscription
	isUnsubscribing   bool
	receivingData     bool
	reconnectAttempts int64
	callback          LogProviderCallback
	cancel            func()
}

func CreateGeyserLogProvider(
	connection *geyser.Connection,
	address solana.PublicKey,
	commitment rpc.CommitmentType,
	resubTimeoutMs *int64,
) *GeyserLogProvider {
	return &GeyserLogProvider{
		connection:     connection,
		address:        address,
		commitment:     commitment,
		resubTimeoutMs: resubTimeoutMs,
	}
}

func (p *GeyserLogProvider) IsSubscribed() bool {
	return p.subscription != nil
}

func (p *GeyserLogProvider) Subscribe(callback LogProviderCallback, skipHistory ...bool) bool {
	if p.subscription != nil {
		return true
	}
	p.callback = callback
	return p.setSubscription(callback)
}

func (p *GeyserLogProvider) setSubscription(callback LogProviderCallback) bool {
	subscription, err := p.connection.Subscription()
	if err != nil {
		return false
	}
	request := geyser.NewSubscriptionRequestBuilder().
		Transactions([]string{p.address.String()}, []string{}, []string{}, false, false).
		Build()
	err = subscription.Request(request)
	if err != nil {
		return false
	}
	var lastUpdated int64
	subscription.Subscribe(&geyser.SubscribeEvents{
		Update: func(data interface{}) {

			lastUpdated = time.Now().UnixMilli()
			updateData := data.(*solana_geyser_grpc.SubscribeUpdate)
			transaction := updateData.GetTransaction()
			if transaction == nil {
				return
			}
			p.callback(
				solana.SignatureFromBytes(transaction.GetTransaction().GetSignature()),
				transaction.GetSlot(),
				transaction.GetTransaction().GetMeta().GetLogMessages(),
				0,
			)
		},
		Error: nil,
		Eof:   nil,
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func(ctx context.Context) {
		ticker := time.NewTicker(time.Microsecond * 50)
		for {
			select {
			case <-ticker.C:
				if p.resubTimeoutMs != nil && lastUpdated > 0 && time.Now().UnixMilli()-lastUpdated > *p.resubTimeoutMs {
					if !p.isUnsubscribing {
						p.setTimeout()
					}
				}
			case <-ctx.Done():
				return
			}
		}

	}(ctx)
	p.subscription = subscription
	p.cancel = cancel
	return true
}

func (p *GeyserLogProvider) Unsubscribe(external ...bool) bool {
	p.isUnsubscribing = true
	if p.subscription != nil {
		p.subscription.Unsubscribe()
		p.subscription = nil
	}
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	return true
}

func (p *GeyserLogProvider) setTimeout() {
	p.Unsubscribe()
	p.Subscribe(p.callback)
}
