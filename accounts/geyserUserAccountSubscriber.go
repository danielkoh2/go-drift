package accounts

import (
	go_drift "driftgo"
	"driftgo/anchor/types"
	"driftgo/lib/drift"
	"driftgo/utils"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type GeyserUserAccountSubscriber struct {
	UserAccountSubscriber
	reconnectTimeoutMs   *int64
	commitment           *rpc.CommitmentType
	program              types.IProgram
	userAccountPublicKey solana.PublicKey

	userDataAccountSubscriber IAccountSubscriber[drift.User]
}

func CreateGeyserUserAccountSubscriber(
	program types.IProgram,
	userAccountPublicKey solana.PublicKey,
	reconnectTimeoutMs *int64,
	commitment *rpc.CommitmentType,
) *GeyserUserAccountSubscriber {
	p := &GeyserUserAccountSubscriber{
		UserAccountSubscriber: UserAccountSubscriber{
			EventEmitter: go_drift.EventEmitter(),
			IsSubscribed: false,
		},
		program:              program,
		userAccountPublicKey: userAccountPublicKey,
		reconnectTimeoutMs:   reconnectTimeoutMs,
		commitment:           utils.TT(commitment != nil, commitment, &program.GetProvider().GetOpts().Commitment),
	}
	return p
}

func (p *GeyserUserAccountSubscriber) Subscribed() bool {
	return p.IsSubscribed
}

func (p *GeyserUserAccountSubscriber) Subscribe(userAccount *drift.User) bool {
	if p.IsSubscribed {
		return true
	}

	p.userDataAccountSubscriber = CreateGeyserAccountSubscriber[drift.User](
		"user",
		p.program,
		p.userAccountPublicKey,
		nil,
		p.reconnectTimeoutMs,
		p.commitment,
	)

	if userAccount != nil {
		p.userDataAccountSubscriber.SetData(userAccount)
	}
	p.userDataAccountSubscriber.Subscribe(func(data *drift.User) {
		p.EventEmitter.Emit("userAccountUpdate", data)
		p.EventEmitter.Emit("update")
	})
	p.EventEmitter.Emit("update")
	p.IsSubscribed = true
	return true
}

func (p *GeyserUserAccountSubscriber) Fetch() {
	p.userDataAccountSubscriber.Fetch()
}

func (p *GeyserUserAccountSubscriber) Unsubscribe() {
	if p.IsSubscribed == false {
		return
	}
	p.userDataAccountSubscriber.Unsubscribe()
	p.IsSubscribed = false
}

func (p *GeyserUserAccountSubscriber) AssertIsSubscribed() {
	if !p.IsSubscribed {
		panic("You must call `subscribe` before using this function")
	}
}

func (p *GeyserUserAccountSubscriber) GetUserAccountAndSlot() *DataAndSlot[*drift.User] {
	return p.userDataAccountSubscriber.GetDataAndSlot()
}

func (p *GeyserUserAccountSubscriber) UpdateData(userAccount *drift.User, slot uint64) {
	var currentDataSlot uint64 = 0
	dataAndSlot := p.userDataAccountSubscriber.GetDataAndSlot()
	if dataAndSlot != nil {
		currentDataSlot = dataAndSlot.Slot
	}
	if currentDataSlot <= slot {
		p.userDataAccountSubscriber.SetData(userAccount, slot)
		p.EventEmitter.Emit("userAccountUpdate", userAccount)
		p.EventEmitter.Emit("update")
	}
}
