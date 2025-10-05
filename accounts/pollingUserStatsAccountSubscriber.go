package accounts

import (
	go_drift "driftgo"
	"driftgo/anchor/types"
	driftlib "driftgo/lib/drift"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type PollingUserStatsAccountSubscriber struct {
	UserStatsAccountSubscriber
	program                   types.IProgram
	userStatsAccountPublicKey solana.PublicKey

	accountLoader   *BulkAccountLoader
	callbackId      string
	errorCallbackId string

	userStats *DataAndSlot[*driftlib.UserStats]
}

func CreatePollingUserStatsAccountSubscriber(
	program types.IProgram,
	userStatsAccountPublicKey solana.PublicKey,
	accountLoader *BulkAccountLoader,
) *PollingUserStatsAccountSubscriber {
	return &PollingUserStatsAccountSubscriber{
		UserStatsAccountSubscriber: UserStatsAccountSubscriber{
			EventEmitter: go_drift.EventEmitter(),
			IsSubscribed: false,
		},
		program:                   program,
		accountLoader:             accountLoader,
		userStatsAccountPublicKey: userStatsAccountPublicKey,
	}
}

func (p *PollingUserStatsAccountSubscriber) Subscribe(
	userStatsAccount *driftlib.UserStats,
) bool {
	if p.IsSubscribed {
		return true
	}
	if userStatsAccount != nil {
		p.userStats = &DataAndSlot[*driftlib.UserStats]{
			Data:   userStatsAccount,
			Slot:   0,
			Pubkey: solana.PublicKey{},
		}
	}

	p.AddToAccountLoader()

	p.FetchIfUnloaded()

	if p.DoesAccountExists() {
		p.EventEmitter.Emit("update")
	}

	p.IsSubscribed = true
	return true
}

func (p *PollingUserStatsAccountSubscriber) AddToAccountLoader() {
	if p.callbackId != "" {
		return
	}

	p.callbackId = p.accountLoader.AddAccount(
		p.userStatsAccountPublicKey,
		func(buffer []byte, slot uint64) {
			if len(buffer) == 0 {
				return
			}

			if p.userStats != nil && p.userStats.Slot > slot {
				return
			}

			var account driftlib.UserStats
			err := bin.NewBinDecoder(buffer).Decode(&account)
			if err != nil {
				return
			}
			p.userStats = &DataAndSlot[*driftlib.UserStats]{
				Data: &account,
				Slot: slot,
			}
			p.EventEmitter.Emit("userStatsAccountUpdate", account)
			p.EventEmitter.Emit("update")

		})
	p.errorCallbackId = p.accountLoader.AddErrorCallbacks(func(err error) {
		p.EventEmitter.Emit("error", err)
	})
}

func (p *PollingUserStatsAccountSubscriber) FetchIfUnloaded() {
	if p.userStats == nil {
		p.Fetch()
	}
}

func (p *PollingUserStatsAccountSubscriber) Fetch() {
	//SharedUserStatsCache()
	//if userStatsAccount == nil {
	//	buffer, err := accounts.SharedUserStatsCache().Get(userStatsAccountKey.String(), nil)
	//	if err != nil {
	//		var userStatsLoaded drift.UserStats
	//		err = bin.NewBinDecoder(buffer).Decode(&userStatsLoaded)
	//		if err == nil {
	//			userStatsAccount = &userStatsLoaded
	//		}
	//	}
	//}
	data, context := p.program.
		GetAccounts(
			driftlib.UserStats{},
			"UserStats",
		).
		FetchAndContext(
			p.userStatsAccountPublicKey,
			p.accountLoader.commitment,
		)
	if p.userStats == nil || context.Slot > p.userStats.Slot {
		p.userStats = &DataAndSlot[*driftlib.UserStats]{
			Data: data.(*driftlib.UserStats),
			Slot: context.Slot,
		}
	}
}

func (p *PollingUserStatsAccountSubscriber) DoesAccountExists() bool {
	return p.userStats != nil
}

func (p *PollingUserStatsAccountSubscriber) Unsubscribe() {
	if !p.IsSubscribed {
		return
	}

	p.accountLoader.RemoveAccount(p.userStatsAccountPublicKey, p.callbackId)
	p.callbackId = ""

	p.accountLoader.RemoveErrorCallbacks(p.errorCallbackId)
	p.errorCallbackId = ""

	p.IsSubscribed = false
}

func (p *PollingUserStatsAccountSubscriber) GetUserStatsAccountAndSlot() *DataAndSlot[*driftlib.UserStats] {
	return p.userStats
}
