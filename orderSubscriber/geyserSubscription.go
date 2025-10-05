package orderSubscriber

import (
	"context"
	go_drift "driftgo"
	"driftgo/accounts"
	"driftgo/lib/drift"
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"time"
)

type GeyserSubscription struct {
	orderSubscriber  *OrderSubscriber
	commitment       rpc.CommitmentType
	skipInitialLoad  *bool
	resubTimeoutMs   *int64
	resyncIntervalMs *int64
	cancel           func()
	subscriber       *accounts.GeyserProgramAccountSubscriber[drift.User]
	decoded          *bool
}

func CreateGeyserSubscription(
	orderSubscriber *OrderSubscriber,
	commitment rpc.CommitmentType,
	skipInitialLoad *bool,
	resubTimeoutMs *int64,
	resyncIntervalMs *int64,
	decoded *bool,
) *GeyserSubscription {
	subscription := &GeyserSubscription{
		orderSubscriber:  orderSubscriber,
		commitment:       commitment,
		skipInitialLoad:  skipInitialLoad,
		resubTimeoutMs:   resubTimeoutMs,
		resyncIntervalMs: resyncIntervalMs,
		decoded:          decoded,
	}

	return subscription
}

func (p *GeyserSubscription) Subscribe() {
	if p.subscriber != nil {
		return
	}

	p.subscriber = accounts.CreateGeyserProgramAccountSubscriber(
		"OrderSubscriber",
		"User",
		p.orderSubscriber.DriftClient.GetProgram(),
		p.orderSubscriber.DecodeFn,
		accounts.ProgramAccountSubscriberOptions{
			Filters:    []rpc.RPCFilter{go_drift.GetUserFilter(), go_drift.GetNonIdleUserFiler()},
			Commitment: &p.commitment,
		},
		p.resubTimeoutMs,
	)

	p.subscriber.Subscribe(func(
		accountId solana.PublicKey,
		account *drift.User,
		context rpc.Context,
		buffer []byte,
		accountInfo ...*solana_geyser_grpc.SubscribeUpdateAccountInfo,
	) {
		if p.decoded != nil && *p.decoded {
			p.orderSubscriber.TryUpdateUserAccount(
				accountId.String(),
				"decoded",
				account,
				context.Slot,
			)
		} else {
			p.orderSubscriber.TryUpdateUserAccount(
				accountId.String(),
				"buffer",
				account,
				context.Slot,
			)
		}
	})

	if p.resyncIntervalMs != nil && *p.resyncIntervalMs > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		p.cancel = cancel
		go func(ctx context.Context) {
			lastSynced := time.Now().UnixMilli()
			tickerTimeout := time.NewTicker(time.Millisecond * 100)
			for {
				<-tickerTimeout.C
				if time.Now().UnixMilli()-lastSynced > *p.resyncIntervalMs {
					lastSynced = time.Now().UnixMilli()
					p.orderSubscriber.Fetch()
				}
			}
		}(ctx)
	}

	if p.skipInitialLoad == nil || !*p.skipInitialLoad {
		p.orderSubscriber.Fetch()
	}
}

func (p *GeyserSubscription) Unsubscribe() {
	if p.subscriber == nil {
		return
	}
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.subscriber.Unsubscribe()
	p.subscriber = nil
}
