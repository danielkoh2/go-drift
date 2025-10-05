package auctionSubscriber

import (
	go_drift "driftgo"
	"driftgo/accounts"
	"driftgo/drift/types"
	"driftgo/lib/drift"
	"driftgo/lib/event"
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type AuctionSubscriber struct {
	driftClient    types.IDriftClient
	opts           go_drift.ConfirmOptions
	resubTimeoutMs int64

	eventEmitter *event.EventEmitter
	subscriber   accounts.IProgramAccountSubscriber[drift.User]
}

func CreateAuctionSubscriber(config AuctionSubscriberConfig) *AuctionSubscriber {
	return &AuctionSubscriber{
		driftClient:    config.DriftClient,
		opts:           config.Opts,
		resubTimeoutMs: max(5000, config.ResubTimeoutMs),
		eventEmitter:   go_drift.EventEmitter(),
		subscriber:     nil,
	}
}

func (p *AuctionSubscriber) Subscribe() {
	if p.subscriber == nil {
		p.subscriber = accounts.CreateGeyserProgramAccountSubscriber(
			"AuctionSubscriber",
			"User",
			p.driftClient.GetProgram(),
			func(discriminator string, buffer []byte) *drift.User {
				var userAccount drift.User
				err := bin.NewBinDecoder(buffer).Decode(&userAccount)
				if err != nil {
					return nil
				}
				return &userAccount
			},
			accounts.ProgramAccountSubscriberOptions{
				Filters:    []rpc.RPCFilter{go_drift.GetUserFilter(), go_drift.GetUserWithAuctionFilter()},
				Commitment: &p.opts.Commitment,
			},
			&p.resubTimeoutMs,
		)
	}
	p.subscriber.Subscribe(func(accountId solana.PublicKey, userAccount *drift.User, rpcContext rpc.Context, buffer []byte, accountInfo ...*solana_geyser_grpc.SubscribeUpdateAccountInfo) {
		p.eventEmitter.Emit("onAccountUpdate", userAccount, accountId, rpcContext.Slot)
	})
}

func (p *AuctionSubscriber) Unsubscribe() {
	if p.subscriber == nil {
		return
	}
	p.subscriber.Unsubscribe()
}
