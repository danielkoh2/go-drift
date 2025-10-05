package drift

import (
	"context"
	drift "driftgo"
	"driftgo/accounts"
	"driftgo/addresses"
	"driftgo/anchor"
	"driftgo/anchor/types"
	"driftgo/connection"
	"driftgo/constants"
	config2 "driftgo/drift/config"
	drifttype "driftgo/drift/types"
	driftlib "driftgo/lib/drift"
	"driftgo/lib/event"
	associatedtokenaccount2 "driftgo/lib/solana/associated-token-account"
	spl_token "driftgo/lib/spl-token"
	"driftgo/math"
	oracles "driftgo/oracles/types"
	"driftgo/tx"
	"driftgo/utils"
	"errors"
	"fmt"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	addresslookuptable "github.com/gagliardetto/solana-go/programs/address-lookup-table"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"math/big"
	"slices"
)

type DriftClient struct {
	drifttype.IDriftClient
	ConnectionManager *connection.Manager
	Rpc               *rpc.Client

	Wallet   drift.IWallet
	Program  types.IProgram
	Provider types.IProvider

	Opts                          drift.ConfirmOptions
	Users                         map[string]*User
	UserStats                     *UserStats
	ActiveSubAccountId            uint16
	UserAccountSubscriptionConfig drifttype.UserSubscriptionConfig
	UserStatsSubscriptionConfig   drifttype.UserStatsSubscriptionConfig
	AccountSubscriber             accounts.IDriftClientAccountSubscriber
	EventEmitter                  *event.EventEmitter
	_isSubscribed                 bool

	TxSender                     tx.ITxSender
	PerpMarketLastSlotCache      map[uint16]uint64
	SpotMarketLastSlotCache      map[uint16]uint64
	MustIncludePerpMarketIndexes map[uint16]bool
	MustIncludeSpotMarketIndexes map[uint16]bool
	Authority                    solana.PublicKey
	MarketLookupTable            solana.PublicKey
	LookupTableAccount           *addresslookuptable.KeyedAddressLookupTable
	IncludeDelegates             bool
	AuthoritySubAccountMap       map[string][]uint16
	SkipLoadUsers                bool
	TxParams                     *drift.TxParams

	statePublicKey            solana.PublicKey
	signerPublicKey           solana.PublicKey
	userStatsAccountPublicKey solana.PublicKey
}

func (p *DriftClient) GetProgram() types.IProgram {
	return p.Program
}

func (p *DriftClient) GetOpts() *drift.ConfirmOptions {
	return &p.Opts
}

func (p *DriftClient) IsSubscribed() bool {
	return p._isSubscribed && p.AccountSubscriber.Subscribed()
}

func (p *DriftClient) SetSubscribed(val bool) {
	p._isSubscribed = val
}

func CreateDriftClient(config drifttype.DriftClientConfig) *DriftClient {
	driftClient := &DriftClient{
		ConnectionManager:            config.Connection,
		Wallet:                       config.Wallet,
		Opts:                         config.Opts,
		ActiveSubAccountId:           config.ActiveSubAccountId,
		TxSender:                     config.TxSender,
		PerpMarketLastSlotCache:      make(map[uint16]uint64),
		SpotMarketLastSlotCache:      make(map[uint16]uint64),
		MustIncludePerpMarketIndexes: make(map[uint16]bool),
		MustIncludeSpotMarketIndexes: make(map[uint16]bool),
		Users:                        make(map[string]*User),
		MarketLookupTable:            config.MarketLookupTable,
		IncludeDelegates:             config.IncludeDelegates,
		SkipLoadUsers:                config.SkipLoadUsers,
	}
	driftClient.Rpc = driftClient.ConnectionManager.GetRpc()
	driftClient.Provider = anchor.CreateAnchorProvider(
		driftClient.Wallet,
		driftClient.Authority,
		driftClient.Opts,
		config.Connection,
	)
	driftClient.Program = anchor.CreateProgram(
		config.ProgramId,
		driftClient.Provider,
	)
	if config.Authority.IsZero() {
		driftClient.Authority = config.Wallet.GetPublicKey()
	} else {
		driftClient.Authority = config.Authority
	}
	if config.TxParams == nil {
		driftClient.TxParams = &drift.TxParams{
			BaseTxParams: drift.BaseTxParams{
				ComputeUnits:      600000,
				ComputeUnitsPrice: 0,
			},
		}
	} else {
		driftClient.TxParams = config.TxParams
	}
	if config.IncludeDelegates && len(config.SubAccountIds) > 0 {
		panic("Can only pass one of includeDelegates or subAccountIds. If you want to specify subaccount ids for multiple authorities, pass authoritySubaccountMap instead")
	}
	if len(config.AuthoritySubAccountMap) > 0 && len(config.SubAccountIds) > 0 {
		panic("Can only pass one of authoritySubaccountMap or subAccountIds")
	}
	if len(config.AuthoritySubAccountMap) > 0 && config.IncludeDelegates {
		panic("Can only pass one of authoritySubaccountMap or includeDelegates")
	}
	if len(config.AuthoritySubAccountMap) > 0 {
		driftClient.AuthoritySubAccountMap = config.AuthoritySubAccountMap
	} else {
		driftClient.AuthoritySubAccountMap = make(map[string][]uint16)
		if len(config.SubAccountIds) > 0 {
			driftClient.AuthoritySubAccountMap[driftClient.Authority.String()] = config.SubAccountIds
		}
	}
	driftClient.UserAccountSubscriptionConfig.ResubTimeoutMs = config.AccountSubscription.ResubTimeoutMs
	driftClient.UserAccountSubscriptionConfig.Commitment = config.AccountSubscription.Commitment

	driftClient.UserStatsSubscriptionConfig.ResubTimeoutMs = config.AccountSubscription.ResubTimeoutMs
	driftClient.UserStatsSubscriptionConfig.Commitment = config.AccountSubscription.Commitment

	if config.UserStats {
		driftClient.UserStats = NewUserStats(UserStatsConfig{
			DriftClient: driftClient,
			UserStatsAccountPublicKey: addresses.GetUserStatsAccountPublicKey(
				driftClient.Program.GetProgramId(),
				driftClient.Authority,
			),
			AccountSubscription: drifttype.UserStatsSubscriptionConfig{
				ResubTimeoutMs: config.AccountSubscription.ResubTimeoutMs,
				Commitment:     config.AccountSubscription.Commitment,
			},
		})
	}

	driftClient.MarketLookupTable = config.MarketLookupTable
	if config.Env != config2.DriftEnvNone && config.MarketLookupTable.IsZero() {
		driftClient.MarketLookupTable = solana.MPK(config2.DriftConfigs[config.Env].MARKET_LOOKUP_TABLE)
	}
	noMarketsAndOracleSpecified := len(config.PerpMarketIndexes) == 0 && len(config.SpotMarketIndexes) == 0 && len(config.OracleInfos) == 0
	driftClient.AccountSubscriber = accounts.CreateGeyserDriftClientAccountSubscriber(
		driftClient.Program,
		config.PerpMarketIndexes,
		config.SpotMarketIndexes,
		config.OracleInfos,
		noMarketsAndOracleSpecified,
		config.AccountSubscription.ResubTimeoutMs,
		config.AccountSubscription.Commitment,
	)
	driftClient.EventEmitter = drift.EventEmitter()
	if config.TxSender != nil {
		driftClient.TxSender = config.TxSender
	} else {
		driftClient.TxSender = tx.CreateBaseTxSender(
			driftClient.Rpc,
			solana.Wallet{PrivateKey: driftClient.Wallet.GetPrivateKey()},
			&driftClient.Opts,
			0,
		)
	}
	return driftClient
}

func (p *DriftClient) GetUserMapKey(subAccountId uint16, authority solana.PublicKey) string {
	return fmt.Sprintf("%d_%s", subAccountId, authority.String())
}

func (p *DriftClient) createUser(
	subAccountId uint16,
	accountSubscriptionConfig drifttype.UserSubscriptionConfig,
	authority solana.PublicKey,
) *User {
	if authority.IsZero() {
		authority = p.Authority
	}
	userAccountPublicKey := addresses.GetUserAccountPublicKey(
		p.Program.GetProgramId(),
		authority,
		subAccountId,
	)
	return NewUser(UserConfig{
		AccountSubscription:  accountSubscriptionConfig,
		DriftClient:          p,
		UserAccountPublicKey: userAccountPublicKey,
		EventEmitter:         p.EventEmitter,
	})
}

func (p *DriftClient) Subscribe() bool {

	p.addAndSubscribeToUsers()
	p._isSubscribed = p.AccountSubscriber.Subscribe()
	return p._isSubscribed
}

func (p *DriftClient) subscribeUsers() bool {
	for _, user := range p.Users {
		user.Subscribe(nil)
	}
	return true
}

func (p *DriftClient) addAndSubscribeToUsers() bool {
	// save the rpc calls if driftclient is initialized without a real wallet
	if p.SkipLoadUsers {
		return true
	}

	result := true

	if len(p.AuthoritySubAccountMap) > 0 {
		firstSubAccountId := uint16(0)
		var firstAuthority string
		for key, subAccountIds := range p.AuthoritySubAccountMap {
			if len(firstAuthority) == 0 {
				firstAuthority = key
			}
			for _, subAccountId := range subAccountIds {
				if firstSubAccountId == 0 {
					firstSubAccountId = subAccountId
				}
				pkey := solana.MPK(key)
				result = result && p.AddUser(subAccountId, &pkey, nil)
			}
		}

		if p.ActiveSubAccountId == 0 {
			pkey := solana.MPK(firstAuthority)
			p.SwitchActiveUser(firstSubAccountId, &pkey)
		}
	} else {
		var userAccounts []*driftlib.User
		var delegatedAccounts []*driftlib.User

		userAccounts = p.GetUserAccountsForAuthority(p.Wallet.GetPublicKey())

		if p.IncludeDelegates {
			delegatedAccounts = p.GetUserAccountsForDelegate(p.Wallet.GetPublicKey())
		}

		allAccounts := append(userAccounts, delegatedAccounts...)
		firstSubAccountId := uint16(0)
		var firstAuthority string
		for _, account := range allAccounts {
			if len(firstAuthority) == 0 {
				firstAuthority = account.Authority.String()
				firstSubAccountId = account.SubAccountId
			}
			p.AddUser(account.SubAccountId, &account.Authority, account)
		}

		if p.ActiveSubAccountId == 0 {
			pkey := solana.MPK(firstAuthority)
			p.SwitchActiveUser(firstSubAccountId, &pkey)
		}
	}
	return result
}

//func (p *DriftClient) initializeUserAccount(
//	subAccountId uint16,
//	name string,
//	referrerInfo *drift.ReferrerInfo,
//) (solana.Signature, solana.PublicKey) {
//
//}

func (p *DriftClient) GetActiveSubAccountId() uint16 {
	return p.ActiveSubAccountId
}

func (p *DriftClient) GetUserAccountsForDelegate(delegate solana.PublicKey) []*driftlib.User {
	programAccounts := p.Program.GetAccounts(driftlib.User{}, "User").All(
		[]rpc.RPCFilter{
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: 40,
					Bytes:  delegate.Bytes(),
				},
			},
		})
	slices.SortFunc(programAccounts, func(a, b interface{}) int {
		accountA := a.(*drift.ProgramAccount[driftlib.User])
		accountB := a.(*drift.ProgramAccount[driftlib.User])
		if accountA.Account.SubAccountId < accountB.Account.SubAccountId {
			return 1
		} else if accountA.Account.SubAccountId > accountB.Account.SubAccountId {
			return -1
		}
		return 0
	})
	var accounts []*driftlib.User
	for _, account := range programAccounts {
		accounts = append(accounts, account.(*drift.ProgramAccount[driftlib.User]).Account)
	}
	return accounts

}

func (p *DriftClient) GetUserAccountsForAuthority(authority solana.PublicKey) []*driftlib.User {
	programAccounts := p.Program.GetAccounts(driftlib.User{}, "User").All(
		[]rpc.RPCFilter{
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: 8,
					Bytes:  authority.Bytes(),
				},
			},
		})
	var accounts []*driftlib.User
	for _, account := range programAccounts {
		accounts = append(accounts, account.(*driftlib.User))
	}
	return accounts
}

func (p *DriftClient) FetchAccounts() {
	for _, user := range p.Users {
		user.FetchAccounts()
	}
	p.AccountSubscriber.Fetch()
	if p.UserStats != nil {
		p.UserStats.FetchAccounts()
	}
}

func (p *DriftClient) Unsubscribe() {
	p.unsubscribeUsers()
	if p.UserStats != nil {
		p.UserStats.Unsubscribe()
	}
	p.AccountSubscriber.Unsubscribe()
	p._isSubscribed = false
}

func (p *DriftClient) unsubscribeUsers() {
	for _, user := range p.Users {
		user.Unsubscribe()
	}
}

func (p *DriftClient) GetStatePublicKey() solana.PublicKey {
	if !p.statePublicKey.IsZero() {
		return p.statePublicKey
	}
	p.statePublicKey = addresses.GetDriftStateAccountPublicKey(
		p.Program.GetProgramId(),
	)
	return p.statePublicKey
}

func (p *DriftClient) GetSignerPublicKey() solana.PublicKey {
	if !p.signerPublicKey.IsZero() {
		return p.signerPublicKey
	}
	p.signerPublicKey = addresses.GetDriftSignerPublicKey(
		p.Program.GetProgramId(),
	)
	return p.signerPublicKey
}

