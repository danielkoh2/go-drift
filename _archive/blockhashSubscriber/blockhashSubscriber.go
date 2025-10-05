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
	"github.com/go-errors/errors"
	"sync"
	"time"
)

type BlockhashSubscriberConfig struct {
	Program        types.IProgram
	ConnectionId   string
	Commitment     *rpc.CommitmentType
	ResubTimeoutMs *int64
}

type BlockhashSubscriber struct {
	program      types.IProgram
	connectionId string
	commitment   rpc.CommitmentType

	subscription *geyser.Subscription
	eventEmitter *event.EventEmitter

	resubTimeoutMs    int64
	isUnsubscribing   bool
	latestBlockHeight uint64
	receivingData     bool
	blockhashes       []*rpc.LatestBlockhashResult
	mxState           *sync.RWMutex
	cancel            func()
}

func CreateBlockhashSubscriber(config BlockhashSubscriberConfig) *BlockhashSubscriber {
	return &BlockhashSubscriber{
		program:        config.Program,
		connectionId:   config.ConnectionId,
		commitment:     utils2.TTM[rpc.CommitmentType](config.Commitment == nil, rpc.CommitmentConfirmed, func() rpc.CommitmentType { return *config.Commitment }),
		resubTimeoutMs: utils2.TTM[int64](config.ResubTimeoutMs == nil, int64(5000), func() int64 { return *config.ResubTimeoutMs }),
		eventEmitter:   go_drift.EventEmitter(),
		mxState:        new(sync.RWMutex),
	}
}

func (p *BlockhashSubscriber) updateBlockhash() {
	defer p.mxState.Unlock()
	p.mxState.Lock()
	blockhash, _ := p.program.GetProvider().GetConnection().GetLatestBlockhash(context.TODO(), p.commitment)
	blockHeight, _ := p.program.GetProvider().GetConnection().GetBlockHeight(context.TODO(), p.commitment)

	p.latestBlockHeight = blockHeight

	if len(p.blockhashes) > 0 {
		if blockhash.Value.Blockhash.Equals(p.blockhashes[0].Blockhash) {
			return
		}
	}
	p.blockhashes = append(p.blockhashes, blockhash.Value)
}

func (p *BlockhashSubscriber) GetBlockhashCacheSize() int {
	return len(p.blockhashes)
}

func (p *BlockhashSubscriber) GetLatestBlockHeight() uint64 {
	return p.latestBlockHeight
}

func (p *BlockhashSubscriber) GetLatestBlockhash(offsets ...int) *rpc.LatestBlockhashResult {
	defer p.mxState.RUnlock()
	p.mxState.RLock()
	offset := utils2.TTM[int](len(offsets) > 0, func() int { return offsets[0] }, 0)
	if len(p.blockhashes) == 0 {
		return nil
	}
	clampedOffset := max(0, min(len(p.blockhashes)-1, offset))
	return p.blockhashes[len(p.blockhashes)-1-clampedOffset]
}

func (p *BlockhashSubscriber) pruneBlockhashes() {
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

func (p *BlockhashSubscriber) Subscribe() {
	if p.subscription != nil {
		return
	}
	p.updateBlockhash()
	connection := p.program.GetProvider().GetGeyserConnection()
	subscription, _ := connection.Subscription()
	_ = subscription.Request(geyser.NewSubscriptionRequestBuilder().BlockMeta().Build())
	lastReceived := time.Now().UnixMilli()
	subscription.Subscribe(&geyser.SubscribeEvents{
		Update: func(data interface{}) {
			defer func() {
				if e := recover(); e != nil {
					fmt.Println("Blockhash subscriber error", errors.Wrap(e, 2).ErrorStack())
				}
			}()
			lastReceived = time.Now().UnixMilli()
			//fmt.Printf("BlockHashSubscriber new update : %v\n", data)
			//defer p.mxState.Unlock()
			//p.mxState.Lock()
			blockMeta := data.(*solana_geyser_grpc.SubscribeUpdate).GetBlockMeta()
			if blockMeta == nil {
				return
			}
			blockHash, _ := ag_solanago.HashFromBase58(blockMeta.GetBlockhash())
			blockHeight := blockMeta.GetBlockHeight().GetBlockHeight()
			defer p.mxState.Unlock()
			p.mxState.Lock()
			p.latestBlockHeight = blockHeight
			p.blockhashes = append(p.blockhashes, &rpc.LatestBlockhashResult{
				Blockhash:            blockHash,
				LastValidBlockHeight: blockHeight,
			})
			//p.eventEmitter.Emit("newBlock", blockHash)
			p.pruneBlockhashes()
			//fmt.Printf("BlockHashSubscriber  : %v\n", blockMeta)
		},
		Error: nil,
		Eof:   nil,
	})
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go func(ctx context.Context) {
		ticker := time.NewTicker(100 * time.Millisecond)
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

func (p *BlockhashSubscriber) setTimeout() {
	if p.isUnsubscribing {
		return
	}
	p.Unsubscribe()
	time.Sleep(500 * time.Millisecond)
	p.Subscribe()
}

func (p *BlockhashSubscriber) Unsubscribe() {
	//fmt.Println("Blockhash Unsubscribing")
	if p.subscription == nil {
		return
	}
	p.isUnsubscribing = true
	p.subscription.Unsubscribe()
	p.subscription = nil
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.isUnsubscribing = false
}
