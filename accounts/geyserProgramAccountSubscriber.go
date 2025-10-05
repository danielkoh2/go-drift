package accounts

import (
	"bytes"
	"context"
	"driftgo/anchor/types"
	"driftgo/lib/geyser"
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	"driftgo/utils"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/go-errors/errors"
	"time"
)

type GeyserProgramAccountSubscriber[T any] struct {
	ProgramAccountSubscriber[T]
	dataAndSlot          *DataAndSlot[*T]
	bufferAndSlot        *BufferAndSlot
	subscriptionName     string
	accountDiscriminator string
	program              types.IProgram
	decodeBuffer         func(string, []byte) *T
	onChange             func(solana.PublicKey, *T, rpc.Context, []byte, ...*solana_geyser_grpc.SubscribeUpdateAccountInfo)
	options              ProgramAccountSubscriberOptions
	resubTimeoutMs       int64
	lastReceived         int64
	commitment           rpc.CommitmentType
	isUnsubscribing      bool

	subscription  *geyser.Subscription
	receivingData bool
	cancel        func()
}

func CreateGeyserProgramAccountSubscriber[T any](
	subscriptionName string,
	accountDiscriminator string,
	program types.IProgram,
	decodeBufferFn func(string, []byte) *T,
	options ProgramAccountSubscriberOptions,
	resubTimeoutMs *int64,
) *GeyserProgramAccountSubscriber[T] {
	subscriber := &GeyserProgramAccountSubscriber[T]{
		subscriptionName:     subscriptionName,
		program:              program,
		accountDiscriminator: accountDiscriminator,
		decodeBuffer:         decodeBufferFn,
		resubTimeoutMs:       utils.TTM[int64](resubTimeoutMs == nil, int64(1000), func() int64 { return max(1000, *resubTimeoutMs) }),
		lastReceived:         0,
		options:              options,
		receivingData:        false,
	}

	return subscriber
}

func (p *GeyserProgramAccountSubscriber[T]) Subscribed() bool {
	return p.subscription != nil
}

func (p *GeyserProgramAccountSubscriber[T]) Subscribe(onChange func(accountId solana.PublicKey, data *T, ctx rpc.Context, buffer []byte, accountInfo ...*solana_geyser_grpc.SubscribeUpdateAccountInfo)) {
	p.onChange = onChange
	subscription, err := p.program.GetProvider().GetGeyserConnection().Subscription()
	if err != nil {
		panic(err)
		return
	}
	err = subscription.Request(
		geyser.NewSubscriptionRequestBuilder().
			ProgramAccounts(p.program.GetProgramId().String()).
			AccountFilters(p.options.Filters).
			SetCommitment(utils.TT(p.options.Commitment == nil, &p.program.GetProvider().GetOpts().Commitment, p.options.Commitment)).
			Build(),
	)
	if err != nil {
		panic(err)
		return
	}
	p.subscription = subscription
	p.subscription.Subscribe(&geyser.SubscribeEvents{
		Update: func(data interface{}) {
			//fmt.Printf("A: %v", data)
			p.lastReceived = time.Now().UnixMilli()
			p.handleRpcResponse(data.(*solana_geyser_grpc.SubscribeUpdate))
		},
		Error: func(err error) {
			fmt.Printf("A error: %v", err)
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

func (p *GeyserProgramAccountSubscriber[T]) setTimeout() {
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

func (p *GeyserProgramAccountSubscriber[T]) handleRpcResponse(subscribeUpdate *solana_geyser_grpc.SubscribeUpdate) {
	defer func() {
		if e := recover(); e != nil {
			fmt.Println(errors.Wrap(e, 2).ErrorStack())
			spew.Dump("program account subscriber error : ", e)
		}
	}()
	if p.decodeBuffer == nil {
		return
	}
	keyedAccountInfo := subscribeUpdate.GetAccount()
	if keyedAccountInfo == nil {
		return
	}
	newSlot := keyedAccountInfo.GetSlot()
	newBuffer := keyedAccountInfo.GetAccount().GetData()
	if p.bufferAndSlot == nil {
		p.bufferAndSlot = &BufferAndSlot{
			Buffer: newBuffer,
			Slot:   newSlot,
		}
		if len(newBuffer) > 0 {
			account := p.decodeBuffer(p.accountDiscriminator, newBuffer)
			if account != nil {
				accountId := solana.PublicKeyFromBytes(keyedAccountInfo.GetAccount().GetPubkey())
				p.dataAndSlot = &DataAndSlot[*T]{
					Data:   account,
					Slot:   newSlot,
					Pubkey: accountId,
				}
				p.onChange(accountId, account, rpc.Context{
					Slot: newSlot,
				}, newBuffer, keyedAccountInfo.GetAccount())
			}
		}
		return
	}

	if newSlot < p.bufferAndSlot.Slot {
		return
	}

	oldBuffer := p.bufferAndSlot.Buffer
	if newBuffer != nil && (oldBuffer == nil || bytes.Compare(oldBuffer, newBuffer) != 0) {
		p.bufferAndSlot = &BufferAndSlot{
			Buffer: newBuffer,
			Slot:   newSlot,
		}
		account := p.decodeBuffer(p.accountDiscriminator, newBuffer)
		if account != nil {
			accountId := solana.PublicKeyFromBytes(keyedAccountInfo.GetAccount().GetPubkey())
			p.dataAndSlot = &DataAndSlot[*T]{
				Data:   account,
				Slot:   newSlot,
				Pubkey: accountId,
			}
			p.onChange(accountId, account, rpc.Context{
				Slot: newSlot,
			}, newBuffer, keyedAccountInfo.GetAccount())
		}
	}
}

func (p *GeyserProgramAccountSubscriber[T]) Unsubscribe() {
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
