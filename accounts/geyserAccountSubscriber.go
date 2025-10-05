package accounts

import (
	"context"
	"driftgo/anchor/types"
	"driftgo/lib/geyser"
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	"driftgo/utils"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"time"
)

type GeyserAccountSubscriber[T any] struct {
	AccountSubscriber[T]
	dataAndSlot      *DataAndSlot[*T]
	bufferAndSlot    *BufferAndSlot
	accountName      string
	program          types.IProgram
	accountPublicKey solana.PublicKey
	decodeBufferFn   func([]byte) *T
	onChange         func(*T)
	resubTimeoutMs   int64
	lastReceived     int64
	commitment       rpc.CommitmentType
	isUnsubscribing  bool

	subscription  *geyser.Subscription
	receivingData bool
	cancel        func()
}

func CreateGeyserAccountSubscriber[T any](
	accountName string,
	program types.IProgram,
	accountPublicKey solana.PublicKey,
	decodeBuffer func([]byte) *T,
	resubTimeoutMs *int64,
	commitment *rpc.CommitmentType,
) IAccountSubscriber[T] {
	subscriber := &GeyserAccountSubscriber[T]{
		accountName:      accountName,
		program:          program,
		accountPublicKey: accountPublicKey,
		decodeBufferFn:   decodeBuffer,
		resubTimeoutMs:   utils.TTM[int64](resubTimeoutMs == nil, int64(10000), func() int64 { return max(10000, *resubTimeoutMs) }),
		lastReceived:     0,
		commitment:       utils.TTM[rpc.CommitmentType](commitment != nil, func() rpc.CommitmentType { return *commitment }, program.GetProvider().GetOpts().Commitment),
		receivingData:    false,
	}

	return subscriber
}

func (p *GeyserAccountSubscriber[T]) Subscribed() bool {
	return p.subscription != nil
}

func (p *GeyserAccountSubscriber[T]) Subscribe(onChange func(data *T)) {
	if p.isUnsubscribing {
		return
	}
	p.onChange = onChange
	if p.dataAndSlot == nil {
		p.Fetch()
	}
	subscription, err := p.program.GetProvider().GetGeyserConnection().Subscription()
	if err != nil {
		return
	}
	err = subscription.Request(geyser.NewSubscriptionRequestBuilder().Accounts(p.accountPublicKey.String()).Build())
	if err != nil {
		return
	}
	p.subscription = subscription
	p.subscription.Subscribe(&geyser.SubscribeEvents{
		Update: func(data interface{}) {
			p.lastReceived = time.Now().UnixMilli()
			p.handleRpcResponse(data.(*solana_geyser_grpc.SubscribeUpdate))
		},
		Error: func(err error) {
		},
		Eof: func() {
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go func(ctx context.Context) {
		tickerTimeout := time.NewTicker(time.Millisecond * 100)
		for {
			select {
			case <-tickerTimeout.C:
				if p.lastReceived > 0 && time.Now().UnixMilli()-p.lastReceived > p.resubTimeoutMs {
					p.setTimeout()
				}
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

}

func (p *GeyserAccountSubscriber[T]) SetData(data *T, slot ...uint64) {
	newSlot := uint64(0)
	if len(slot) > 0 {
		newSlot = slot[0]
	}
	if p.dataAndSlot != nil && p.dataAndSlot.Slot > newSlot {
		return
	}
	if p.dataAndSlot == nil {
		p.dataAndSlot = &DataAndSlot[*T]{
			Data: data,
			Slot: newSlot,
		}
	} else {
		p.dataAndSlot.Data = data
		p.dataAndSlot.Slot = newSlot
	}
}

func (p *GeyserAccountSubscriber[T]) setTimeout() {
	if p.onChange == nil {
		panic("onChange callback function must be set")
	}
	if p.isUnsubscribing {
		// If we are in the process of unsubscribing, do not attempt to resubscribe
		return
	}
	p.Unsubscribe()
	time.Sleep(500 * time.Millisecond)
	p.Subscribe(p.onChange)
}
func (p *GeyserAccountSubscriber[T]) Fetch() {
	accountInfo, err := p.program.GetProvider().GetConnection().GetAccountInfo(context.TODO(), p.accountPublicKey)
	if err != nil {
		return
	}
	var data *T
	if p.decodeBufferFn != nil {
		data = p.decodeBufferFn(accountInfo.GetBinary())
	} else {
		err = bin.NewBinDecoder(accountInfo.GetBinary()).Decode(&data)
		if err != nil {
			return
		}
	}
	p.SetData(data)
	p.onChange(data)
}

func (p *GeyserAccountSubscriber[T]) handleRpcResponse(subscribeUpdate *solana_geyser_grpc.SubscribeUpdate) {
	account := subscribeUpdate.GetAccount()
	if account == nil {
		return
	}
	var data T
	err := bin.NewBinDecoder(account.GetAccount().GetData()).Decode(&data)
	if err != nil {
		return
	}
	p.SetData(&data, account.GetSlot())
	p.onChange(&data)
}

func (p *GeyserAccountSubscriber[T]) Unsubscribe() {
	p.isUnsubscribing = true
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	if p.subscription != nil {
		p.subscription.Unsubscribe()
	}
	p.isUnsubscribing = false
}

func (p *GeyserAccountSubscriber[T]) GetDataAndSlot() *DataAndSlot[*T] {
	return p.dataAndSlot
}
