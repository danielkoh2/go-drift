package userMap

import (
	"crypto/md5"
	go_drift "driftgo"
	"driftgo/accounts"
	"driftgo/common"
	dlob "driftgo/dlob"
	dloblib "driftgo/dlob/types"
	"driftgo/drift"
	drifttype "driftgo/drift/types"
	driftlib "driftgo/lib/drift"
	"driftgo/lib/geyser"
	"driftgo/userMap/types"
	"driftgo/utils"
	"fmt"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"sync"
	"time"
)

type UserMap struct {
	userMap     map[string]*accounts.DataAndSlot[drifttype.IUser]
	hashMap     map[string][16]byte
	statusMap   map[string]*types.UserStatus
	driftClient drifttype.IDriftClient
	connection  *geyser.Connection
	//connection                       *rpc.Client
	commitment                       rpc.CommitmentType
	includeIdle                      bool
	disableSyncOnTotalAccountsChange bool
	lastNumberOfSubAccounts          uint64
	subscription                     *GeyserSubscription
	stateAccountUpdateCallback       func(state *driftlib.State)
	decode                           func(string, []byte) *driftlib.User
	mostRecentSlot                   uint64
	synchronizing                    bool
	synchronized                     bool
	mxState                          *sync.RWMutex
	mxHashmap                        *sync.RWMutex
}

func CreateUserMap(config types.UserMapConfig) *UserMap {
	connection := config.Connection
	if connection == nil {
		connection = config.DriftClient.GetProgram().GetProvider().GetGeyserConnection()
	}
	commitment := config.Commitment
	if commitment == "" {
		commitment = config.DriftClient.GetOpts().Commitment
	}
	userMap := &UserMap{
		userMap:                          make(map[string]*accounts.DataAndSlot[drifttype.IUser]),
		hashMap:                          make(map[string][16]byte),
		statusMap:                        make(map[string]*types.UserStatus),
		driftClient:                      config.DriftClient,
		connection:                       connection,
		commitment:                       commitment,
		includeIdle:                      config.IncludeIdle,
		disableSyncOnTotalAccountsChange: config.DisableSyncOnTotalAccountsChange,
		lastNumberOfSubAccounts:          0,
		mostRecentSlot:                   0,
		mxState:                          new(sync.RWMutex),
		mxHashmap:                        new(sync.RWMutex),
	}
	userMap.stateAccountUpdateCallback = func(state *driftlib.State) {
		if state.NumberOfSubAccounts != userMap.lastNumberOfSubAccounts {
			userMap.Sync()
			userMap.lastNumberOfSubAccounts = state.NumberOfSubAccounts
		}
	}
	decodeFn := func(accountName string, buffer []byte) *driftlib.User {
		user := driftlib.User{}
		err := bin.NewBinDecoder(buffer).Decode(&user)
		if err != nil {
			return nil
		}
		return &user
	}
	userMap.decode = decodeFn
	userMap.subscription = CreateGeyserSubscription(
		userMap,
		userMap.commitment,
		config.SkipInitialLoad,
		config.SubscriptionConfig.ResubTimeoutMs,
		userMap.includeIdle,
		decodeFn,
	)
	return userMap
}

func (p *UserMap) Subscribe() {
	//if p.Size() > 0 {
	//	return
	//}

	if !p.disableSyncOnTotalAccountsChange {
		go_drift.EventEmitter().On(
			"stateAccountUpdate",
			func(object ...interface{}) {
				fmt.Printf("stateAccountUpdate: %v\n", object[0])
				p.stateAccountUpdateCallback(object[0].(*driftlib.State))
			},
		)
	}
	fmt.Println("Starting driftclient subscription.")
	p.driftClient.Subscribe()
	p.lastNumberOfSubAccounts = p.driftClient.GetStateAccount().NumberOfSubAccounts
	fmt.Println("Starting userMap subscription.")
	p.subscription.Subscribe()
	fmt.Println("Congratulation! UserMap subscription started.")
}