func (p *DriftClient) GetStateAccount() *driftlib.State {
	return p.AccountSubscriber.GetStateAccountAndSlot().Data
}

func (p *DriftClient) ForceGetStateAccount() *driftlib.State {
	p.AccountSubscriber.Fetch()
	return p.AccountSubscriber.GetStateAccountAndSlot().Data
}

func (p *DriftClient) GetPerpMarketAccount(marketIndex uint16) *driftlib.PerpMarket {
	dataAndSlot := p.AccountSubscriber.GetMarketAccountAndSlot(marketIndex)
	if dataAndSlot == nil {
		return nil
	}
	return dataAndSlot.Data
}

func (p *DriftClient) ForceGetPerpMarketAccount(marketIndex uint16) *driftlib.PerpMarket {
	p.AccountSubscriber.Fetch()
	dataAndSlot := p.AccountSubscriber.GetMarketAccountAndSlot(marketIndex)
	if dataAndSlot == nil {
		return nil
	}
	return dataAndSlot.Data
}

func (p *DriftClient) GetPerpMarketAccounts() []*driftlib.PerpMarket {
	var perpMarketAccounts []*driftlib.PerpMarket
	for _, dataAndSlot := range p.AccountSubscriber.GetMarketAccountsAndSlots() {
		if dataAndSlot != nil && dataAndSlot.Data != nil {
			perpMarketAccounts = append(perpMarketAccounts, dataAndSlot.Data)
		}
	}
	return perpMarketAccounts
}

func (p *DriftClient) GetSpotMarketAccount(marketIndex uint16) *driftlib.SpotMarket {
	dataAndSlot := p.AccountSubscriber.GetSpotMarketAccountAndSlot(marketIndex)
	if dataAndSlot == nil {
		return nil
	}
	return dataAndSlot.Data
}

func (p *DriftClient) ForceGetSpotMarketAccount(marketIndex uint16) *driftlib.SpotMarket {
	p.AccountSubscriber.Fetch()
	dataAndSlot := p.AccountSubscriber.GetSpotMarketAccountAndSlot(marketIndex)
	if dataAndSlot == nil {
		return nil
	}
	return dataAndSlot.Data
}

func (p *DriftClient) GetSpotMarketAccounts() []*driftlib.SpotMarket {
	var spotMarketAccounts []*driftlib.SpotMarket
	for _, dataAndSlot := range p.AccountSubscriber.GetSpotMarketAccountsAndSlots() {
		if dataAndSlot != nil && dataAndSlot.Data != nil {
			spotMarketAccounts = append(spotMarketAccounts, dataAndSlot.Data)
		}
	}
	return spotMarketAccounts
}

func (p *DriftClient) GetQuoteSpotMarketAccount() *driftlib.SpotMarket {
	return p.AccountSubscriber.GetSpotMarketAccountAndSlot(
		constants.QUOTE_SPOT_MARKET_INDEX,
	).Data
}

func (p *DriftClient) GetOraclePriceDataAndSlot(
	oraclePublicKey solana.PublicKey,
) *accounts.DataAndSlot[*oracles.OraclePriceData] {
	return p.AccountSubscriber.GetOraclePriceDataAndSlot(oraclePublicKey)
}

func (p *DriftClient) GetSerumV3FulfillmentConfig(serumMarket solana.PublicKey) *driftlib.SerumV3FulfillmentConfig {
	address := addresses.GetSerumFulfillmentConfigPublicKey(
		p.Program.GetProgramId(),
		serumMarket,
	)
	account := p.Program.GetAccounts(
		driftlib.SerumV3FulfillmentConfig{},
		"serumV3FulfillmentConfig",
	).Fetch(
		address,
		rpc.CommitmentProcessed,
	)
	return account.(*driftlib.SerumV3FulfillmentConfig)
}

func (p *DriftClient) GetPhoenixV1FulfillmentConfig(phoenixMarket solana.PublicKey) *driftlib.PhoenixV1FulfillmentConfig {
	address := addresses.GetPhoenixFulfillmentConfigPublicKey(
		p.Program.GetProgramId(),
		phoenixMarket,
	)
	account := p.Program.GetAccounts(
		driftlib.PhoenixV1FulfillmentConfig{},
		"phoenixV1FulfillmentConfig",
	).
		Fetch(
			address,
			rpc.CommitmentProcessed,
		)
	return account.(*driftlib.PhoenixV1FulfillmentConfig)
}

func (p *DriftClient) FetchMarketLookupTableAccount() *addresslookuptable.KeyedAddressLookupTable {
	if p.LookupTableAccount != nil {
		return p.LookupTableAccount
	}
	if p.MarketLookupTable.IsZero() {
		fmt.Println("Market lookup table address not set")
		return nil
	}
	account, err := p.Rpc.GetAccountInfo(context.TODO(), p.MarketLookupTable)
	if err != nil {
		return nil
	}
	lookupTableAccount := addresslookuptable.NewKeyedAddressLookupTable(p.MarketLookupTable)
	err = bin.NewBinDecoder(account.GetBinary()).Decode(&lookupTableAccount.State)
	if err != nil {
		return nil
	}
	p.LookupTableAccount = lookupTableAccount
	return p.LookupTableAccount
}

func (p *DriftClient) SwitchActiveUser(subAccountId uint16, authority *solana.PublicKey) {
	authorityChanged := authority != nil && !p.Authority.Equals(*authority)

	p.ActiveSubAccountId = subAccountId
	p.Authority = utils.TTM[solana.PublicKey](authority == nil || authority.IsZero(), p.Authority, func() solana.PublicKey { return *authority })
	p.userStatsAccountPublicKey = addresses.GetUserStatsAccountPublicKey(
		p.Program.GetProgramId(),
		p.Authority,
	)

	/* If changing the user authority ie switching from delegate to non-delegate account, need to re-subscribe to the user stats account */
	if authorityChanged {
		if p.UserStats != nil && p.UserStats.IsSubscribed {
			p.UserStats.Unsubscribe()
		}

		p.UserStats = NewUserStats(UserStatsConfig{
			DriftClient:               p,
			UserStatsAccountPublicKey: p.userStatsAccountPublicKey,
			AccountSubscription: drifttype.UserStatsSubscriptionConfig{
				ResubTimeoutMs: p.UserAccountSubscriptionConfig.ResubTimeoutMs,
				Commitment:     p.UserAccountSubscriptionConfig.Commitment,
			},
		})
		p.UserStats.Subscribe(nil)
	}
}

func (p *DriftClient) AddUser(
	subAccountId uint16,
	authority *solana.PublicKey,
	userAccount *driftlib.User,
) bool {
	if authority == nil {
		authority = &p.Authority
	}
	userKey := p.GetUserMapKey(subAccountId, *authority)
	user, exists := p.Users[userKey]
	if exists && user.Subscribed() {
		return true
	}

	user = p.createUser(
		subAccountId,
		p.UserAccountSubscriptionConfig,
		*authority,
	)
	result := user.Subscribe(userAccount)
	if result {
		p.Users[userKey] = user
		return true
	} else {
		return false
	}
}

//func (p *DriftClient) FetchAllUserAccounts(bool) []drift.ProgramAccount[driftlib.User] {
//
//}

func (p *DriftClient) GetUser(subAccountId *uint16, authority *solana.PublicKey) drifttype.IUser {
	if subAccountId == nil {
		subAccountId = &p.ActiveSubAccountId
	}
	if authority == nil || authority.IsZero() {
		authority = &p.Authority
	}
	userMapKey := p.GetUserMapKey(*subAccountId, *authority)
	user, exists := p.Users[userMapKey]
	if !exists {
		panic(fmt.Sprintf("DriftClient has no user for user id %s", userMapKey))
	}
	return user
}

func (p *DriftClient) HasUser(subAccountId *uint16, authority *solana.PublicKey) bool {
	if subAccountId == nil {
		subAccountId = &p.ActiveSubAccountId
	}
	if authority == nil {
		authority = &p.Authority
	}
	userMapKey := p.GetUserMapKey(*subAccountId, *authority)
	_, exists := p.Users[userMapKey]
	return exists
}

func (p *DriftClient) GetUsers() []drifttype.IUser {
	var users []drifttype.IUser
	var delegates []drifttype.IUser
	for _, user := range p.Users {
		if user.GetUserAccount().Authority.Equals(p.Wallet.GetPublicKey()) {
			users = append(users, user)
		} else {
			delegates = append(delegates, user)
		}
	}
	return append(users, delegates...)
}

func (p *DriftClient) GetUserStats() drifttype.IUserStats {
	return p.UserStats
}

func (p *DriftClient) GetUserStatsAccountPublicKey() solana.PublicKey {
	if p.userStatsAccountPublicKey.IsZero() {
		p.userStatsAccountPublicKey = addresses.GetUserStatsAccountPublicKey(
			p.Program.GetProgramId(),
			p.Authority,
		)
	}
	return p.userStatsAccountPublicKey
}
func (p *DriftClient) GetUserAccountPublicKey(
	subAccountId *uint16,
	authority *solana.PublicKey,
) solana.PublicKey {
	return p.GetUser(subAccountId, authority).GetUserAccountPublicKey()
}

func (p *DriftClient) GetUserAccount(
	subAccountId *uint16,
	authority *solana.PublicKey,
) *driftlib.User {
	return p.GetUser(subAccountId, authority).GetUserAccount()
}

func (p *DriftClient) ForceGetUserAccount(subAccountId *uint16) *driftlib.User {
	user := p.GetUser(subAccountId, nil)
	user.FetchAccounts()
	return user.GetUserAccount()
}

func (p *DriftClient) GetUserAccountAndSlot(subAccountId *uint16) *accounts.DataAndSlot[*driftlib.User] {
	return p.GetUser(subAccountId, nil).GetUserAccountAndSlot()
}

func (p *DriftClient) GetSpotPosition(
	marketIndex uint16,
	subAccountId *uint16,
) *driftlib.SpotPosition {
	user := p.GetUserAccount(subAccountId, nil)
	for idx := 0; idx < len(user.SpotPositions); idx++ {
		position := &user.SpotPositions[idx]
		if position.MarketIndex == marketIndex {
			return position
		}
	}
	return nil
}

func (p *DriftClient) GetQuoteAssetTokenAmount() *big.Int {
	return p.GetTokenAmount(constants.QUOTE_SPOT_MARKET_INDEX)
}

func (p *DriftClient) GetTokenAmount(marketIndex uint16) *big.Int {
	spotPosition := p.GetSpotPosition(marketIndex, nil)
	if spotPosition == nil {
		return utils.BN(0)
	}
	spotMarket := p.GetSpotMarketAccount(marketIndex)
	return math.GetSignedTokenAmount(
		math.GetTokenAmount(
			utils.BN(spotPosition.ScaledBalance),
			spotMarket,
			spotPosition.BalanceType,
		),
		spotPosition.BalanceType,
	)
}

func (p *DriftClient) GetRemainingAccounts(
	params *drifttype.RemainingAccountParams,
) solana.AccountMetaSlice {
	oracleAccountMap, spotMarketAccountMap, perpMarketAccountMap := p.getRemainingAccountMapsForUsers(params.UserAccounts)

	if params.UseMarketLastSlotCache {
		lastUserSlot := p.GetUserAccountAndSlot(nil).Slot
		for marketIndex, slot := range p.PerpMarketLastSlotCache {
			if slot > lastUserSlot {
				p.addPerpMarketToRemainingAccountMaps(
					marketIndex,
					false,
					&oracleAccountMap,
					&spotMarketAccountMap,
					&perpMarketAccountMap,
				)
			} else {
				delete(p.PerpMarketLastSlotCache, marketIndex)
			}
		}
		for marketIndex, slot := range p.SpotMarketLastSlotCache {
			if slot > lastUserSlot {
				p.addSpotMarketToRemainingAccountMaps(
					marketIndex,
					false,
					&oracleAccountMap,
					&spotMarketAccountMap,
				)
			} else {
				delete(p.SpotMarketLastSlotCache, marketIndex)
			}
		}
	}

	if len(params.ReadablePerpMarketIndex) > 0 {
		readablePerpMarketIndexes := params.ReadablePerpMarketIndex
		for _, marketIndex := range readablePerpMarketIndexes {
			p.addPerpMarketToRemainingAccountMaps(
				marketIndex,
				false,
				&oracleAccountMap,
				&spotMarketAccountMap,
				&perpMarketAccountMap,
			)
		}
	}

	for perpMarketIndex, _ := range p.MustIncludePerpMarketIndexes {
		p.addPerpMarketToRemainingAccountMaps(
			perpMarketIndex,
			false,
			&oracleAccountMap,
			&spotMarketAccountMap,
			&perpMarketAccountMap,
		)
	}

	if len(params.ReadableSpotMarketIndex) > 0 {
		for _, readableSpotMarketIndex := range params.ReadableSpotMarketIndex {
			p.addSpotMarketToRemainingAccountMaps(
				readableSpotMarketIndex,
				false,
				&oracleAccountMap,
				&spotMarketAccountMap,
			)
		}
	}
	for spotMarketIndex, _ := range p.MustIncludeSpotMarketIndexes {
		p.addSpotMarketToRemainingAccountMaps(
			spotMarketIndex,
			false,
			&oracleAccountMap,
			&spotMarketAccountMap,
		)
	}

	if len(params.WritablePerpMarketIndexes) > 0 {
		for _, writablePerpMarketIndex := range params.WritablePerpMarketIndexes {
			p.addPerpMarketToRemainingAccountMaps(
				writablePerpMarketIndex,
				true,
				&oracleAccountMap,
				&spotMarketAccountMap,
				&perpMarketAccountMap,
			)
		}
	}

	if len(params.WritableSpotMarketIndexes) > 0 {
		for _, writableSpotMarketIndex := range params.WritableSpotMarketIndexes {
			p.addSpotMarketToRemainingAccountMaps(
				writableSpotMarketIndex,
				true,
				&oracleAccountMap,
				&spotMarketAccountMap,
			)
		}
	}
	var remainingAccounts solana.AccountMetaSlice
	for _, accountMeta := range oracleAccountMap {
		remainingAccounts = append(remainingAccounts, accountMeta)
	}
	for _, accountMeta := range spotMarketAccountMap {
		remainingAccounts = append(remainingAccounts, accountMeta)
	}
	for _, accountMeta := range perpMarketAccountMap {
		remainingAccounts = append(remainingAccounts, accountMeta)
	}
	return remainingAccounts
}

