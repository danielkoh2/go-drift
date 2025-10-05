package userMap

import (
	go_drift "driftgo"
	"driftgo/accounts"
	"driftgo/lib/drift"
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type GeyserSubscription struct {
	userMap         *UserMap
	commitment      rpc.CommitmentType
	skipInitialLoad bool
	resubTimeoutMs  int64
	includeIdle     bool
	decodeFn        func(name string, data []byte) *drift.User
	subscriber      accounts.IProgramAccountSubscriber[drift.User]
}

func CreateGeyserSubscription(
	userMap *UserMap,
	commitment rpc.CommitmentType,
	skipInitialLoad bool,
	resubTimeoutMs int64,
	includeIdle bool,
	decodeFn func(name string, data []byte) *drift.User,
) *GeyserSubscription {
	return &GeyserSubscription{
		userMap:         userMap,
		commitment:      commitment,
		skipInitialLoad: skipInitialLoad,
		resubTimeoutMs:  resubTimeoutMs,
		includeIdle:     includeIdle,
		decodeFn:        decodeFn,
	}
}

func (p *GeyserSubscription) Subscribe() {
	if p.subscriber != nil {
		return
	}
	filters := []rpc.RPCFilter{go_drift.GetUserFilter()}
	if !p.includeIdle {
		filters = append(filters, go_drift.GetNonIdleUserFiler())
	}
	p.subscriber = accounts.CreateGeyserProgramAccountSubscriber[drift.User](
		"UserMap",
		"User",
		p.userMap.driftClient.GetProgram(),
		p.decodeFn,
		accounts.ProgramAccountSubscriberOptions{
			Filters:    filters,
			Commitment: &p.commitment,
		},
		&p.resubTimeoutMs,
	)

	if !p.skipInitialLoad {
		p.userMap.Sync()
	}

	p.subscriber.Subscribe(func(
		accountId solana.PublicKey,
		account *drift.User,
		context rpc.Context,
		buffer []byte,
		accountInfo ...*solana_geyser_grpc.SubscribeUpdateAccountInfo,
	) {
		txnHash := ""
		userKey := accountId.String()
		if len(accountInfo) > 0 {
			txnHash = solana.SignatureFromBytes(accountInfo[0].TxnSignature).String()
		}
		p.userMap.UpdateUserAccount(userKey, account, context.Slot, txnHash, buffer, true)
		fmt.Printf("user updated : %s\n", userKey)
	})
}

func (p *GeyserSubscription) Unsubscribe() {
	if p.subscriber == nil {
		return
	}
	p.subscriber.Unsubscribe()
	p.subscriber = nil
}
