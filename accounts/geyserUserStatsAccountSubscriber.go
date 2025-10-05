package accounts

import (
	go_drift "driftgo"
	"driftgo/anchor/types"
	"driftgo/lib/drift"
	"driftgo/utils"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type GeyserUserStatsAccountSubscriber struct {
	UserStatsAccountSubscriber
	reconnectTimeoutMs        *int64
	commitment                *rpc.CommitmentType
	program                   types.IProgram
	userStatsAccountPublicKey solana.PublicKey

	userStatsAccountSubscriber IAccountSubscriber[drift.UserStats]
}

func CreateGeyserUserStatsAccountSubscriber(
	program types.IProgram,
	userStatsAccountPublicKey solana.PublicKey,
	reconnectTimeoutMs *int64,
	commitment *rpc.CommitmentType,
) *GeyserUserStatsAccountSubscriber {
	p := &GeyserUserStatsAccountSubscriber{
		UserStatsAccountSubscriber: UserStatsAccountSubscriber{
			EventEmitter: go_drift.EventEmitter(),
			IsSubscribed: false,
		},
		program:                   program,
		userStatsAccountPublicKey: userStatsAccountPublicKey,
		reconnectTimeoutMs:        reconnectTimeoutMs,
		commitment:                utils.TT(commitment != nil, commitment, &program.GetProvider().GetOpts().Commitment),
	}
	return p
}

func (p *GeyserUserStatsAccountSubscriber) Subscribed() bool {
	return p.IsSubscribed
}

func (p *GeyserUserStatsAccountSubscriber) Subscribe(userStatsAccount *drift.UserStats) bool {
	if p.IsSubscribed {
		return true
	}

	p.userStatsAccountSubscriber = CreateGeyserAccountSubscriber[drift.UserStats](
		"userStats",
		p.program,
		p.userStatsAccountPublicKey,
		nil,
		p.reconnectTimeoutMs,
		p.commitment,
	)

	if userStatsAccount != nil {
		p.userStatsAccountSubscriber.SetData(userStatsAccount)
	}
	p.userStatsAccountSubscriber.Subscribe(func(data *drift.UserStats) {
		p.EventEmitter.Emit("userStatsAccountUpdate", data)
		p.EventEmitter.Emit("update")
	})
	p.EventEmitter.Emit("update")
	p.IsSubscribed = true
	return true
}

func (p *GeyserUserStatsAccountSubscriber) Fetch() {
	p.userStatsAccountSubscriber.Fetch()
}

func (p *GeyserUserStatsAccountSubscriber) Unsubscribe() {
	if p.IsSubscribed == false {
		return
	}
	p.userStatsAccountSubscriber.Unsubscribe()
	p.IsSubscribed = false
}

func (p *GeyserUserStatsAccountSubscriber) AssertIsSubscribed() {
	if !p.IsSubscribed {
		panic("You must call `subscribe` before using this function")
	}
}

func (p *GeyserUserStatsAccountSubscriber) GetUserStatsAccountAndSlot() *DataAndSlot[*drift.UserStats] {
	return p.userStatsAccountSubscriber.GetDataAndSlot()
}