func (p *DriftClient) addPerpMarketToRemainingAccountMaps(
	marketIndex uint16,
	writable bool,
	oracleAccountMap *map[string]*solana.AccountMeta,
	spotMarketAccountMap *map[uint16]*solana.AccountMeta,
	perpMarketAccountMap *map[uint16]*solana.AccountMeta,
) bool {
	perpMarketAccount := p.GetPerpMarketAccount(marketIndex)
	if perpMarketAccount == nil {
		return false
	}
	(*perpMarketAccountMap)[marketIndex] = &solana.AccountMeta{
		PublicKey:  perpMarketAccount.Pubkey,
		IsSigner:   false,
		IsWritable: writable,
	}
	oracleWritable := writable && perpMarketAccount.Amm.OracleSource == driftlib.OracleSource_Prelaunch

	(*oracleAccountMap)[perpMarketAccount.Amm.Oracle.String()] = &solana.AccountMeta{
		PublicKey:  perpMarketAccount.Amm.Oracle,
		IsSigner:   false,
		IsWritable: oracleWritable,
	}
	p.addSpotMarketToRemainingAccountMaps(
		perpMarketAccount.QuoteSpotMarketIndex,
		false,
		oracleAccountMap,
		spotMarketAccountMap,
	)
	return true
}

func (p *DriftClient) addSpotMarketToRemainingAccountMaps(
	marketIndex uint16,
	writable bool,
	oracleAccountMap *map[string]*solana.AccountMeta,
	spotMarketAccountMap *map[uint16]*solana.AccountMeta,
) {
	spotMarketAccount := p.GetSpotMarketAccount(marketIndex)
	(*spotMarketAccountMap)[spotMarketAccount.MarketIndex] = &solana.AccountMeta{
		PublicKey:  spotMarketAccount.Pubkey,
		IsSigner:   false,
		IsWritable: writable,
	}

	if !spotMarketAccount.Oracle.IsZero() {
		(*oracleAccountMap)[spotMarketAccount.Oracle.String()] = &solana.AccountMeta{
			PublicKey:  spotMarketAccount.Oracle,
			IsSigner:   false,
			IsWritable: false,
		}
	}
}

func (p *DriftClient) getRemainingAccountMapsForUsers(
	userAccounts []*driftlib.User,
) (
	map[string]*solana.AccountMeta,
	map[uint16]*solana.AccountMeta,
	map[uint16]*solana.AccountMeta,
) {
	oracleAccountMap := make(map[string]*solana.AccountMeta)
	spotMarketAccountMap := make(map[uint16]*solana.AccountMeta)
	perpMarketAccountMap := make(map[uint16]*solana.AccountMeta)

	for _, userAccount := range userAccounts {
		for idx := 0; idx < len(userAccount.SpotPositions); idx++ {
			spotPosition := &userAccount.SpotPositions[idx]
			if !math.IsSpotPositionAvailable(spotPosition) {
				p.addSpotMarketToRemainingAccountMaps(spotPosition.MarketIndex, false, &oracleAccountMap, &spotMarketAccountMap)
				if spotPosition.OpenAsks != 0 || spotPosition.OpenBids != 0 {
					p.addSpotMarketToRemainingAccountMaps(
						constants.QUOTE_SPOT_MARKET_INDEX,
						false,
						&oracleAccountMap,
						&spotMarketAccountMap,
					)
				}
			}
		}
		for idx := 0; idx < len(userAccount.PerpPositions); idx++ {
			perpPosition := &userAccount.PerpPositions[idx]
			if !math.PositionIsAvailable(perpPosition) {
				p.addPerpMarketToRemainingAccountMaps(perpPosition.MarketIndex, false, &oracleAccountMap, &spotMarketAccountMap, &perpMarketAccountMap)
			}
		}
	}

	return oracleAccountMap, spotMarketAccountMap, perpMarketAccountMap
}

func (p *DriftClient) GetOrder(orderId uint32, subAccountId *uint16) *driftlib.Order {
	user := p.GetUserAccount(subAccountId, nil)
	if user == nil {
		return nil
	}
	for idx := 0; idx < len(user.Orders); idx++ {
		order := &user.Orders[idx]
		if order.OrderId == orderId {
			return order
		}
	}
	return nil
}

func (p *DriftClient) GetOrderByUserId(userOrderId uint8, subAccountId *uint16) *driftlib.Order {
	user := p.GetUserAccount(subAccountId, nil)
	if user == nil {
		return nil
	}
	for idx := 0; idx < len(user.Orders); idx++ {
		order := &user.Orders[idx]
		if order.UserOrderId == userOrderId {
			return order
		}
	}
	return nil
}

func (p *DriftClient) getProcessedTransactionParams(
	instructions []solana.Instruction,
	txParams drift.BaseTxParams,
	lookupTables []addresslookuptable.KeyedAddressLookupTable,
	txParamProcessingParams *drift.ProcessingTxParams,
) drift.BaseTxParams {
	return tx.ProcessTxParams(
		&tx.TransactionProps{
			Instructions: instructions,
			TxParams:     txParams,
			LookupTables: lookupTables,
		},
		func(updatedTxParams *tx.TransactionProps) *solana.Transaction {
			return p.BuildTransaction(
				updatedTxParams.Instructions,
				&drift.TxParams{
					BaseTxParams:       updatedTxParams.TxParams,
					ProcessingTxParams: drift.ProcessingTxParams{},
				},
				updatedTxParams.LookupTables)
		},
		txParamProcessingParams,
		p.Rpc,
	)

}
func (p *DriftClient) BuildTransaction(
	instructions []solana.Instruction,
	txParams *drift.TxParams,
	lookupTables []addresslookuptable.KeyedAddressLookupTable,
) *solana.Transaction {
	baseTxParams := drift.BaseTxParams{
		ComputeUnits:      utils.TTM[uint64](txParams != nil, func() uint64 { return txParams.ComputeUnits }, p.TxParams.ComputeUnits),
		ComputeUnitsPrice: utils.TTM[uint64](txParams != nil, func() uint64 { return txParams.ComputeUnitsPrice }, p.TxParams.ComputeUnitsPrice),
	}

	if txParams != nil && txParams.UseSimulatedComputeUnits != nil && *txParams.UseSimulatedComputeUnits {
		splitTxParams := drift.TxParams{
			BaseTxParams: drift.BaseTxParams{
				ComputeUnits:      txParams.ComputeUnits,
				ComputeUnitsPrice: txParams.ComputeUnitsPrice,
			},
			ProcessingTxParams: drift.ProcessingTxParams{
				UseSimulatedComputeUnits:                     txParams.UseSimulatedComputeUnits,
				ComputeUnitsBufferMultiplier:                 txParams.ComputeUnitsBufferMultiplier,
				UseSimulateComputeUnitsForCUPriceCalculation: txParams.UseSimulateComputeUnitsForCUPriceCalculation,
				GetCUPriceFromComputeUnits:                   txParams.GetCUPriceFromComputeUnits,
			},
		}
		processedTxParams := p.getProcessedTransactionParams(
			instructions,
			splitTxParams.BaseTxParams,
			lookupTables,
			&splitTxParams.ProcessingTxParams,
		)
		baseTxParams.ComputeUnitsPrice = processedTxParams.ComputeUnitsPrice
		baseTxParams.ComputeUnits = processedTxParams.ComputeUnits
	}

	// # Create Tx Instructions
	allIx := []solana.Instruction{}
	computeUnits := baseTxParams.ComputeUnits
	if computeUnits != 200_000 {
		allIx = append(allIx, computebudget.NewSetComputeUnitLimitInstructionBuilder().SetUnits(uint32(computeUnits)).Build())
	}

	computeUnitsPrice := baseTxParams.ComputeUnitsPrice

	if computeUnitsPrice != 0 {
		allIx = append(allIx, computebudget.NewSetComputeUnitPriceInstructionBuilder().SetMicroLamports(computeUnitsPrice).Build())
	}

	allIx = append(allIx, instructions...)

	latestBlockHashAndContext, _ := p.Rpc.GetLatestBlockhash(context.TODO(), p.Opts.Commitment)

	// # Create and return Transaction
	marketLookupTable := p.FetchMarketLookupTableAccount()
	if len(lookupTables) > 0 {
		lookupTables = append(lookupTables, *marketLookupTable)
	} else {
		lookupTables = []addresslookuptable.KeyedAddressLookupTable{*marketLookupTable}
	}

	addressTables := make(map[solana.PublicKey]solana.PublicKeySlice)
	for _, lookupTable := range lookupTables {
		addressTables[lookupTable.Key] = lookupTable.State.Addresses
	}
	transactionBuilder := solana.NewTransactionBuilder().
		SetFeePayer(p.Program.GetProvider().GetWallet().GetPublicKey()).
		SetRecentBlockHash(latestBlockHashAndContext.Value.Blockhash).
		WithOpt(solana.TransactionAddressTables(addressTables))
	for _, instruction := range allIx {
		transactionBuilder.AddInstruction(instruction)
	}
	transaction, _ := transactionBuilder.Build()

	return transaction
}

func (p *DriftClient) SendTransaction(
	tx *solana.Transaction,
	opts *drift.ConfirmOptions,
	preSigned bool,
) (*tx.TxSigAndSlot, error) {
	txSigAndSlot, err := p.TxSender.Send(tx, opts, preSigned, nil)
	if err != nil {
		return nil, err
	}
	return txSigAndSlot, nil
}

func (p *DriftClient) GetAssociatedTokenAccount(marketIndex uint16, useNative bool) solana.PublicKey {
	spotMarket := p.GetSpotMarketAccount(marketIndex)
	if useNative && spotMarket.Mint.Equals(constants.WRAPPED_SOL_MINT) {
		return p.Wallet.GetPublicKey()
	}
	mint := spotMarket.Mint
	associatedToken, _, _ := solana.FindAssociatedTokenAddress(p.Wallet.GetPublicKey(), mint)
	return associatedToken
}

