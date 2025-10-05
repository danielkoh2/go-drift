package accounts

import (
	"context"
	"driftgo/anchor/types"
	"driftgo/lib/geyser"
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	"driftgo/utils"
	"fmt"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"slices"
	"time"
)

type GeyserMultiAccountSubscriber[T any] struct {
	MultiAccountSubscriber[T]
	dataAndSlots      map[string]*DataAndSlot[*T]
	bufferAndSlot     map[string]*BufferAndSlot
	accountName       string
	program           types.IProgram
	accountPublicKeys []solana.PublicKey
	decodeBufferFn    func(solana.PublicKey, []byte) *T
	onChange          func(string, *T)
	resubTimeoutMs    int64
	lastReceived      int64
	commitment        rpc.CommitmentType
	isUnsubscribing   bool

	subscription  *geyser.Subscription
	receivingData bool
	cancel        func()
}

func CreateGeyserMultiAccountSubscriber[T any](
	accountName string,
	program types.IProgram,
	accountPublicKeys []solana.PublicKey,
	decodeBuffer func(solana.PublicKey, []byte) *T,
	resubTimeoutMs *int64,
	commitment *rpc.CommitmentType,
) *GeyserMultiAccountSubscriber[T] {
	subscriber := &GeyserMultiAccountSubscriber[T]{
		accountName:       accountName,
		program:           program,
		accountPublicKeys: accountPublicKeys,
		decodeBufferFn:    decodeBuffer,
		dataAndSlots:      make(map[string]*DataAndSlot[*T]),
		bufferAndSlot:     make(map[string]*BufferAndSlot),
		resubTimeoutMs:    utils.TTM[int64](resubTimeoutMs == nil, int64(10000), func() int64 { return max(10000, *resubTimeoutMs) }),
		lastReceived:      0,
		commitment:        utils.TTM[rpc.CommitmentType](commitment != nil, func() rpc.CommitmentType { return *commitment }, program.GetProvider().GetOpts().Commitment),
		receivingData:     false,
	}
	return subscriber
}
func (p *GeyserMultiAccountSubscriber[T]) Subscribe(onChange func(string, *T)) {
	if p.isUnsubscribing {
		return
	}
	p.onChange = onChange
	if len(p.dataAndSlots) == 0 {
		p.Fetch()
	}
	subscription, err := p.program.GetProvider().GetGeyserConnection().Subscription()
	if err != nil {
		return
	}
	pubKeys := utils.ValuesFunc[solana.PublicKey, string](p.accountPublicKeys, func(key solana.PublicKey) string {
		return key.String()
	})

	err = subscription.Request(geyser.NewSubscriptionRequestBuilder().Accounts(pubKeys...).Build())
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
func (p *GeyserMultiAccountSubscriber[T]) Fetch() {
	defer func() {
		if e := recover(); e != nil {
			fmt.Println("Multi account subscriber error: "+p.accountName, e)
		}
	}()
	accountInfos, err := p.program.GetProvider().GetConnection().GetMultipleAccounts(context.TODO(), p.accountPublicKeys...)
	if err != nil {
		return
	}
	var data *T
	idx := 0
	for _, accountInfo := range accountInfos.Value {
		var key solana.PublicKey
		if len(p.accountPublicKeys) > idx {
			key = p.accountPublicKeys[idx]
		} else {
			continue
		}
		if p.decodeBufferFn != nil {
			data = p.decodeBufferFn(key, accountInfo.Data.GetBinary())
		} else {
			var newData T
			data = &newData

			err = bin.NewBinDecoder(accountInfo.Data.GetBinary()).Decode(data)
			if err != nil {
				continue
			}
		}
		p.SetData(key, data)
		p.onChange(key.String(), data)
		idx++
	}
}
func (p *GeyserMultiAccountSubscriber[T]) Add(pubkey solana.PublicKey) bool {
	exists := slices.ContainsFunc(p.accountPublicKeys, func(key solana.PublicKey) bool {
		return pubkey.Equals(key)
	})
	if exists {
		return false
	}
	p.Unsubscribe()
	p.accountPublicKeys = append(p.accountPublicKeys, pubkey)
	p.Subscribe(p.onChange)
	return true
}

func (p *GeyserMultiAccountSubscriber[T]) Unsubscribe() {
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
func (p *GeyserMultiAccountSubscriber[T]) SetData(key solana.PublicKey, data *T, slot ...uint64) {
	newSlot := uint64(0)
	if len(slot) > 0 {
		newSlot = slot[0]
	}
	dataAndSlot, exists := p.dataAndSlots[key.String()]
	if exists && dataAndSlot.Slot > newSlot {
		return
	}
	if !exists {
		p.dataAndSlots[key.String()] = &DataAndSlot[*T]{
			Data: data,
			Slot: newSlot,
		}
	} else {
		dataAndSlot.Data = data
		dataAndSlot.Slot = newSlot
	}
}
func (p *GeyserMultiAccountSubscriber[T]) GetDataAndSlots() map[string]*DataAndSlot[*T] {
	return p.dataAndSlots
}

func (p *GeyserMultiAccountSubscriber[T]) GetDataAndSlot(key string) *DataAndSlot[*T] {
	dataAndSlot, exists := p.dataAndSlots[key]
	if exists {
		return dataAndSlot
	}
	return nil
}

func (p *GeyserMultiAccountSubscriber[T]) Subscribed() bool {
	return p.subscription != nil
}

func (p *GeyserMultiAccountSubscriber[T]) setTimeout() {
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

func (p *GeyserMultiAccountSubscriber[T]) handleRpcResponse(subscribeUpdate *solana_geyser_grpc.SubscribeUpdate) {
	account := subscribeUpdate.GetAccount()
	if account == nil {
		return
	}
	key := solana.PublicKeyFromBytes(account.GetAccount().GetPubkey())
	var data *T
	if p.decodeBufferFn != nil {
		data = p.decodeBufferFn(key, account.GetAccount().GetData())
	} else {
		var newData T
		data = &newData

		err := bin.NewBinDecoder(account.GetAccount().GetData()).Decode(data)
		if err != nil {
			return
		}
	}
	p.SetData(key, data, account.GetSlot())
	p.onChange(key.String(), data)
}
