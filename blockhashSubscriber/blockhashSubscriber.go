package blockhashSubscriber

import (
	"context"
	go_drift "driftgo"
	"driftgo/anchor/types"
	"driftgo/lib/event"
	"driftgo/lib/geyser"
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	utils2 "driftgo/utils"
	"fmt"
	ag_solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"sync"
	"time"
)

type BlockHashSubscriberConfig struct {
	Program          types.IProgram
	ConnectionId     string
	ResubTimeoutMs   *int64
	Commitment       *rpc.CommitmentType
	updateIntervalMs *int64
	EventEmitter     *event.EventEmitter
}

type BlockHashSubscriber struct {
	program      types.IProgram
	connectionId string
	commitment   rpc.CommitmentType

	// Slot data
	currentSlot uint64

	// Blockhash data
	updateIntervalMs  int64
	latestBlockHeight uint64
	latestBlockHash   ag_solanago.Hash
	blockhashes       []*rpc.LatestBlockhashResult

	connection   *geyser.Connection
	subscription *geyser.Subscription
	eventEmitter *event.EventEmitter

	// Reconnection
	resubTimeoutMs  int64
	isUnsubscribing bool
	receivingData   bool
	mxState         *sync.RWMutex
	cancel          func()
}

func CreateBlockHashSubscriber(config BlockHashSubscriberConfig) *BlockHashSubscriber {
	return &BlockHashSubscriber{
		program:          config.Program,
		connectionId:     config.ConnectionId,
		commitment:       utils2.TTM[rpc.CommitmentType](config.Commitment == nil, rpc.CommitmentConfirmed, func() rpc.CommitmentType { return *config.Commitment }),
		resubTimeoutMs:   utils2.TTM[int64](config.ResubTimeoutMs == nil, int64(10_000), func() int64 { return *config.ResubTimeoutMs }),
		eventEmitter:     utils2.TT(config.EventEmitter == nil, go_drift.EventEmitter(), config.EventEmitter),
		mxState:          new(sync.RWMutex),
		updateIntervalMs: utils2.TTM[int64](config.updateIntervalMs == nil, int64(100), func() int64 { return max(*config.updateIntervalMs, 1000) }),
	}
}

func (p *BlockHashSubscriber) GetEventEmitter() *event.EventEmitter {
	return p.eventEmitter
}

func (p *BlockHashSubscriber) updateBlockhash() {
	defer p.mxState.Unlock()
	p.mxState.Lock()
	blockhash, _ := p.program.GetProvider().GetConnection(p.connectionId).GetLatestBlockhash(context.TODO(), p.commitment)
	blockHeight, _ := p.program.GetProvider().GetConnection(p.connectionId).GetBlockHeight(context.TODO(), p.commitment)

	p.latestBlockHeight = blockHeight

	if len(p.blockhashes) > 0 {
		if blockhash.Value.Blockhash.Equals(p.blockhashes[0].Blockhash) {
			return
		}
	}
	p.blockhashes = append(p.blockhashes, blockhash.Value)
	p.pruneBlockhashes()
}

func (p *BlockHashSubscriber) GetBlockhashCacheSize() int {
	defer p.mxState.RUnlock()
	p.mxState.RLock()
	return len(p.blockhashes)
}

func (p *BlockHashSubscriber) GetLatestBlockHeight() uint64 {
	defer p.mxState.RUnlock()
	p.mxState.RLock()
	return p.latestBlockHeight
}

func (p *BlockHashSubscriber) GetLatestBlockhash(offsets ...int) *rpc.LatestBlockhashResult {
	defer p.mxState.RUnlock()
	p.mxState.RLock()
	//return p.latestBlockHash
	offset := utils2.TTM[int](len(offsets) > 0, func() int { return offsets[0] }, 0)
	if len(p.blockhashes) == 0 {
		return nil
	}
	clampedOffset := max(0, min(len(p.blockhashes)-1, offset))
	return p.blockhashes[len(p.blockhashes)-1-clampedOffset]
}

func (p *BlockHashSubscriber) pruneBlockhashes() {
	if p.latestBlockHeight > 0 {
		var newBlockhashes []*rpc.LatestBlockhashResult
		for _, blockhash := range p.blockhashes {
			if blockhash.LastValidBlockHeight > p.latestBlockHeight {
				newBlockhashes = append(newBlockhashes, blockhash)
			}
		}
		p.blockhashes = newBlockhashes
	}
}

func (p *BlockHashSubscriber) GetSlot() uint64 {
	defer p.mxState.RUnlock()
	p.mxState.RLock()

	return p.currentSlot
}

func (p *BlockHashSubscriber) Fetch() {
	connection := p.program.GetProvider().GetConnection(p.connectionId)
	slot, err := connection.GetSlot(context.TODO(), p.commitment)
	if err == nil {
		p.currentSlot = slot
	}
	blockhash, err := connection.GetLatestBlockhash(context.TODO(), p.commitment)
	if err == nil {
		p.latestBlockHash = blockhash.Value.Blockhash
		p.blockhashes = append(p.blockhashes, blockhash.Value)
	}
	blockHeight, err := connection.GetBlockHeight(context.TODO(), p.commitment)
	if err == nil {
		p.latestBlockHeight = blockHeight
	}
}

func (p *BlockHashSubscriber) Subscribe() {
	if p.subscription != nil {
		return
	}
	p.Fetch()
	p.connection = p.program.GetProvider().GetGeyserConnection(p.connectionId)

	subscription, err := p.connection.Subscription()
	if err != nil {
		panic(err.Error())
	}
	err = subscription.Request(
		geyser.NewSubscriptionRequestBuilder().
			Slots().
			//Blocks().
			//BlockMeta().
			SetCommitment(&p.commitment).
			Build())
	if err != nil {
		fmt.Println(err)
		return
	}
	p.subscription = subscription
	lastReceived := time.Now().UnixMilli()
	subscription.Subscribe(&geyser.SubscribeEvents{
		Update: func(data interface{}) {
			//defer func () {
			//	if e := recover();  e != nil {
			//		fmt.Println(errors.Wrap(e, 2).ErrorStack())
			//	}
			//}()
			//spew.Dump(data)
			lastReceived = time.Now().UnixMilli()
			defer p.mxState.Unlock()
			p.mxState.Lock()

			slotInfo := data.(*solana_geyser_grpc.SubscribeUpdate).GetSlot()
			if slotInfo != nil {
				p.currentSlot = slotInfo.Slot
				p.eventEmitter.Emit("newSlot", p.currentSlot)
			}
		},
		Error: func(err error) {
			fmt.Println("BlockHashSubscriber error : %v", err)
		},
		Eof: func() {
			fmt.Println("BlockHashSubscriber eof")
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	go func(ctx context.Context) {
		ticker := time.NewTicker(500 * time.Millisecond)
		tickerForUpdateBlockHash := time.NewTicker(time.Duration(p.updateIntervalMs) * time.Millisecond)
		for {

			select {
			case <-tickerForUpdateBlockHash.C:
				p.updateBlockhash()
			case <-ticker.C:
				//_, _ = p.subscription.Client.Ping(context.TODO(), &solana_geyser_grpc.PingRequest{
				//	Count: 1,
				//})
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

func (p *BlockHashSubscriber) setTimeout() {
	if p.isUnsubscribing {
		return
	}
	p.Unsubscribe()
	time.Sleep(500 * time.Millisecond)
	p.Subscribe()
}

func (p *BlockHashSubscriber) Unsubscribe() {
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