func (p *DriftClient) CreateAssociatedTokenAccountIdempotentInstruction(
	account solana.PublicKey,
	payer solana.PublicKey,
	owner solana.PublicKey,
	mint solana.PublicKey,
) solana.Instruction {
	return solana.NewInstruction(spl_token.TOKEN_PROGRAM_ID, solana.AccountMetaSlice{
		{PublicKey: payer, IsSigner: true, IsWritable: true},
		{PublicKey: account, IsSigner: false, IsWritable: true},
		{PublicKey: owner, IsSigner: false, IsWritable: false},
		{PublicKey: mint, IsSigner: false, IsWritable: false},
		{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		{PublicKey: spl_token.TOKEN_PROGRAM_ID, IsSigner: false, IsWritable: false},
	}, []byte{1})
}

func (p *DriftClient) CheckIfAccountExists(account solana.PublicKey) bool {
	accountInfo, err := p.Rpc.GetAccountInfo(context.TODO(), account)
	if err != nil || accountInfo == nil || accountInfo.Value == nil {
		return false
	}
	return true
}

func (p *DriftClient) GetWrappedSolAccountCreationIxs(
	amount *big.Int,
	includeRent *bool,
) ([]solana.Instruction, solana.PublicKey) {
	authority := p.Wallet.GetPublicKey()

	// Generate a random seed for wrappedSolAccount
	seed := utils.GenerateIdentity()

	// Calculate a publicKey that will be controlled by the authority.
	wrappedSolAccount, _ := solana.CreateWithSeed(
		authority,
		seed,
		spl_token.TOKEN_PROGRAM_ID,
	)

	var ixs []solana.Instruction

	rentSpaceLamports := utils.BN(solana.LAMPORTS_PER_SOL / 100)

	lamports := utils.TT(
		includeRent != nil && *includeRent,
		utils.AddX(amount, rentSpaceLamports),
		rentSpaceLamports,
	)

	ixs = append(ixs,
		system.NewCreateAccountWithSeedInstructionBuilder().
			SetSeed(seed).
			SetBase(authority).
			SetFundingAccount(authority).
			SetCreatedAccount(wrappedSolAccount).
			SetLamports(lamports.Uint64()).
			SetSpace(165).
			SetOwner(spl_token.TOKEN_PROGRAM_ID).
			Build(),
		token.NewInitializeAccountInstructionBuilder().
			SetAccount(wrappedSolAccount).
			SetMintAccount(constants.WRAPPED_SOL_MINT).
			SetOwnerAccount(authority).
			SetSysVarRentPubkeyAccount(solana.SysVarRentPubkey).Build(),
	)
	return ixs, wrappedSolAccount
}

func (p *DriftClient) GetAssociatedTokenAccountCreationIx(
	tokenMintAddress solana.PublicKey,
	associatedTokenAddress solana.PublicKey,
) (solana.Instruction, error) {
	return associatedtokenaccount2.NewCreateInstructionBuilder().
		SetPayer(p.Wallet.GetPublicKey()).
		SetMint(tokenMintAddress).
		SetAssociatedToken(associatedTokenAddress).
		SetWallet(p.Wallet.GetPublicKey()).ValidateAndBuild()
}

func (p *DriftClient) GetWithdrawalIxs(
	amount *big.Int,
	marketIndex uint16,
	associatedTokenAddress solana.PublicKey,
	reduceOnly bool,
	subAccountId *uint16,
) ([]solana.Instruction, error) {
	var withdrawIxs []solana.Instruction

	spotMarketAccount := p.GetSpotMarketAccount(marketIndex)

	isSolMarket := spotMarketAccount.Mint.Equals(constants.WRAPPED_SOL_MINT)

	authority := p.Wallet.GetPublicKey()

	createWSOLTokenAccount := isSolMarket && associatedTokenAddress.Equals(authority)

	if createWSOLTokenAccount {
		ixs, pubkey := p.GetWrappedSolAccountCreationIxs(
			amount,
			utils.NewPtr(false),
		)

		associatedTokenAddress = pubkey
		withdrawIxs = append(withdrawIxs, ixs...)
	} else {
		accountExists := p.CheckIfAccountExists(
			associatedTokenAddress,
		)
		if !accountExists {
			createAssociatedTokenAccountIx, _ := p.GetAssociatedTokenAccountCreationIx(
				spotMarketAccount.Mint,
				associatedTokenAddress,
			)
			withdrawIxs = append(withdrawIxs, createAssociatedTokenAccountIx)
		}
	}

	withdrawCollateralIx, _ := p.GetWithdrawIx(
		amount,
		spotMarketAccount.MarketIndex,
		associatedTokenAddress,
		reduceOnly,
		subAccountId,
	)

	withdrawIxs = append(withdrawIxs, withdrawCollateralIx)

	// Close the wrapped sol account at the end of the transaction
	if createWSOLTokenAccount {
		closeAccountIx, _ := token.NewCloseAccountInstructionBuilder().
			SetAccount(associatedTokenAddress).
			SetDestinationAccount(authority).
			SetOwnerAccount(authority).
			ValidateAndBuild()
		withdrawIxs = append(withdrawIxs, closeAccountIx)
	}

	return withdrawIxs, nil
}

func (p *DriftClient) Withdraw(
	amount *big.Int,
	marketIndex uint16,
	associatedTokenAddress solana.PublicKey,
	reduceOnly bool,
	subAccountId *uint16,
	txParams *drift.TxParams,
) (solana.Signature, error) {
	withdrawIxs, err := p.GetWithdrawalIxs(
		amount,
		marketIndex,
		associatedTokenAddress,
		reduceOnly,
		subAccountId,
	)
	if err != nil {
		return solana.Signature{}, err
	}

	tx := p.BuildTransaction(withdrawIxs, utils.TT(txParams == nil, p.TxParams, txParams), []addresslookuptable.KeyedAddressLookupTable{})

	txSig, err := p.SendTransaction(tx, &p.Opts, false)
	if err != nil {
		return solana.Signature{}, err
	}
	p.SpotMarketLastSlotCache[marketIndex] = txSig.Slot
	return txSig.TxSig, nil
}

func (p *DriftClient) WithdrawAllDustPositions(*uint16, *drift.TxParams, func(int)) (solana.Signature, error) {
	return solana.Signature{}, nil
}

//func (p *DriftClient) GetWithdrawIx(
//	amount *big.Int,
//	marketIndex uint16,
//	userTokenAccount solana.PublicKey,
//	reduceOnly bool,
//	subAccountId *uint16,
//) (solana.Instruction, error) {
//	user := p.GetUserAccountPublicKey(subAccountId, nil)
//
//	spotMarketAccount := p.GetSpotMarketAccount(marketIndex)
//	remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//		UserAccounts:              []*driftlib.User{p.GetUserAccount(subAccountId, nil)},
//		UseMarketLastSlotCache:    true,
//		WritableSpotMarketIndexes: []uint16{marketIndex},
//		ReadableSpotMarketIndex:   []uint16{constants.QUOTE_SPOT_MARKET_INDEX},
//	})
//	builder := driftlib.NewWithdrawInstructionBuilder().
//		SetMarketIndex(marketIndex).
//		SetAmount(amount.Uint64()).
//		SetReduceOnly(reduceOnly).
//		SetStateAccount(p.GetStatePublicKey()).
//		SetSpotMarketVaultAccount(spotMarketAccount.Vault).
//		SetDriftSignerAccount(p.GetSignerPublicKey()).
//		SetUserAccount(user).
//		SetUserStatsAccount(p.GetUserStatsAccountPublicKey()).
//		SetUserTokenAccountAccount(userTokenAccount).
//		SetAuthorityAccount(p.Wallet.GetPublicKey()).
//		SetTokenProgramAccount(spl_token.TOKEN_PROGRAM_ID)
//	//builder, _ := driftlib.NewWithdrawInstruction(
//	//	marketIndex,
//	//	amount.Uint64(),
//	//	reduceOnly,
//	//	p.GetStatePublicKey(),
//	//	user,
//	//	p.GetUserStatsAccountPublicKey(),
//	//	p.Wallet.GetPublicKey(),
//	//	spotMarketAccount.Vault,
//	//	p.GetSignerPublicKey(),
//	//	userTokenAccount,
//	//	spl_token.TOKEN_PROGRAM_ID,
//	//)
//	for _, account := range remainingAccounts {
//		builder.Append(account)
//	}
//	return builder.ValidateAndBuild()
//}

func (p *DriftClient) PlacePerpOrder(
	orderParams drift.OptionalOrderParams,
	txParams *drift.TxParams,
	subAccountId *uint16,
) (solana.Signature, error) {
	return solana.Signature{}, nil
}

//func (p *DriftClient) GetPlacePerpOrderIx(
//	orderParams drift.OptionalOrderParams,
//	subAccountId *uint16,
//) (solana.Instruction, error) {
//	nOrderParams := GetOrderParams(orderParams, &drift.OptionalOrderParams{
//		MarketType: utils.NewPtr(driftlib.MarketType_Perp),
//	})
//	remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//		UserAccounts:            []*driftlib.User{p.GetUserAccount(subAccountId, nil)},
//		ReadablePerpMarketIndex: []uint16{nOrderParams.MarketIndex},
//		UseMarketLastSlotCache:  true,
//	})
//	placeOrders := driftlib.NewPlacePerpOrderInstructionBuilder().
//		SetParams(nOrderParams).
//		SetStateAccount(p.GetStatePublicKey()).
//		SetUserAccount(p.GetUserAccountPublicKey(subAccountId, nil)).
//		SetAuthorityAccount(p.Wallet.GetPublicKey())
//
//	for _, account := range remainingAccounts {
//		placeOrders.Append(account)
//	}
//	return placeOrders.ValidateAndBuild()
//}

func (p *DriftClient) CancelOrders(
	marketType *driftlib.MarketType,
	marketIndex *uint16,
	direction *driftlib.PositionDirection,
	txParams *drift.TxParams,
	subAccountId *uint16,
) (solana.Signature, error) {
	ix, err := p.GetCancelOrdersIx(
		marketType,
		marketIndex,
		direction,
		subAccountId,
	)
	if err != nil {
		return solana.Signature{}, errors.New("can not get cancel orders ix")
	}
	tx := p.BuildTransaction(
		[]solana.Instruction{ix},
		txParams,
		[]addresslookuptable.KeyedAddressLookupTable{},
	)
	ret, err := p.SendTransaction(tx, &p.Opts, false)
	if err == nil {
		return ret.TxSig, nil
	}
	return solana.Signature{}, err
}

//func (p *DriftClient) GetCancelOrdersIx(
//	marketType *driftlib.MarketType,
//	marketIndex *uint16,
//	direction *driftlib.PositionDirection,
//	subAccountId *uint16,
//) (solana.Instruction, error) {
//	//) (*driftlib.Instruction, error) {
//	user := p.GetUserAccountPublicKey(subAccountId, nil)
//	builder := driftlib.NewCancelOrdersInstructionBuilder()
//	var readablePerpMarketIndex []uint16
//	var readableSpotMarketIndex []uint16
//	if marketIndex != nil {
//		builder.SetMarketIndex(*marketIndex)
//		if marketType != nil && *marketType == driftlib.MarketType_Perp {
//			readablePerpMarketIndex = []uint16{*marketIndex}
//		} else if marketType != nil && *marketType == driftlib.MarketType_Spot {
//			readableSpotMarketIndex = []uint16{*marketIndex}
//		}
//	}
//	builder.SetStateAccount(p.GetStatePublicKey()).
//		SetUserAccount(user).
//		SetAuthorityAccount(p.Wallet.GetPublicKey())
//	if marketType != nil {
//		builder.SetMarketType(*marketType)
//	}
//	if direction != nil {
//		builder.SetDirection(*direction)
//	}
//
//	remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//		UserAccounts:            []*driftlib.User{p.GetUserAccount(subAccountId, nil)},
//		ReadablePerpMarketIndex: readablePerpMarketIndex,
//		ReadableSpotMarketIndex: readableSpotMarketIndex,
//		UseMarketLastSlotCache:  true,
//	})
//	for _, account := range remainingAccounts {
//		builder.Append(account)
//	}
//	return builder.ValidateAndBuild()
//}

func (p *DriftClient) FillPerpOrder(
	userAccountPublicKey solana.PublicKey,
	userAccount *driftlib.User,
	order *driftlib.Order,
	makerInfo []*drift.MakerInfo,
	referrerInfo *drift.ReferrerInfo,
	txParams *drift.TxParams,
	fillerSubAccountId *uint16,
) (solana.Signature, error) {
	ix := p.GetFillPerpOrderIx(
		userAccountPublicKey,
		userAccount,
		order,
		makerInfo,
		referrerInfo,
		fillerSubAccountId,
	)
	if ix == nil {
		return solana.Signature{}, errors.New("can not get fill perp order ix")
	}
	tx := p.BuildTransaction(
		[]solana.Instruction{ix},
		txParams,
		[]addresslookuptable.KeyedAddressLookupTable{},
	)
	ret, err := p.SendTransaction(tx, &p.Opts, false)
	if err == nil {
		return ret.TxSig, nil
	}
	return solana.Signature{}, err
}

//func (p *DriftClient) GetFillPerpOrderIx(
//	userAccountPublicKey solana.PublicKey,
//	userAccount *driftlib.User,
//	order *driftlib.Order,
//	makerInfo []*drift.MakerInfo,
//	referrerInfo *drift.ReferrerInfo,
//	fillerSubAccountId *uint16,
//) solana.Instruction {
//	if order == nil {
//		return nil
//	}
//	userStatsPublicKey := addresses.GetUserStatsAccountPublicKey(
//		p.Program.GetProgramId(),
//		userAccount.Authority,
//	)
//
//	filler := p.GetUserAccountPublicKey(fillerSubAccountId, nil)
//	fillerStatsPublicKey := p.GetUserStatsAccountPublicKey()
//	var marketIndex uint16
//	if order != nil {
//		marketIndex = order.MarketIndex
//	} else {
//		for i := 0; i < len(userAccount.Orders); i++ {
//			if userAccount.Orders[i].OrderId == userAccount.NextOrderId-1 {
//				marketIndex = userAccount.Orders[i].MarketIndex
//				break
//			}
//		}
//	}
//	var userAccounts = []*driftlib.User{userAccount}
//	for _, maker := range makerInfo {
//		userAccounts = append(userAccounts, maker.MakerUserAccount)
//	}
//	remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//		UserAccounts:              userAccounts,
//		WritablePerpMarketIndexes: []uint16{marketIndex},
//	})
//
//	for _, maker := range makerInfo {
//		remainingAccounts = append(remainingAccounts, &solana.AccountMeta{
//			PublicKey:  maker.Maker,
//			IsWritable: true,
//			IsSigner:   false,
//		}, &solana.AccountMeta{
//			PublicKey:  maker.MakerStats,
//			IsWritable: true,
//			IsSigner:   false,
//		})
//	}
//
//	if referrerInfo != nil {
//		referrerIsMaker := false
//		for _, maker := range makerInfo {
//			if maker.Maker.Equals(referrerInfo.Referrer) {
//				referrerIsMaker = true
//				break
//			}
//		}
//		if !referrerIsMaker {
//			remainingAccounts = append(remainingAccounts, &solana.AccountMeta{
//				PublicKey:  referrerInfo.Referrer,
//				IsWritable: true,
//				IsSigner:   false,
//			}, &solana.AccountMeta{
//				PublicKey:  referrerInfo.ReferrerStats,
//				IsWritable: true,
//				IsSigner:   false,
//			})
//		}
//	}
//	orderId := order.OrderId
//	FillPerpOrder := driftlib.NewFillPerpOrderInstructionBuilder().
//		SetOrderId(orderId).
//		SetStateAccount(p.GetStatePublicKey()).
//		SetFillerAccount(filler).
//		SetFillerStatsAccount(fillerStatsPublicKey).
//		SetUserStatsAccount(userStatsPublicKey).
//		SetUserAccount(userAccountPublicKey).
//		SetAuthorityAccount(p.Wallet.GetPublicKey())
//	for _, account := range remainingAccounts {
//		FillPerpOrder.Append(account)
//	}
//	instruction, err := FillPerpOrder.ValidateAndBuild()
//
//	if err != nil {
//		return nil
//	}
//	return instruction
//}

//func (p *DriftClient) GetRevertFillIx(fillerPublicKey *solana.PublicKey) solana.Instruction {
//	filler := utils.TTM[solana.PublicKey](fillerPublicKey == nil, p.GetUserAccountPublicKey(nil, nil), func() solana.PublicKey { return *fillerPublicKey })
//	fillerStatsPublicKey := p.GetUserStatsAccountPublicKey()
//
//	instruction, err := driftlib.NewRevertFillInstructionBuilder().
//		SetStateAccount(p.GetStatePublicKey()).
//		SetFillerAccount(filler).
//		SetFillerStatsAccount(fillerStatsPublicKey).
//		SetAuthorityAccount(p.Wallet.GetPublicKey()).
//		ValidateAndBuild()
//	if err != nil {
//		return nil
//	}
//	return instruction
//}

func (p *DriftClient) PlaceSpotOrder(orderParams drift.OptionalOrderParams, txParams *drift.TxParams, subAccountId *uint16) (solana.Signature, error) {
	ix, err := p.GetPlaceSpotOrderIx(
		orderParams,
		subAccountId,
	)
	if err != nil {
		return solana.Signature{}, errors.New("can not get place spot order ix")
	}
	tx := p.BuildTransaction(
		[]solana.Instruction{ix},
		txParams,
		[]addresslookuptable.KeyedAddressLookupTable{},
	)
	ret, err := p.SendTransaction(tx, &p.Opts, false)
	if err == nil {
		return ret.TxSig, nil
	}
	return solana.Signature{}, err
}

//func (p *DriftClient) GetPlaceSpotOrderIx(orderParams drift.OptionalOrderParams, subAccountId *uint16) (solana.Instruction, error) {
//	nOrderParams := GetOrderParams(orderParams, &drift.OptionalOrderParams{
//		MarketType: utils.NewPtr(driftlib.MarketType_Spot),
//	})
//	userAccountPublicKey := p.GetUserAccountPublicKey(subAccountId, nil)
//	remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//		UserAccounts:           []*driftlib.User{p.GetUserAccount(subAccountId, nil)},
//		UseMarketLastSlotCache: true,
//		ReadableSpotMarketIndex: []uint16{
//			nOrderParams.MarketIndex,
//			constants.QUOTE_SPOT_MARKET_INDEX,
//		},
//	})
//	builder := driftlib.NewPlaceSpotOrderInstructionBuilder().
//		SetParams(nOrderParams).
//		SetStateAccount(p.GetStatePublicKey()).
//		SetUserAccount(userAccountPublicKey).
//		SetAuthorityAccount(p.Wallet.GetPublicKey())
//	for _, account := range remainingAccounts {
//		builder.Append(account)
//	}
//	return builder.ValidateAndBuild()
//}

func (p *DriftClient) TriggerOrder(
	userAccountPublicKey solana.PublicKey,
	userAccount *driftlib.User,
	order *driftlib.Order,
	txParams *drift.TxParams,
	fillerPublicKey *solana.PublicKey,
) (solana.Signature, error) {
	ix := p.GetTriggerOrderIx(
		userAccountPublicKey,
		userAccount,
		order,
		fillerPublicKey,
	)
	if ix == nil {
		return solana.Signature{}, errors.New("can not get trigger order ix")
	}
	tx := p.BuildTransaction(
		[]solana.Instruction{ix},
		txParams,
		[]addresslookuptable.KeyedAddressLookupTable{},
	)
	ret, err := p.SendTransaction(tx, &p.Opts, false)
	if err == nil {
		return ret.TxSig, nil
	}
	return solana.Signature{}, err
}

//func (p *DriftClient) GetTriggerOrderIx(
//	userAccountPublicKey solana.PublicKey,
//	userAccount *driftlib.User,
//	order *driftlib.Order,
//	fillerPublicKey *solana.PublicKey,
//) solana.Instruction {
//	filler := utils.TTM[solana.PublicKey](fillerPublicKey == nil, p.GetUserAccountPublicKey(nil, nil), func() solana.PublicKey { return *fillerPublicKey })
//
//	remainingAccountsParams := &drifttype.RemainingAccountParams{
//		UserAccounts:              []*driftlib.User{userAccount},
//		WritablePerpMarketIndexes: utils.TT(order.MarketType == driftlib.MarketType_Perp, []uint16{order.MarketIndex}, []uint16{}),
//		WritableSpotMarketIndexes: utils.TT(order.MarketType == driftlib.MarketType_Spot, []uint16{order.MarketIndex, constants.QUOTE_SPOT_MARKET_INDEX}, []uint16{}),
//	}
//
//	remainingAccounts := p.GetRemainingAccounts(remainingAccountsParams)
//	orderId := order.OrderId
//	triggerOrder := driftlib.NewTriggerOrderInstructionBuilder().
//		SetOrderId(orderId).
//		SetStateAccount(p.GetStatePublicKey()).
//		SetFillerAccount(filler).
//		SetUserAccount(userAccountPublicKey).
//		SetAuthorityAccount(p.Wallet.GetPublicKey())
//	for _, account := range remainingAccounts {
//		triggerOrder.Append(account)
//	}
//	instruction, err := triggerOrder.ValidateAndBuild()
//	if err != nil {
//		return nil
//	}
//	return instruction
//}

func (p *DriftClient) FillSpotOrder(
	userAccountPublicKey solana.PublicKey,
	user *driftlib.User,
	order *driftlib.Order,
	fulfillmentConfig *drift.FulfillmentConfig,
	makerInfo []*drift.MakerInfo,
	referrerInfo *drift.ReferrerInfo,
	txParams *drift.TxParams,
) (solana.Signature, error) {
	ix, err := p.GetFillSpotOrderIx(
		userAccountPublicKey,
		user,
		order,
		fulfillmentConfig,
		makerInfo,
		referrerInfo,
		nil,
	)
	if err != nil {
		return solana.Signature{}, err
	}
	tx := p.BuildTransaction(
		[]solana.Instruction{ix},
		txParams,
		[]addresslookuptable.KeyedAddressLookupTable{},
	)
	ret, err := p.SendTransaction(tx, &p.Opts, false)
	if err == nil {
		return ret.TxSig, nil
	}
	return solana.Signature{}, err
}

//func (p *DriftClient) GetFillSpotOrderIx(
//	userAccountPublicKey solana.PublicKey,
//	userAccount *driftlib.User,
//	order *driftlib.Order,
//	fulfillmentConfig *drift.FulfillmentConfig,
//	makerInfo []*drift.MakerInfo,
//	referrerInfo *drift.ReferrerInfo,
//	fillerPublicKey *solana.PublicKey,
//) (solana.Instruction, error) {
//	var fulfillmentType *driftlib.SpotFulfillmentType = nil
//	if fulfillmentConfig != nil {
//		if fulfillmentConfig.SerumV3FulfillmentConfig != nil {
//			fulfillmentType = &fulfillmentConfig.SerumV3FulfillmentConfig.FulfillmentType
//		} else if fulfillmentConfig.PhoenixV1FulfillmentConfig != nil {
//			fulfillmentType = &fulfillmentConfig.PhoenixV1FulfillmentConfig.FulfillmentType
//		}
//	}
//	userStatsPublicKey := addresses.GetUserStatsAccountPublicKey(
//		p.Program.GetProgramId(),
//		userAccount.Authority,
//	)
//
//	filler := utils.TTM[solana.PublicKey](fillerPublicKey == nil, p.GetUserAccountPublicKey(nil, nil), func() solana.PublicKey { return *fillerPublicKey })
//	fillerStatsPublicKey := p.GetUserStatsAccountPublicKey()
//
//	var marketIndex uint16
//	if order != nil {
//		marketIndex = order.MarketIndex
//	} else {
//		for i := 0; i < len(userAccount.Orders); i++ {
//			if userAccount.Orders[i].OrderId == userAccount.NextOrderId-1 {
//				marketIndex = userAccount.Orders[i].MarketIndex
//				break
//			}
//		}
//	}
//
//	userAccounts := []*driftlib.User{userAccount}
//	for _, maker := range makerInfo {
//		userAccounts = append(userAccounts, maker.MakerUserAccount)
//	}
//	remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//		UserAccounts:              userAccounts,
//		WritableSpotMarketIndexes: []uint16{marketIndex, constants.QUOTE_SPOT_MARKET_INDEX},
//	})
//
//	for _, maker := range makerInfo {
//		remainingAccounts = append(remainingAccounts, []*solana.AccountMeta{
//			{
//				PublicKey:  maker.Maker,
//				IsWritable: true,
//				IsSigner:   false,
//			},
//			{
//				PublicKey:  maker.MakerStats,
//				IsWritable: true,
//				IsSigner:   false,
//			},
//		}...)
//	}
//
//	orderId := order.OrderId
//
//	remainingAccounts = append(remainingAccounts, p.getSpotFulfillmentAccounts(
//		marketIndex,
//		fulfillmentConfig,
//	)...)
//	fillSpotOrder := driftlib.NewFillSpotOrderInstructionBuilder().SetOrderId(orderId)
//	if fulfillmentType != nil {
//		fillSpotOrder.SetFulfillmentType(*fulfillmentType)
//	}
//	fillSpotOrder.
//		SetStateAccount(p.GetStatePublicKey()).
//		SetFillerAccount(filler).
//		SetFillerStatsAccount(fillerStatsPublicKey).
//		SetUserAccount(userAccountPublicKey).
//		SetUserStatsAccount(userStatsPublicKey).
//		SetAuthorityAccount(p.Wallet.GetPublicKey())
//	for _, account := range remainingAccounts {
//		fillSpotOrder.Append(account)
//	}
//	return fillSpotOrder.ValidateAndBuild()
//}

//func (p *DriftClient) GetJupiterSwapIxV6(
//	jupiterClient *jupiter.JupiterClient,
//	outMarketIndex uint16,
//	inMarketIndex uint16,
//	outAssociatedTokenAccount *solana.PublicKey,
//	inAssociatedTokenAccount *solana.PublicKey,
//	amount *big.Int,
//	slippageBps *int,
//	swapMode *jupiter2.SwapMode,
//	onlyDirectRoutes *bool,
//	quote *jupiter2.QuoteResponse,
//	reduceOnly *driftlib.SwapReduceOnly,
//	userAccountPublicKey *solana.PublicKey,
//) ([]solana.Instruction, []addresslookuptable.KeyedAddressLookupTable, error) {
//	outMarket := p.GetSpotMarketAccount(outMarketIndex)
//	inMarket := p.GetSpotMarketAccount(inMarketIndex)
//
//	if quote == nil {
//		fetchQuote, err := jupiterClient.GetQuote(
//			inMarket.Mint,
//			outMarket.Mint,
//			amount,
//			nil,
//			slippageBps,
//			(*jupiter2.GetQuoteParamsSwapMode)(swapMode),
//			onlyDirectRoutes,
//			[]string{},
//		)
//		if err != nil {
//			return nil, nil, err
//		}
//		quote = fetchQuote
//	}
//
//	transaction, err := jupiterClient.GetSwap(quote, p.Provider.GetWallet().GetPublicKey(), slippageBps)
//	if err != nil {
//		return nil, nil, err
//	}
//
//	transactionMessage, lookupTables := jupiterClient.GetTransactionMessageAndLookupTables(transaction)
//
//	jupiterInstructions := jupiterClient.GetJupiterInstructions(transactionMessage, inMarket.Mint, outMarket.Mint)
//
//	var preInstructions []solana.Instruction
//	if outAssociatedTokenAccount == nil {
//		outAssociatedTokenAccount = utils.NewPtr(p.GetAssociatedTokenAccount(
//			outMarket.MarketIndex,
//			false,
//		))
//		accountInfo, err := p.Rpc.GetAccountInfo(context.TODO(), *outAssociatedTokenAccount)
//		if err != nil || accountInfo == nil {
//			preInstructions = append(preInstructions, p.CreateAssociatedTokenAccountIdempotentInstruction(
//				*outAssociatedTokenAccount,
//				p.Provider.GetWallet().GetPublicKey(),
//				p.Provider.GetWallet().GetPublicKey(),
//				outMarket.Mint,
//			))
//		}
//	}
//
//	if inAssociatedTokenAccount == nil {
//		inAssociatedTokenAccount = utils.NewPtr(p.GetAssociatedTokenAccount(
//			inMarket.MarketIndex,
//			false,
//		))
//		accountInfo, err := p.Rpc.GetAccountInfo(context.TODO(), *inAssociatedTokenAccount)
//		if err != nil || accountInfo == nil {
//			preInstructions = append(preInstructions, p.CreateAssociatedTokenAccountIdempotentInstruction(
//				*inAssociatedTokenAccount,
//				p.Provider.GetWallet().GetPublicKey(),
//				p.Provider.GetWallet().GetPublicKey(),
//				inMarket.Mint,
//			))
//		}
//	}
//
//	inAmount, _ := strconv.ParseInt(quote.InAmount, 10, 64)
//	beginSwapIx, endSwapIx, err := p.GetSwapIx(
//		outMarketIndex,
//		inMarketIndex,
//		utils.BN(inAmount),
//		*inAssociatedTokenAccount,
//		*outAssociatedTokenAccount,
//		nil,
//		reduceOnly,
//		userAccountPublicKey,
//	)
//	if err != nil {
//		return nil, nil, err
//	}
//	ixs := append(preInstructions, beginSwapIx)
//	ixs = append(ixs, jupiterInstructions...)
//	ixs = append(ixs, endSwapIx)
//	return ixs, lookupTables, nil
//}

// GetSwapIx
/**
 * Get the drift begin_swap and end_swap instructions
 *
 * @param outMarketIndex the market index of the token you're buying
 * @param inMarketIndex the market index of the token you're selling
 * @param amountIn the amount of the token to sell
 * @param inTokenAccount the token account to move the tokens being sold
 * @param outTokenAccount the token account to receive the tokens being bought
 * @param limitPrice the limit price of the swap
 * @param reduceOnly
 * @param userAccountPublicKey optional, specify a custom userAccountPublicKey to use instead of getting the current user account; can be helpful if the account is being created within the current tx
 */
//func (p *DriftClient) GetSwapIx(
//	outMarketIndex uint16,
//	inMarketIndex uint16,
//	amountIn *big.Int,
//	inTokenAccount solana.PublicKey,
//	outTokenAccount solana.PublicKey,
//	limitPrice *big.Int,
//	reduceOnly *driftlib.SwapReduceOnly,
//	userAccountPublicKey *solana.PublicKey,
//) (solana.Instruction, solana.Instruction, error) {
//	userAccountPublicKeyToUse := utils.TTM[solana.PublicKey](
//		userAccountPublicKey == nil,
//		p.GetUserAccountPublicKey(nil, nil),
//		*userAccountPublicKey,
//	)
//
//	var userAccounts []*driftlib.User
//	if p.HasUser(nil, nil) && p.GetUser(nil, nil).GetUserAccountAndSlot() != nil {
//		userAccounts = append(userAccounts, p.GetUser(nil, nil).GetUserAccountAndSlot().Data)
//	}
//
//	remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//		UserAccounts:              userAccounts,
//		WritableSpotMarketIndexes: []uint16{outMarketIndex, inMarketIndex},
//		ReadableSpotMarketIndex:   []uint16{constants.QUOTE_SPOT_MARKET_INDEX},
//	})
//
//	outSpotMarket := p.GetSpotMarketAccount(outMarketIndex)
//	inSpotMarket := p.GetSpotMarketAccount(inMarketIndex)
//
//	beginSwapIxBuilder := driftlib.NewBeginSwapInstructionBuilder().
//		SetInMarketIndex(inMarketIndex).
//		SetOutMarketIndex(outMarketIndex).
//		SetAmountIn(amountIn.Uint64()).
//		SetStateAccount(p.GetStatePublicKey()).
//		SetUserAccount(userAccountPublicKeyToUse).
//		SetUserStatsAccount(p.GetUserStatsAccountPublicKey()).
//		SetAuthorityAccount(p.Wallet.GetPublicKey()).
//		SetOutSpotMarketVaultAccount(outSpotMarket.Vault).
//		SetInSpotMarketVaultAccount(inSpotMarket.Vault).
//		SetInTokenAccountAccount(inTokenAccount).
//		SetOutTokenAccountAccount(outTokenAccount).
//		SetTokenProgramAccount(spl_token.TOKEN_PROGRAM_ID).
//		SetDriftSignerAccount(p.GetStateAccount().Signer).
//		SetInstructionsAccount(solana.SysVarInstructionsPubkey)
//	endSwapIxBuilder := driftlib.NewEndSwapInstructionBuilder().
//		SetInMarketIndex(inMarketIndex).
//		SetOutMarketIndex(outMarketIndex).
//		SetStateAccount(p.GetStatePublicKey()).
//		SetUserAccount(userAccountPublicKeyToUse).
//		SetUserStatsAccount(p.GetUserStatsAccountPublicKey()).
//		SetAuthorityAccount(p.Wallet.GetPublicKey()).
//		SetOutSpotMarketVaultAccount(outSpotMarket.Vault).
//		SetInSpotMarketVaultAccount(inSpotMarket.Vault).
//		SetInTokenAccountAccount(inTokenAccount).
//		SetOutTokenAccountAccount(outTokenAccount).
//		SetTokenProgramAccount(spl_token.TOKEN_PROGRAM_ID).
//		SetDriftSignerAccount(p.GetStateAccount().Signer).
//		SetInstructionsAccount(solana.SysVarInstructionsPubkey)
//	if limitPrice != nil {
//		endSwapIxBuilder.SetLimitPrice(limitPrice.Uint64())
//	}
//	if reduceOnly != nil {
//		endSwapIxBuilder.SetReduceOnly(*reduceOnly)
//	}
//	for _, account := range remainingAccounts {
//		beginSwapIxBuilder.Append(account)
//		endSwapIxBuilder.Append(account)
//	}
//	beginSwapIx, err := beginSwapIxBuilder.ValidateAndBuild()
//	if err != nil {
//		return nil, nil, err
//	}
//	endSwapIx, err := endSwapIxBuilder.ValidateAndBuild()
//	if err != nil {
//		return nil, nil, err
//	}
//	return beginSwapIx, endSwapIx, nil
//}

func (p *DriftClient) ForceCancelOrders(
	userAccountPublicKey solana.PublicKey,
	userAccount *driftlib.User,
	txParams *drift.TxParams,
	fillerPublicKey *solana.PublicKey,
) (solana.Signature, error) {
	ix := p.GetForceCancelOrdersIx(
		userAccountPublicKey,
		userAccount,
		fillerPublicKey,
	)
	if ix == nil {
		return solana.Signature{}, errors.New("can not get force cancel orders ix")
	}
	tx := p.BuildTransaction(
		[]solana.Instruction{ix},
		txParams,
		[]addresslookuptable.KeyedAddressLookupTable{},
	)
	ret, err := p.SendTransaction(tx, &p.Opts, false)
	if err == nil {
		return ret.TxSig, nil
	}
	return solana.Signature{}, err
}

//func (p *DriftClient) GetForceCancelOrdersIx(
//	userAccountPublicKey solana.PublicKey,
//	userAccount *driftlib.User,
//	fillerPublicKey *solana.PublicKey,
//) solana.Instruction {
//	filler := utils.TTM[solana.PublicKey](
//		fillerPublicKey == nil,
//		p.GetUserAccountPublicKey(nil, nil),
//		func() solana.PublicKey {
//			return *fillerPublicKey
//		},
//	)
//	remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//		UserAccounts:              []*driftlib.User{userAccount},
//		WritableSpotMarketIndexes: []uint16{constants.QUOTE_SPOT_MARKET_INDEX},
//	})
//	forceCancelOrders := driftlib.NewForceCancelOrdersInstructionBuilder().
//		SetStateAccount(p.GetStatePublicKey()).
//		SetFillerAccount(filler).
//		SetUserAccount(userAccountPublicKey).
//		SetAuthorityAccount(p.Wallet.GetPublicKey())
//	for _, account := range remainingAccounts {
//		forceCancelOrders.Append(account)
//	}
//	instruction, err := forceCancelOrders.ValidateAndBuild()
//	if err != nil {
//		return nil
//	}
//	return instruction
//}

func (p *DriftClient) getSpotFulfillmentAccounts(
	marketIndex uint16,
	fulfillmentConfig *drift.FulfillmentConfig,
) solana.AccountMetaSlice {
	var remainingAccounts solana.AccountMetaSlice
	if fulfillmentConfig != nil {
		if fulfillmentConfig.SerumV3FulfillmentConfig != nil {
			remainingAccounts = append(remainingAccounts, p.getSerumRemainingAccounts(
				marketIndex,
				fulfillmentConfig.SerumV3FulfillmentConfig,
			)...)
		} else if fulfillmentConfig.PhoenixV1FulfillmentConfig != nil {
			remainingAccounts = append(remainingAccounts, p.getPhoenixRemainingAccounts(
				marketIndex,
				fulfillmentConfig.PhoenixV1FulfillmentConfig,
			)...)
		}
	} else {
		remainingAccounts = append(remainingAccounts, []*solana.AccountMeta{
			{
				PublicKey:  p.GetSpotMarketAccount(marketIndex).Vault,
				IsWritable: false,
				IsSigner:   false,
			},
			{
				PublicKey:  p.GetQuoteSpotMarketAccount().Vault,
				IsWritable: false,
				IsSigner:   false,
			},
		}...)
	}
	return remainingAccounts
}

func (p *DriftClient) getSerumRemainingAccounts(
	marketIndex uint16,
	fulfillmentConfig *driftlib.SerumV3FulfillmentConfig,
) solana.AccountMetaSlice {
	var remainingAccounts solana.AccountMetaSlice
	remainingAccounts = append(remainingAccounts, []*solana.AccountMeta{
		{
			PublicKey:  fulfillmentConfig.Pubkey,
			IsWritable: false,
			IsSigner:   false,
		},
		{
			PublicKey:  fulfillmentConfig.SerumProgramId,
			IsWritable: false,
			IsSigner:   false,
		},
		{
			PublicKey:  fulfillmentConfig.SerumMarket,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  fulfillmentConfig.SerumRequestQueue,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  fulfillmentConfig.SerumEventQueue,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  fulfillmentConfig.SerumBids,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  fulfillmentConfig.SerumAsks,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  fulfillmentConfig.SerumBaseVault,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  fulfillmentConfig.SerumQuoteVault,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  fulfillmentConfig.SerumOpenOrders,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey: addresses.GetSerumSignerPublicKey(
				fulfillmentConfig.SerumProgramId,
				fulfillmentConfig.SerumMarket,
				fulfillmentConfig.SerumSignerNonce,
			),
			IsWritable: false,
			IsSigner:   false,
		},
		{
			PublicKey:  p.GetSignerPublicKey(),
			IsWritable: false,
			IsSigner:   false,
		},
		{
			PublicKey:  spl_token.TOKEN_PROGRAM_ID,
			IsWritable: false,
			IsSigner:   false,
		},
		{
			PublicKey:  p.GetSpotMarketAccount(marketIndex).Vault,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  p.GetQuoteSpotMarketAccount().Vault,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  p.GetStateAccount().SrmVault,
			IsWritable: false,
			IsSigner:   false,
		},
	}...)
	return remainingAccounts
}

func (p *DriftClient) getPhoenixRemainingAccounts(
	marketIndex uint16,
	fulfillmentConfig *driftlib.PhoenixV1FulfillmentConfig,
) solana.AccountMetaSlice {
	var remainingAccounts solana.AccountMetaSlice
	remainingAccounts = append(remainingAccounts, []*solana.AccountMeta{
		{
			PublicKey:  fulfillmentConfig.Pubkey,
			IsWritable: false,
			IsSigner:   false,
		},
		{
			PublicKey:  fulfillmentConfig.PhoenixProgramId,
			IsWritable: false,
			IsSigner:   false,
		},
		{
			PublicKey:  fulfillmentConfig.PhoenixLogAuthority,
			IsWritable: false,
			IsSigner:   false,
		},
		{
			PublicKey:  fulfillmentConfig.PhoenixMarket,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  p.GetSignerPublicKey(),
			IsWritable: false,
			IsSigner:   false,
		},
		{
			PublicKey:  fulfillmentConfig.PhoenixBaseVault,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  fulfillmentConfig.PhoenixQuoteVault,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  p.GetSpotMarketAccount(marketIndex).Vault,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  p.GetQuoteSpotMarketAccount().Vault,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  spl_token.TOKEN_PROGRAM_ID,
			IsWritable: false,
			IsSigner:   false,
		},
	}...)
	return remainingAccounts
}

func (p *DriftClient) SettlePNLs(users []struct {
	SettleeUserAccountPublicKey solana.PublicKey
	SettleeUserAccount          *driftlib.User
},
	marketIndexes []uint16,
	filterInvalidMarkets *bool,
	txParams *drift.TxParams,
) (solana.Signature, error) {
	// # Filter market indexes by markets with valid oracle
	marketIndexToSettle := utils.TT(
		filterInvalidMarkets != nil && *filterInvalidMarkets,
		[]uint16{},
		marketIndexes,
	)

	if filterInvalidMarkets != nil && *filterInvalidMarkets {
		for _, marketIndex := range marketIndexes {
			perpMarketAccount := p.GetPerpMarketAccount(marketIndex)
			oraclePriceData := p.GetOracleDataForPerpMarket(marketIndex)
			stateAccountAndSlot := p.AccountSubscriber.GetStateAccountAndSlot()
			oracleGuardRails := &stateAccountAndSlot.Data.OracleGuardRails

			isValid := math.IsOracleValid(
				perpMarketAccount,
				oraclePriceData,
				oracleGuardRails,
				stateAccountAndSlot.Slot,
			)
			if isValid {
				marketIndexToSettle = append(marketIndexToSettle, marketIndex)
			}
		}
	}

	// # Settle filtered market indexes
	ixs, _ := p.GetSettlePNLsIxs(users, marketIndexToSettle)

	tx := p.BuildTransaction(
		ixs,
		utils.TT(txParams != nil, txParams, &drift.TxParams{
			BaseTxParams: drift.BaseTxParams{
				ComputeUnits: 1_400_000,
			},
		}),
		[]addresslookuptable.KeyedAddressLookupTable{},
	)

	txSig, err := p.SendTransaction(tx, &p.Opts, false)
	if err != nil {
		return solana.Signature{}, err
	}
	return txSig.TxSig, nil
}

func (p *DriftClient) GetSettlePNLsIxs(users []struct {
	SettleeUserAccountPublicKey solana.PublicKey
	SettleeUserAccount          *driftlib.User
}, marketIndexes []uint16) ([]solana.Instruction, error) {
	var ixs []solana.Instruction
	for _, user := range users {
		for _, marketIndex := range marketIndexes {
			ix, err := p.SettlePNLIx(user.SettleeUserAccountPublicKey, user.SettleeUserAccount, marketIndex)
			if err != nil {
				return nil, err
			}
			ixs = append(ixs, ix)
		}
	}
	return ixs, nil
}

func (p *DriftClient) SettlePNL(settleeUserAccountPublicKey solana.PublicKey,
	settleeUserAccount *driftlib.User,
	marketIndex uint16,
	txParams *drift.TxParams,
) (solana.Signature, error) {
	ix, _ := p.SettlePNLIx(settleeUserAccountPublicKey, settleeUserAccount, marketIndex)
	txSig, err := p.SendTransaction(
		p.BuildTransaction(
			[]solana.Instruction{ix},
			txParams,
			[]addresslookuptable.KeyedAddressLookupTable{},
		),
		&p.Opts,
		false,
	)
	if err != nil {
		return solana.Signature{}, err
	}
	return txSig.TxSig, nil
}

//func (p *DriftClient) SettlePNLIx(
//	settleeUserAccountPublicKey solana.PublicKey,
//	settleeUserAccount *driftlib.User,
//	marketIndex uint16,
//) (solana.Instruction, error) {
//	remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//		UserAccounts:              []*driftlib.User{settleeUserAccount},
//		WritablePerpMarketIndexes: []uint16{marketIndex},
//		WritableSpotMarketIndexes: []uint16{constants.QUOTE_SPOT_MARKET_INDEX},
//	})
//	builder := driftlib.NewSettlePnlInstructionBuilder().
//		SetStateAccount(p.GetStatePublicKey()).
//		SetAuthorityAccount(p.Wallet.GetPublicKey()).
//		SetUserAccount(settleeUserAccountPublicKey).
//		SetSpotMarketVaultAccount(p.GetQuoteSpotMarketAccount().Vault)
//	for _, account := range remainingAccounts {
//		builder.Append(account)
//	}
//	return builder.ValidateAndBuild()
//}

func (p *DriftClient) LiquidatePerp(
	userAccountPublicKey solana.PublicKey,
	userAccount *driftlib.User,
	marketIndex uint16,
	maxBaseAssetAmount *big.Int,
	limitPrice *big.Int,
	txParams *drift.TxParams,
	liquidatorSubAccountId *uint16,
) (solana.Signature, error) {
	ix, _ := p.GetLiquidatePerpIx(
		userAccountPublicKey,
		userAccount,
		marketIndex,
		maxBaseAssetAmount,
		limitPrice,
		liquidatorSubAccountId,
	)
	txSig, err := p.SendTransaction(
		p.BuildTransaction(
			[]solana.Instruction{ix},
			txParams,
			[]addresslookuptable.KeyedAddressLookupTable{},
		),
		&p.Opts,
		false,
	)
	if err != nil {
		return solana.Signature{}, err
	}
	p.PerpMarketLastSlotCache[marketIndex] = txSig.Slot
	return txSig.TxSig, nil
}

//func (p *DriftClient) GetLiquidatePerpIx(
//	userAccountPublicKey solana.PublicKey,
//	userAccount *driftlib.User,
//	marketIndex uint16,
//	maxBaseAssetAmount *big.Int,
//	limitPrice *big.Int,
//	liquidatorSubAccountId *uint16,
//) (solana.Instruction, error) {
//	userStatsPublicKey := addresses.GetUserStatsAccountPublicKey(
//		p.Program.GetProgramId(),
//		userAccount.Authority,
//	)
//
//	liquidator := p.GetUserAccountPublicKey(
//		liquidatorSubAccountId,
//		nil,
//	)
//	liquidatorStatsPublicKey := p.GetUserStatsAccountPublicKey()
//
//	remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//		UserAccounts:              []*driftlib.User{p.GetUserAccount(liquidatorSubAccountId, nil), userAccount},
//		UseMarketLastSlotCache:    true,
//		WritablePerpMarketIndexes: []uint16{marketIndex},
//	})
//
//	builder := driftlib.NewLiquidatePerpInstructionBuilder().
//		SetMarketIndex(marketIndex).
//		SetLiquidatorMaxBaseAssetAmount(maxBaseAssetAmount.Uint64()).
//		SetStateAccount(p.GetStatePublicKey()).
//		SetAuthorityAccount(p.Wallet.GetPublicKey()).
//		SetUserAccount(userAccountPublicKey).
//		SetUserStatsAccount(userStatsPublicKey).
//		SetLiquidatorAccount(liquidator).
//		SetLiquidatorStatsAccount(liquidatorStatsPublicKey)
//	if limitPrice != nil {
//		builder.SetLimitPrice(limitPrice.Uint64())
//	}
//	for _, account := range remainingAccounts {
//		builder.Append(account)
//	}
//	return builder.ValidateAndBuild()
//}

func (p *DriftClient) LiquidateSpot(
	userAccountPublicKey solana.PublicKey,
	userAccount *driftlib.User,
	assetMarketIndex uint16,
	liabilityMarketIndex uint16,
	maxLiabilityTransfer *big.Int,
	limitPrice *big.Int,
	txParams *drift.TxParams,
	liquidatorSubAccountId *uint16,
) (solana.Signature, error) {
	ix, _ := p.GetLiquidateSpotIx(
		userAccountPublicKey,
		userAccount,
		assetMarketIndex,
		liabilityMarketIndex,
		maxLiabilityTransfer,
		limitPrice,
		liquidatorSubAccountId,
	)
	txSig, err := p.SendTransaction(
		p.BuildTransaction(
			[]solana.Instruction{ix},
			txParams,
			[]addresslookuptable.KeyedAddressLookupTable{},
		),
		&p.Opts,
		false,
	)
	if err != nil {
		return solana.Signature{}, err
	}
	p.SpotMarketLastSlotCache[assetMarketIndex] = txSig.Slot
	p.SpotMarketLastSlotCache[liabilityMarketIndex] = txSig.Slot
	return txSig.TxSig, nil
}

// func (p *DriftClient) GetLiquidateSpotIx(
//
//	userAccountPublicKey solana.PublicKey,
//	userAccount *driftlib.User,
//	assetMarketIndex uint16,
//	liabilityMarketIndex uint16,
//	maxLiabilityTransfer *big.Int,
//	limitPrice *big.Int,
//	liquidatorSubAccountId *uint16,
//
//	) (solana.Instruction, error) {
//		userStatsPublicKey := addresses.GetUserStatsAccountPublicKey(
//			p.Program.GetProgramId(),
//			userAccount.Authority,
//		)
//
//		liquidator := p.GetUserAccountPublicKey(
//			liquidatorSubAccountId,
//			nil,
//		)
//		liquidatorStatsPublicKey := p.GetUserStatsAccountPublicKey()
//
//		remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//			UserAccounts:              []*driftlib.User{p.GetUserAccount(liquidatorSubAccountId, nil), userAccount},
//			UseMarketLastSlotCache:    true,
//			WritableSpotMarketIndexes: []uint16{liabilityMarketIndex, assetMarketIndex},
//		})
//
//		builder := driftlib.NewLiquidateSpotInstructionBuilder().
//			SetAssetMarketIndex(assetMarketIndex).
//			SetLiabilityMarketIndex(liabilityMarketIndex).
//			SetLiquidatorMaxLiabilityTransfer(utils.Uint128(maxLiabilityTransfer)).
//			SetStateAccount(p.GetStatePublicKey()).
//			SetAuthorityAccount(p.Wallet.GetPublicKey()).
//			SetUserAccount(userAccountPublicKey).
//			SetUserStatsAccount(userStatsPublicKey).
//			SetLiquidatorAccount(liquidator).
//			SetLiquidatorStatsAccount(liquidatorStatsPublicKey)
//		if limitPrice != nil {
//			builder.SetLimitPrice(limitPrice.Uint64())
//		}
//		for _, account := range remainingAccounts {
//			builder.Append(account)
//		}
//		return builder.ValidateAndBuild()
//	}
func (p *DriftClient) LiquidateBorrowForPerpPnl(
	userAccountPublicKey solana.PublicKey,
	userAccount *driftlib.User,
	perpMarketIndex uint16,
	liabilityMarketIndex uint16,
	maxLiabilityTransfer *big.Int,
	limitPrice *big.Int,
	txParams *drift.TxParams,
	liquidatorSubAccountId *uint16,
) (solana.Signature, error) {
	ix, _ := p.GetLiquidateBorrowForPerpPnlIx(
		userAccountPublicKey,
		userAccount,
		perpMarketIndex,
		liabilityMarketIndex,
		maxLiabilityTransfer,
		limitPrice,
		liquidatorSubAccountId,
	)
	txSig, err := p.SendTransaction(
		p.BuildTransaction(
			[]solana.Instruction{ix},
			txParams,
			[]addresslookuptable.KeyedAddressLookupTable{},
		),
		&p.Opts,
		false,
	)
	if err != nil {
		return solana.Signature{}, err
	}
	p.PerpMarketLastSlotCache[perpMarketIndex] = txSig.Slot
	p.SpotMarketLastSlotCache[liabilityMarketIndex] = txSig.Slot
	return txSig.TxSig, nil
}

// func (p *DriftClient) GetLiquidateBorrowForPerpPnlIx(
//
//	userAccountPublicKey solana.PublicKey,
//	userAccount *driftlib.User,
//	perpMarketIndex uint16,
//	liabilityMarketIndex uint16,
//	maxLiabilityTransfer *big.Int,
//	limitPrice *big.Int,
//	liquidatorSubAccountId *uint16,
//
//	) (solana.Instruction, error) {
//		userStatsPublicKey := addresses.GetUserStatsAccountPublicKey(
//			p.Program.GetProgramId(),
//			userAccount.Authority,
//		)
//
//		liquidator := p.GetUserAccountPublicKey(
//			liquidatorSubAccountId,
//			nil,
//		)
//		liquidatorStatsPublicKey := p.GetUserStatsAccountPublicKey()
//
//		remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//			UserAccounts:              []*driftlib.User{p.GetUserAccount(liquidatorSubAccountId, nil), userAccount},
//			WritablePerpMarketIndexes: []uint16{perpMarketIndex},
//			WritableSpotMarketIndexes: []uint16{liabilityMarketIndex},
//		})
//
//		builder := driftlib.NewLiquidateBorrowForPerpPnlInstructionBuilder().
//			SetPerpMarketIndex(perpMarketIndex).
//			SetSpotMarketIndex(liabilityMarketIndex).
//			SetLiquidatorMaxLiabilityTransfer(utils.Uint128(maxLiabilityTransfer)).
//			SetStateAccount(p.GetStatePublicKey()).
//			SetAuthorityAccount(p.Wallet.GetPublicKey()).
//			SetUserAccount(userAccountPublicKey).
//			SetUserStatsAccount(userStatsPublicKey).
//			SetLiquidatorAccount(liquidator).
//			SetLiquidatorStatsAccount(liquidatorStatsPublicKey)
//		if limitPrice != nil {
//			builder.SetLimitPrice(limitPrice.Uint64())
//		}
//		for _, account := range remainingAccounts {
//			builder.Append(account)
//		}
//		return builder.ValidateAndBuild()
//	}
func (p *DriftClient) LiquidatePerpPnlForDeposit(
	userAccountPublicKey solana.PublicKey,
	userAccount *driftlib.User,
	perpMarketIndex uint16,
	assetMarketIndex uint16,
	maxPnlTransfer *big.Int,
	limitPrice *big.Int,
	txParams *drift.TxParams,
	liquidatorSubAccountId *uint16,
) (solana.Signature, error) {
	ix, _ := p.GetLiquidatePerpPnlForDepositIx(
		userAccountPublicKey,
		userAccount,
		perpMarketIndex,
		assetMarketIndex,
		maxPnlTransfer,
		limitPrice,
		liquidatorSubAccountId,
	)
	txSig, err := p.SendTransaction(
		p.BuildTransaction(
			[]solana.Instruction{ix},
			txParams,
			[]addresslookuptable.KeyedAddressLookupTable{},
		),
		&p.Opts,
		false,
	)
	if err != nil {
		return solana.Signature{}, err
	}
	p.PerpMarketLastSlotCache[perpMarketIndex] = txSig.Slot
	p.SpotMarketLastSlotCache[assetMarketIndex] = txSig.Slot
	return txSig.TxSig, nil
}

// func (p *DriftClient) GetLiquidatePerpPnlForDepositIx(
//
//	userAccountPublicKey solana.PublicKey,
//	userAccount *driftlib.User,
//	perpMarketIndex uint16,
//	assetMarketIndex uint16,
//	maxPnlTransfer *big.Int,
//	limitPrice *big.Int,
//	liquidatorSubAccountId *uint16,
//
//	) (solana.Instruction, error) {
//		userStatsPublicKey := addresses.GetUserStatsAccountPublicKey(
//			p.Program.GetProgramId(),
//			userAccount.Authority,
//		)
//
//		liquidator := p.GetUserAccountPublicKey(
//			liquidatorSubAccountId,
//			nil,
//		)
//		liquidatorStatsPublicKey := p.GetUserStatsAccountPublicKey()
//
//		remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//			UserAccounts:              []*driftlib.User{p.GetUserAccount(liquidatorSubAccountId, nil), userAccount},
//			WritablePerpMarketIndexes: []uint16{perpMarketIndex},
//			WritableSpotMarketIndexes: []uint16{assetMarketIndex},
//		})
//
//		builder := driftlib.NewLiquidatePerpPnlForDepositInstructionBuilder().
//			SetPerpMarketIndex(perpMarketIndex).
//			SetSpotMarketIndex(assetMarketIndex).
//			SetLiquidatorMaxPnlTransfer(utils.Uint128(maxPnlTransfer)).
//			SetStateAccount(p.GetStatePublicKey()).
//			SetAuthorityAccount(p.Wallet.GetPublicKey()).
//			SetUserAccount(userAccountPublicKey).
//			SetUserStatsAccount(userStatsPublicKey).
//			SetLiquidatorAccount(liquidator).
//			SetLiquidatorStatsAccount(liquidatorStatsPublicKey)
//		if limitPrice != nil {
//			builder.SetLimitPrice(limitPrice.Uint64())
//		}
//		for _, account := range remainingAccounts {
//			builder.Append(account)
//		}
//		return builder.ValidateAndBuild()
//	}
func (p *DriftClient) ResolvePerpBankruptcy(
	userAccountPublicKey solana.PublicKey,
	userAccount *driftlib.User,
	marketIndex uint16,
	txParams *drift.TxParams,
	liquidatorSubAccountId *uint16,
) (solana.Signature, error) {
	ix, err := p.GetResolvePerpBankruptcyIx(
		userAccountPublicKey,
		userAccount,
		marketIndex,
		liquidatorSubAccountId,
	)
	if err != nil {
		return solana.Signature{}, err
	}
	txSig, err := p.SendTransaction(
		p.BuildTransaction([]solana.Instruction{ix}, txParams, []addresslookuptable.KeyedAddressLookupTable{}),
		&p.Opts,
		false,
	)
	if err != nil {
		return solana.Signature{}, err
	}
	return txSig.TxSig, nil
}

// func (p *DriftClient) GetResolvePerpBankruptcyIx(
//
//	userAccountPublicKey solana.PublicKey,
//	userAccount *driftlib.User,
//	marketIndex uint16,
//	liquidatorSubAccountId *uint16,
//
//	) (solana.Instruction, error) {
//		userStatsPublicKey := addresses.GetUserStatsAccountPublicKey(
//			p.Program.GetProgramId(),
//			userAccount.Authority,
//		)
//
//		liquidator := p.GetUserAccountPublicKey(
//			liquidatorSubAccountId,
//			nil,
//		)
//		liquidatorStatsPublicKey := p.GetUserStatsAccountPublicKey()
//
//		remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//			UserAccounts:              []*driftlib.User{p.GetUserAccount(liquidatorSubAccountId, nil), userAccount},
//			WritablePerpMarketIndexes: []uint16{marketIndex},
//			WritableSpotMarketIndexes: []uint16{constants.QUOTE_SPOT_MARKET_INDEX},
//		})
//
//		spotMarket := p.GetQuoteSpotMarketAccount()
//
//		builder := driftlib.NewResolvePerpBankruptcyInstructionBuilder().
//			SetQuoteSpotMarketIndex(constants.QUOTE_SPOT_MARKET_INDEX).
//			SetMarketIndex(marketIndex).
//			SetStateAccount(p.GetStatePublicKey()).
//			SetAuthorityAccount(p.Wallet.GetPublicKey()).
//			SetUserAccount(userAccountPublicKey).
//			SetUserStatsAccount(userStatsPublicKey).
//			SetLiquidatorAccount(liquidator).
//			SetLiquidatorStatsAccount(liquidatorStatsPublicKey).
//			SetSpotMarketVaultAccount(spotMarket.Vault).
//			SetInsuranceFundVaultAccount(spotMarket.InsuranceFund.Vault).
//			SetDriftSignerAccount(p.GetSignerPublicKey()).
//			SetTokenProgramAccount(spl_token.TOKEN_PROGRAM_ID)
//		for _, account := range remainingAccounts {
//			builder.Append(account)
//		}
//		return builder.ValidateAndBuild()
//	}
func (p *DriftClient) ResolveSpotBankruptcy(
	userAccountPublicKey solana.PublicKey,
	userAccount *driftlib.User,
	marketIndex uint16,
	txParams *drift.TxParams,
	liquidatorSubAccountId *uint16,
) (solana.Signature, error) {
	ix, err := p.GetResolveSpotBankruptcyIx(
		userAccountPublicKey,
		userAccount,
		marketIndex,
		liquidatorSubAccountId,
	)
	if err != nil {
		return solana.Signature{}, err
	}
	txSig, err := p.SendTransaction(
		p.BuildTransaction([]solana.Instruction{ix}, txParams, []addresslookuptable.KeyedAddressLookupTable{}),
		&p.Opts,
		false,
	)
	if err != nil {
		return solana.Signature{}, err
	}
	return txSig.TxSig, nil
}

// func (p *DriftClient) GetResolveSpotBankruptcyIx(
//
//	userAccountPublicKey solana.PublicKey,
//	userAccount *driftlib.User,
//	marketIndex uint16,
//	liquidatorSubAccountId *uint16,
//
//	) (solana.Instruction, error) {
//		userStatsPublicKey := addresses.GetUserStatsAccountPublicKey(
//			p.Program.GetProgramId(),
//			userAccount.Authority,
//		)
//
//		liquidator := p.GetUserAccountPublicKey(
//			liquidatorSubAccountId,
//			nil,
//		)
//		liquidatorStatsPublicKey := p.GetUserStatsAccountPublicKey()
//
//		remainingAccounts := p.GetRemainingAccounts(&drifttype.RemainingAccountParams{
//			UserAccounts:              []*driftlib.User{p.GetUserAccount(liquidatorSubAccountId, nil), userAccount},
//			WritablePerpMarketIndexes: []uint16{marketIndex},
//			WritableSpotMarketIndexes: []uint16{constants.QUOTE_SPOT_MARKET_INDEX},
//		})
//
//		spotMarket := p.GetSpotMarketAccount(marketIndex)
//
//		builder := driftlib.NewResolveSpotBankruptcyInstructionBuilder().
//			SetStateAccount(p.GetStatePublicKey()).
//			SetAuthorityAccount(p.Wallet.GetPublicKey()).
//			SetUserAccount(userAccountPublicKey).
//			SetUserStatsAccount(userStatsPublicKey).
//			SetLiquidatorAccount(liquidator).
//			SetLiquidatorStatsAccount(liquidatorStatsPublicKey).
//			SetSpotMarketVaultAccount(spotMarket.Vault).
//			SetInsuranceFundVaultAccount(spotMarket.InsuranceFund.Vault).
//			SetDriftSignerAccount(p.GetSignerPublicKey()).
//			SetTokenProgramAccount(spl_token.TOKEN_PROGRAM_ID)
//		for _, account := range remainingAccounts {
//			builder.Append(account)
//		}
//		return builder.ValidateAndBuild()
//	}
func (p *DriftClient) UpdatePrelaunchOracle(marketIndex uint16, txParams *drift.TxParams) (solana.Signature, error) {
	ix, err := p.GetUpdatePrelaunchOracleIx(marketIndex)
	if err != nil {
		return solana.Signature{}, err
	}
	txSig, err := p.SendTransaction(
		p.BuildTransaction(
			[]solana.Instruction{ix},
			txParams,
			[]addresslookuptable.KeyedAddressLookupTable{},
		),
		&p.Opts,
		false,
	)
	if err != nil {
		return solana.Signature{}, err
	}
	return txSig.TxSig, nil
}

//	func (p *DriftClient) GetUpdatePrelaunchOracleIx(marketIndex uint16) (solana.Instruction, error) {
//		perpMarket := p.GetPerpMarketAccount(marketIndex)
//
//		if perpMarket.Amm.OracleSource != driftlib.OracleSource_Prelaunch {
//			return nil, errors.New(fmt.Sprintf("Wrong oracle source %v", perpMarket.Amm.OracleSource))
//		}
//		return driftlib.NewUpdatePrelaunchOracleInstructionBuilder().
//			SetStateAccount(p.GetStatePublicKey()).
//			SetPerpMarketAccount(perpMarket.Pubkey).
//			SetOracleAccount(perpMarket.Amm.Oracle).
//			ValidateAndBuild()
//	}
func (p *DriftClient) UpdatePerpBidAskTwap(
	marketIndex uint16,
	makers [][]solana.PublicKey,
	txParams *drift.TxParams,
) (solana.Signature, error) {
	ix, err := p.GetUpdatePerpBidAskTwapIx(marketIndex, makers)
	if err != nil {
		return solana.Signature{}, err
	}
	txSig, err := p.SendTransaction(
		p.BuildTransaction(
			[]solana.Instruction{ix},
			txParams,
			[]addresslookuptable.KeyedAddressLookupTable{},
		),
		&p.Opts,
		false,
	)
	if err != nil {
		return solana.Signature{}, err
	}
	return txSig.TxSig, nil
}

// func (p *DriftClient) GetUpdatePerpBidAskTwapIx(
//
//	marketIndex uint16,
//	makers [][]solana.PublicKey,
//
//	) (solana.Instruction, error) {
//		perpMarket := p.GetPerpMarketAccount(marketIndex)
//
//		var remainingAccounts solana.AccountMetaSlice
//		for _, makerKeys := range makers {
//			maker := makerKeys[0]
//			makerStats := makerKeys[1]
//			remainingAccounts = append(remainingAccounts, &solana.AccountMeta{
//				PublicKey:  maker,
//				IsWritable: false,
//				IsSigner:   false,
//			}, &solana.AccountMeta{
//				PublicKey:  makerStats,
//				IsWritable: false,
//				IsSigner:   false,
//			})
//		}
//
//		builder := driftlib.NewUpdatePerpBidAskTwapInstructionBuilder().
//			SetStateAccount(p.GetStatePublicKey()).
//			SetPerpMarketAccount(perpMarket.Pubkey).
//			SetOracleAccount(perpMarket.Amm.Oracle).
//			SetAuthorityAccount(p.Wallet.GetPublicKey()).
//			SetKeeperStatsAccount(p.GetUserStatsAccountPublicKey())
//		for _, accountMeta := range remainingAccounts {
//			builder.Append(accountMeta)
//		}
//		return builder.ValidateAndBuild()
//	}
func (p *DriftClient) TriggerEvent(eventName string, data ...interface{}) {
	p.EventEmitter.Emit(eventName, data...)
}

func (p *DriftClient) GetOracleDataForPerpMarket(marketIndex uint16) *oracles.OraclePriceData {
	dataAndSlot := p.AccountSubscriber.GetOraclePriceDataAndSlotForPerpMarket(
		marketIndex,
	)
	if dataAndSlot != nil {
		return dataAndSlot.Data
	}
	return nil
}

func (p *DriftClient) GetOracleDataForSpotMarket(marketIndex uint16) *oracles.OraclePriceData {
	dataAndSlot := p.AccountSubscriber.GetOraclePriceDataAndSlotForSpotMarket(
		marketIndex,
	)
	if dataAndSlot != nil {
		return dataAndSlot.Data
	}
	return nil
}

func (p *DriftClient) GetPerpMarketExtendedInfo(marketIndex uint16) *drift.PerpMarketExtendedInfo {
	marketAccount := p.GetPerpMarketAccount(marketIndex)
	quoteAccount := p.GetSpotMarketAccount(constants.QUOTE_SPOT_MARKET_INDEX)

	return &drift.PerpMarketExtendedInfo{
		MarketIndex:       marketIndex,
		MinOrderSize:      utils.BN(marketAccount.Amm.MinOrderSize),
		MarginMaintenance: marketAccount.MarginRatioMaintenance,
		PnlPoolValue: math.GetTokenAmount(
			marketAccount.PnlPool.ScaledBalance.BigInt(),
			quoteAccount,
			driftlib.SpotBalanceType_Deposit,
		),
		AvailableInsurance: math.CalculateMarketMaxAvailableInsurance(
			marketAccount,
			quoteAccount,
		),
		ContractTier: marketAccount.ContractTier,
	}
}

func (p *DriftClient) GetMarketFees(
	marketType driftlib.MarketType,
	marketIndex *uint16,
	user drifttype.IUser,
) (int64, int64) {
	var feeTier *driftlib.FeeTier
	if user != nil {
		feeTier = user.GetUserFeeTier(marketType, nil)
	} else {
		state := p.GetStateAccount()
		if marketType == driftlib.MarketType_Perp {
			feeTier = &state.PerpFeeStructure.FeeTiers[0]
		} else {
			feeTier = &state.SpotFeeStructure.FeeTiers[0]
		}
	}

	takerFee := int64(feeTier.FeeNumerator / feeTier.FeeDenominator)
	makerFee := int64(feeTier.MakerRebateNumerator / feeTier.MakerRebateDenominator)

	if marketIndex != nil {
		var feeAdjustment int64
		if marketType == driftlib.MarketType_Perp {
			marketAccount := p.GetPerpMarketAccount(*marketIndex)
			feeAdjustment = int64(marketAccount.FeeAdjustment)
		} else {
			marketAccount := p.GetSpotMarketAccount(*marketIndex)
			feeAdjustment = int64(marketAccount.FeeAdjustment)
		}
		takerFee += takerFee * feeAdjustment / 100
		makerFee += makerFee * feeAdjustment / 100

	}
	return takerFee, makerFee
}