func (p *UserMap) getOrderHash(buffer []byte) [16]byte {
	if buffer == nil {
		return [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	}
	return md5.Sum(buffer[3072:4264])
}

func (p *UserMap) AddPubkey(
	userAccountPublicKey solana.PublicKey,
	userAccount *driftlib.User,
	slot *uint64,
	accountSubscription *drifttype.UserSubscriptionConfig,
	buffer []byte,
) {
	user := drift.NewUser(drift.UserConfig{
		AccountSubscription: utils.TTM[drifttype.UserSubscriptionConfig](accountSubscription == nil, drifttype.UserSubscriptionConfig{
			UseCustom: true,
			CustomSubscriber: accounts.CreateOneShotUserAccountSubscriber(
				p.driftClient.GetProgram(),
				userAccountPublicKey,
				userAccount,
				slot,
				&p.commitment,
			),
		}, func() drifttype.UserSubscriptionConfig { return *accountSubscription }),
		DriftClient:          p.driftClient,
		UserAccountPublicKey: userAccountPublicKey,
	})
	user.Subscribe(userAccount)
	currentSlot := utils.TTM[uint64](slot == nil, user.GetUserAccountAndSlot().Slot, func() uint64 { return *slot })
	key := userAccountPublicKey.String()
	p.userMap[key] = &accounts.DataAndSlot[drifttype.IUser]{
		Data: user,
		Slot: currentSlot,
	}
	//p.mxHashmap.Lock()
	//p.hashMap[key] = p.getOrderHash(buffer)
	//p.mxHashmap.Unlock()
	//go_drift.EventEmitter().Emit(go_drift.EventLibUserOrderUpdated, key, userAccount, currentSlot)
}

func (p *UserMap) Has(key string) bool {
	_, exists := p.userMap[key]
	return exists
}

func (p *UserMap) Get(key string) drifttype.IUser {
	user, exists := p.userMap[key]
	if exists {
		return user.Data
	}
	return nil
}

func (p *UserMap) GetUserStatus(key string) *types.UserStatus {
	userStatus, exists := p.statusMap[key]
	if exists {
		return userStatus
	}
	return nil
}

func (p *UserMap) GetWithSlot(key string) *accounts.DataAndSlot[drifttype.IUser] {
	userAndSlot, exists := p.userMap[key]
	if exists {
		return userAndSlot
	}
	return nil
}

func (p *UserMap) MustGet(
	key string,
	accountSubscription *drifttype.UserSubscriptionConfig,
) drifttype.IUser {
	if !p.Has(key) {
		p.AddPubkey(
			solana.MPK(key),
			nil,
			nil,
			accountSubscription,
			nil,
		)
	}
	return p.Get(key)
}

func (p *UserMap) MustGetWithSlot(
	key string,
	accountSubscription *drifttype.UserSubscriptionConfig,
) *accounts.DataAndSlot[drifttype.IUser] {
	if !p.Has(key) {
		p.AddPubkey(
			solana.MPK(key),
			nil,
			nil,
			accountSubscription,
			nil,
		)
	}
	return p.GetWithSlot(key)

}

func (p *UserMap) GetUserAuthority(key string) solana.PublicKey {
	user, exists := p.userMap[key]
	if !exists {
		return solana.PublicKey{}
	}
	return user.Data.GetUserAccount().Authority
}

func (p *UserMap) GetDLOB(slot uint64) dloblib.IDLOB {
	dlob := dlob.NewDLOB()
	dlob.InitFromUserMap(p, slot)
	return dlob
}

func (p *UserMap) Values() *common.Generator[drifttype.IUser, int] {
	return common.NewGenerator(func(yield common.YieldFn[drifttype.IUser, int]) {
		//defer p.mxState.RUnlock()
		//p.mxState.RLock()
		idx := 0
		for _, user := range p.userMap {
			if yield(user.Data, idx) {
				break
			}
			idx++
		}
	})
}

func (p *UserMap) ValuesWithSlot() *common.Generator[*accounts.DataAndSlot[drifttype.IUser], int] {
	return common.NewGenerator(func(yield common.YieldFn[*accounts.DataAndSlot[drifttype.IUser], int]) {
		//defer p.mxState.RUnlock()
		//p.mxState.RLock()
		idx := 0
		for _, user := range p.userMap {
			if yield(user, idx) {
				break
			}
			idx++
		}
	})
}

func (p *UserMap) Entries() *common.Generator[drifttype.IUser, string] {
	return common.NewGenerator(func(yield common.YieldFn[drifttype.IUser, string]) {
		//defer p.mxState.RUnlock()
		//p.mxState.RLock()
		for key, user := range p.userMap {
			if yield(user.Data, key) {
				break
			}
		}
	})

}

func (p *UserMap) EntriesWithSlot() *common.Generator[*accounts.DataAndSlot[drifttype.IUser], string] {
	return common.NewGenerator(func(yield common.YieldFn[*accounts.DataAndSlot[drifttype.IUser], string]) {
		//defer p.mxState.RUnlock()
		//p.mxState.RLock()
		for key, user := range p.userMap {
			if yield(user, key) {
				break
			}
		}
	})
}

func (p *UserMap) Size() int {
	return len(p.userMap)
}

func (p *UserMap) GetUniqueAuthorities(
	filterCriteria *types.UserAccountFilterCriteria,
) []solana.PublicKey {
	defer p.mxState.RUnlock()
	p.mxState.RLock()
	userAuths := make(map[solana.PublicKey]bool)
	var userAuthKeys []solana.PublicKey
	for _, user := range p.userMap {
		pass := true
		if filterCriteria != nil && filterCriteria.HasOpenOrders {
			pass = pass && user.Data.GetUserAccount().HasOpenOrder
		}
		if pass {
			authority := user.Data.GetUserAccount().Authority
			_, exists := userAuths[authority]
			if !exists {
				userAuths[user.Data.GetUserAccount().Authority] = true
				userAuthKeys = append(userAuthKeys, authority)
			}
		}
	}
	return userAuthKeys
}

func (p *UserMap) Sync() {
	if p.synchronizing {
		return
	}
	p.synchronizing = true
	defer func() {
		p.synchronizing = false
		if e := recover(); e != nil {
			fmt.Println(e)
		}
	}()
	fmt.Println("Synchronizing userMap")
	filters := []rpc.RPCFilter{go_drift.GetUserFilter()}
	if !p.includeIdle {
		filters = append(filters, go_drift.GetNonIdleUserFiler())
	}
	//connection := p.driftClient.
	//	GetProgram().
	//	GetProvider().
	//	GetConnection()
	//connection := go_drift.Connec
	//response, err := solana2.GetProgramAccountsContextWithOpts(
	//	connection,
	//	context.TODO(),
	//	p.driftClient.GetProgram().GetProgramId(),
	//	&rpc.GetProgramAccountsOpts{
	//		Encoding:   solana.EncodingBase64Zstd,
	//		Commitment: p.commitment,
	//		Filters:    filters,
	//	},
	//)
	response, err := GetTestProgramAccount()

	if err != nil {
		fmt.Println("RPCError", err)
		return
	}

	slot := response.Context.Slot

	p.UpdateLatestSlot(slot)

	p.mxState.Lock()
	accountSet := make(map[string]bool)
	//fmt.Printf("[%s]Synchronizing userMap : %d user loaded\n", time.Now().Format(time.RFC3339), len(response.Value))
	for _, programAccount := range response.Value {
		key := programAccount.Pubkey.String()
		buffer := programAccount.Account.Data.GetBinary()

		accountSet[key] = true
		currAccountWithSlot := p.GetWithSlot(key)
		if currAccountWithSlot != nil {
			if slot >= currAccountWithSlot.Slot {
				var userAccount driftlib.User
				err = bin.NewBinDecoder(buffer).Decode(&userAccount)
				if err != nil {
					continue
				}
				p.UpdateUserAccount(key, &userAccount, slot, "", buffer)
			}
		} else {
			var userAccount driftlib.User
			err = bin.NewBinDecoder(buffer).Decode(&userAccount)
			if err != nil {
				fmt.Printf("Error decoding user account %s\n", key)
				continue
			}
			//fmt.Printf("userAccount after decode: %+v\n", userAccount)
			p.AddPubkey(solana.MPK(key), &userAccount, &slot, nil, buffer)
		}
	}
	p.Entries().Each(func(account drifttype.IUser, key string) bool {
		_, exists := accountSet[key]
		if !exists {
			user := p.Get(key)
			if user != nil {
				user.Unsubscribe()
				delete(p.userMap, key)
			}
		}
		return false
	})
	p.synchronized = true
	fmt.Printf("Synchronizing userMap : %d user loaded\n", len(response.Value))
	p.mxState.Unlock()
	go_drift.EventEmitter().Emit("userMapSynchronized")
}

func (p *UserMap) Unsubscribe() {
	p.subscription.Unsubscribe()

	p.mxState.Lock()
	p.Entries().Each(func(user drifttype.IUser, key string) bool {
		user.Unsubscribe()
		delete(p.userMap, key)
		return false
	})
	p.mxState.Unlock()

	if p.lastNumberOfSubAccounts > 0 {
		if !p.disableSyncOnTotalAccountsChange {
			p.driftClient.GetEventEmitter().Off("stateAccountUpdate")
		}
		p.lastNumberOfSubAccounts = 0
	}
}

func (p *UserMap) UpdateUserAccount(
	key string,
	userAccount *driftlib.User,
	slot uint64,
	txnHash string,
	buffer []byte,
	lock ...bool,
) {
	stateLock := len(lock) > 0 && lock[0]
	defer func() {
		if stateLock {
			p.mxState.Unlock()
		}
	}()
	if stateLock {
		p.mxState.Lock()
	}
	userWithSlot := p.GetWithSlot(key)
	p.UpdateLatestSlot(slot)
	go_drift.EventEmitter().Emit(go_drift.EventLibUserAccountUpdated, key, userAccount, slot, txnHash)
	if userWithSlot != nil {
		if slot >= userWithSlot.Slot {
			orderUpdated := false
			userWithSlot.Data.UpdateData(userAccount, slot)
			userWithSlot.Slot = slot
			p.userMap[key] = &accounts.DataAndSlot[drifttype.IUser]{
				Data:   userWithSlot.Data,
				Slot:   slot,
				Pubkey: solana.MPK(key),
			}
			if orderUpdated {
				go_drift.EventEmitter().Emit(go_drift.EventLibUserOrderUpdated, key, userAccount, slot)
			}
		} else {
			return
		}
	} else {
		p.AddPubkey(solana.MPK(key), userAccount, &slot, nil, buffer)

	}
	if p.updateStatus(key, p.getOrderHash(buffer)) {
		go_drift.EventEmitter().Emit(go_drift.EventLibUserOrderUpdated, key, userAccount, slot, txnHash)
	}
}

func (p *UserMap) updateStatus(userAccountKey string, ordersHash [16]byte) bool {
	defer p.mxHashmap.Unlock()
	p.mxHashmap.Lock()
	userStatus, exists := p.statusMap[userAccountKey]
	nowTs := time.Now().UnixMilli()
	if !exists {
		userStatus = &types.UserStatus{
			LastOrdersHash: ordersHash,
			LastUpdated:    nowTs,
			UpdatedCount:   0,
			UpdateRate:     0,
		}
		p.statusMap[userAccountKey] = userStatus
		return true
	} else {
		orderHashChanged := userStatus.LastOrdersHash != ordersHash
		if orderHashChanged {
			lastUpdated := userStatus.LastUpdated
			duration := nowTs - lastUpdated
			updatedCount := userStatus.UpdatedCount
			userStatus.LastUpdated = nowTs
			if updatedCount < 100 {
				userStatus.UpdatedCount++
			}
			var updateRate int64 = 0
			if updatedCount == 0 {
				updateRate = duration
			} else {
				updateRate = (userStatus.UpdateRate*updatedCount + duration) / (updatedCount + 1)
			}
			userStatus.UpdateRate = updateRate
		}
		return orderHashChanged
	}

}

func (p *UserMap) UpdateLatestSlot(slot uint64) {
	p.mostRecentSlot = max(slot, p.mostRecentSlot)
}

func (p *UserMap) GetSlot() uint64 {
	return p.mostRecentSlot
}
