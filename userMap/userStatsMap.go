package userMap

import (
	"driftgo/accounts"
	"driftgo/addresses"
	"driftgo/common"
	driftlib "driftgo/drift"
	"driftgo/drift/types"
	"driftgo/lib/drift"
	"github.com/gagliardetto/solana-go"
	"sync"
)

type UserStatsMap struct {
	userStatsMap      map[string]types.IUserStats
	driftClient       types.IDriftClient
	bulkAccountLoader *accounts.BulkAccountLoader
	mxState           *sync.RWMutex
}

func CreateUserStatsMap(
	driftClient types.IDriftClient,
	bulkAccountLoader *accounts.BulkAccountLoader,
) *UserStatsMap {
	userStatsMap := &UserStatsMap{
		driftClient:  driftClient,
		userStatsMap: make(map[string]types.IUserStats),
		mxState:      new(sync.RWMutex),
	}
	if bulkAccountLoader == nil {
		bulkAccountLoader = accounts.CreateBulkAccountLoader(
			driftClient.GetProgram().GetProvider().GetConnection(),
			driftClient.GetOpts().Commitment,
			0,
		)
	}
	userStatsMap.bulkAccountLoader = bulkAccountLoader
	return userStatsMap
}

func (p *UserStatsMap) Subscribe(authorities []solana.PublicKey) {
	if p.Size() > 0 {
		return
	}
	p.driftClient.Subscribe()
	p.Sync(authorities)
}

func (p *UserStatsMap) AddUserStat(
	authority solana.PublicKey,
	userStatsAccount *drift.UserStats,
	skipFetch bool,
) {
	//fmt.Println("AddUserStat:", authority.String())
	defer p.mxState.Unlock()
	p.mxState.Lock()
	userStat := driftlib.NewUserStats(driftlib.UserStatsConfig{
		AccountSubscription: types.UserStatsSubscriptionConfig{
			UsePolling:    true,
			AccountLoader: p.bulkAccountLoader,
		},
		DriftClient: p.driftClient,
		UserStatsAccountPublicKey: addresses.GetUserStatsAccountPublicKey(
			p.driftClient.GetProgram().GetProgramId(),
			authority,
		),
	})
	if skipFetch {
		userStat.GetSubscriber().(*accounts.PollingUserStatsAccountSubscriber).AddToAccountLoader()
	} else {
		userStat.Subscribe(userStatsAccount)
	}
	p.userStatsMap[authority.String()] = userStat
}

func (p *UserStatsMap) Has(authorityPublicKey string) bool {
	defer p.mxState.RUnlock()
	p.mxState.RLock()
	_, exists := p.userStatsMap[authorityPublicKey]
	return exists
}

func (p *UserStatsMap) Get(authorityPublicKey string) types.IUserStats {
	defer p.mxState.RUnlock()
	p.mxState.RLock()
	userStats, exists := p.userStatsMap[authorityPublicKey]
	if exists {
		return userStats
	}
	return nil
}

func (p *UserStatsMap) MustGet(
	authorityPublicKey string,
) types.IUserStats {
	if !p.Has(authorityPublicKey) {
		p.AddUserStat(
			solana.MPK(authorityPublicKey),
			nil,
			false,
		)
	}
	return p.Get(authorityPublicKey)
}

func (p *UserStatsMap) Values() *common.Generator[types.IUserStats, int] {
	defer p.mxState.RUnlock()
	p.mxState.RLock()
	return common.NewGenerator(func(yield common.YieldFn[types.IUserStats, int]) {
		idx := 0
		for _, user := range p.userStatsMap {
			yield(user, idx)
			idx++
		}
	})
}

func (p *UserStatsMap) Size() int {
	return len(p.userStatsMap)
}

func (p *UserStatsMap) Sync(authorities []solana.PublicKey) {
	for _, authority := range authorities {
		p.AddUserStat(authority, nil, true)
	}
	p.bulkAccountLoader.Load()
}

func (p *UserStatsMap) Unsubscribe() {
	defer p.mxState.Unlock()
	p.mxState.Lock()
	for key, userStats := range p.userStatsMap {
		userStats.Unsubscribe()
		delete(p.userStatsMap, key)
	}
}
