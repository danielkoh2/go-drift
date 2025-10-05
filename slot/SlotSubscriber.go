package slot

import (
	"context"
	go_drift "driftgo"
	"driftgo/anchor/types"
	"driftgo/lib/event"
	"driftgo/lib/geyser"
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	utils2 "driftgo/utils"
	"fmt"
	"github.com/gagliardetto/solana-go/rpc"
	"sync"
	"time"
)

type SlotSubscriberConfig struct {
	ConnectionId   string
	ResubTimeoutMs *int64
}

type SlotSubscriber struct {
	currentSlot  uint64
	program      types.IProgram
	connectionId string
	subscription *geyser.Subscription
	eventEmitter *event.EventEmitter

	// Reconnection
	resubTimeoutMs  int64
	isUnsubscribing bool
	receivingData   bool
	mxState         *sync.RWMutex
	cancel          func()
}

func CreateSlotSubscriber(
	program types.IProgram,
	config SlotSubscriberConfig,
) *SlotSubscriber {
	p := &SlotSubscriber{
		program:        program,
		connectionId:   config.ConnectionId,
		eventEmitter:   go_drift.EventEmitter(),
		resubTimeoutMs: utils2.TTM[int64](config.ResubTimeoutMs == nil, int64(10000), func() int64 { return max(10000, *config.ResubTimeoutMs) }),
		mxState:        new(sync.RWMutex),
	}
	return p
}

func (p *SlotSubscriber) Subscribe() {
	if p.subscription != nil {
		return
	}

	slot, err := p.program.GetProvider().GetConnection(p.connectionId).GetSlot(context.TODO(), rpc.CommitmentConfirmed)
	if err == nil {
		p.currentSlot = slot
	}

	subscription, err := p.program.GetProvider().GetGeyserConnection(p.connectionId).Subscription()
	if err != nil {
		panic(err.Error())
	}
	err = subscription.Request(geyser.NewSubscriptionRequestBuilder().Slots().Build())
	if err != nil {
		return
	}
	p.subscription = subscription
	lastReceived := time.Now().UnixMilli()
	subscription.Subscribe(&geyser.SubscribeEvents{
		Update: func(data interface{}) {
			lastReceived = time.Now().UnixMilli()
			//defer p.mxState.Unlock()
			//p.mxState.Lock()
			slotInfo := data.(*solana_geyser_grpc.SubscribeUpdate).GetSlot()
			if slotInfo == nil {
				return
			}
			//fmt.Printf("SlotSubscriber new update : %v\n", data)
			if p.currentSlot <= 0 || p.currentSlot < slotInfo.Slot {
				p.currentSlot = slotInfo.Slot
				p.eventEmitter.Emit("newSlot", p.currentSlot)
			}
		},
		Error: func(err error) {
			fmt.Println("SlotSubscriber error : %v", err)
		},
		Eof: func() {
			fmt.Println("SlotSubscriber eof")
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go func(ctx context.Context) {
		ticker := time.NewTicker(500 * time.Millisecond)
		for {

			select {
			case <-ticker.C:
				if !p.isUnsubscribing {
					if time.Now().UnixMilli()-lastReceived > p.resubTimeoutMs {
						p.setTimeout()
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}(ctx)
}

func (p *SlotSubscriber) GetSlot() uint64 {
	defer p.mxState.RUnlock()
	p.mxState.RLock()

	return p.currentSlot
}

func (p *SlotSubscriber) setTimeout() {
	if p.isUnsubscribing {
		return
	}
	p.Unsubscribe()
	time.Sleep(500 * time.Millisecond)
	p.Subscribe()
}

func (p *SlotSubscriber) Unsubscribe() {
	if p.subscription == nil {
		return
	}
	fmt.Println("Slot Unsubscribing")
	p.isUnsubscribing = true
	p.subscription.Unsubscribe()
	p.subscription = nil
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.isUnsubscribing = false
}
