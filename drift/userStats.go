package drift

import (
	go_drift "driftgo"
	"driftgo/accounts"
	"driftgo/addresses"
	"driftgo/drift/types"
	"driftgo/lib/drift"
	"github.com/gagliardetto/solana-go"
)

type UserStats struct {
	DriftClient               types.IDriftClient
	UserStatsAccountPublicKey solana.PublicKey
	accountSubscriber         accounts.IUserStatsAccountSubscriber
	IsSubscribed              bool
}

func NewUserStats(config UserStatsConfig) *UserStats {
	userStats := &UserStats{
		DriftClient:               config.DriftClient,
		UserStatsAccountPublicKey: config.UserStatsAccountPublicKey,
	}
	if config.AccountSubscription.UsePolling {
		userStats.accountSubscriber = accounts.CreatePollingUserStatsAccountSubscriber(
			config.DriftClient.GetProgram(),
			config.UserStatsAccountPublicKey,
			config.AccountSubscription.AccountLoader,
		)
	} else {
		userStats.accountSubscriber = accounts.CreateGeyserUserStatsAccountSubscriber(
			config.DriftClient.GetProgram(),
			config.UserStatsAccountPublicKey,
			config.AccountSubscription.ResubTimeoutMs,
			config.AccountSubscription.Commitment,
		)
	}
	return userStats
}

func (p *UserStats) GetSubscriber() accounts.IUserStatsAccountSubscriber {
	return p.accountSubscriber
}

func (p *UserStats) Subscribe(
	userStatsAccount *drift.UserStats,
) bool {
	p.IsSubscribed = p.accountSubscriber.Subscribe(userStatsAccount)
	return p.IsSubscribed
}

func (p *UserStats) FetchAccounts() {
	p.accountSubscriber.Fetch()
}

func (p *UserStats) Unsubscribe() {
	p.accountSubscriber.Unsubscribe()
}

func (p *UserStats) GetAccountAndSlot() *accounts.DataAndSlot[*drift.UserStats] {
	return p.accountSubscriber.GetUserStatsAccountAndSlot()
}

func (p *UserStats) GetAccount() *drift.UserStats {
	return p.accountSubscriber.GetUserStatsAccountAndSlot().Data
}

func (p *UserStats) GetReferrerInfo() *go_drift.ReferrerInfo {
	if p.GetAccount().Referrer.IsZero() {
		return nil
	} else {
		return &go_drift.ReferrerInfo{
			Referrer: addresses.GetUserAccountPublicKey(
				p.DriftClient.GetProgram().GetProgramId(),
				p.GetAccount().Referrer,
				0,
			),
			ReferrerStats: addresses.GetUserStatsAccountPublicKey(
				p.DriftClient.GetProgram().GetProgramId(),
				p.GetAccount().Referrer,
			),
		}
	}
}

func (p *UserStats) GetOldestActionTs(account *drift.UserStats) int64 {
	return min(
		account.LastFillerVolume30DTs,
		account.LastMakerVolume30DTs,
		account.LastTakerVolume30DTs,
	)
}

func (p *UserStats) GetPublicKey() solana.PublicKey {
	return p.UserStatsAccountPublicKey
}
