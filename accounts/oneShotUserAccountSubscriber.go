package accounts

import (
	"driftgo/anchor/types"
	"driftgo/lib/drift"
	"driftgo/utils"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type OneShotUserAccountSubscriber struct {
	BasicUserAccountSubscriber
	program    types.IProgram
	commitment rpc.CommitmentType
}

func CreateOneShotUserAccountSubscriber(
	program types.IProgram,
	userAccountPublicKey solana.PublicKey,
	data *drift.User,
	slot *uint64,
	commitment *rpc.CommitmentType,
) *OneShotUserAccountSubscriber {
	return &OneShotUserAccountSubscriber{
		BasicUserAccountSubscriber: *CreateBasicUserAccountSubscriber(userAccountPublicKey, data, slot),
		program:                    program,
		commitment:                 utils.TT(commitment == nil, rpc.CommitmentConfirmed, *commitment),
	}
}

func (p *OneShotUserAccountSubscriber) Subscribe(userAccount *drift.User) bool {
	if userAccount != nil {
		p.user.Data = userAccount
		return true
	}
	p.fetchIfUnloaded()
	if p.doesAccountExist() {
		p.EventEmitter.Emit("update")
	}
	return true
}

func (p *OneShotUserAccountSubscriber) fetchIfUnloaded() {
	if p.user.Data == nil {
		p.Fetch()
	}
}

func (p *OneShotUserAccountSubscriber) Fetch() {
	data, context := p.program.GetAccounts(drift.User{}, "User").FetchAndContext(
		p.userAccountPublicKey,
		p.commitment,
	)
	if data != nil {
		if context.Slot > p.user.Slot {
			p.user.Data = data.(*drift.User)
		}
	} else {
		fmt.Println("OneShotUserAccountSubscriber.fetch() UserAccount does not exist")
	}
}
