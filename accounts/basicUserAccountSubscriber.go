package accounts

import (
	go_drift "driftgo"
	"driftgo/lib/drift"
	"driftgo/utils"
	"github.com/gagliardetto/solana-go"
)

type BasicUserAccountSubscriber struct {
	UserAccountSubscriber
	userAccountPublicKey solana.PublicKey

	user *DataAndSlot[*drift.User]
}

func CreateBasicUserAccountSubscriber(
	userAccountPublicKey solana.PublicKey,
	data *drift.User,
	slot *uint64,
) *BasicUserAccountSubscriber {
	return &BasicUserAccountSubscriber{
		UserAccountSubscriber: UserAccountSubscriber{
			EventEmitter: go_drift.EventEmitter(),
			IsSubscribed: false,
		},
		userAccountPublicKey: userAccountPublicKey,
		user: &DataAndSlot[*drift.User]{
			Data: data,
			Slot: utils.TTM[uint64](slot == nil, uint64(0), func() uint64 { return *slot }),
		},
	}
}

func (p *BasicUserAccountSubscriber) Subscribed() bool {
	return p.IsSubscribed
}

func (p *BasicUserAccountSubscriber) Subscribe(data *drift.User) bool {
	return true
}

func (p *BasicUserAccountSubscriber) Fetch() {}

func (p *BasicUserAccountSubscriber) doesAccountExist() bool {
	return p.user != nil
}

func (p *BasicUserAccountSubscriber) Unsubscribe() {

}

func (p *BasicUserAccountSubscriber) assertIsSubscribed() {}

func (p *BasicUserAccountSubscriber) GetUserAccountAndSlot() *DataAndSlot[*drift.User] {
	return p.user
}

func (p *BasicUserAccountSubscriber) UpdateData(userAccount *drift.User, slot uint64) {
	if p.user == nil && slot >= p.user.Slot {
		p.user = &DataAndSlot[*drift.User]{
			Data: userAccount,
			Slot: slot,
		}
		p.EventEmitter.Emit("userAccountUpdate", userAccount)
		p.EventEmitter.Emit("update")
	}
}
