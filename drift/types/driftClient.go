package types

import (
	driftgo "driftgo"
	"driftgo/accounts"
	"driftgo/anchor/types"
	"driftgo/jupiter"
	driftlib "driftgo/lib/drift"
	"driftgo/lib/event"
	oracles "driftgo/oracles/types"
	"driftgo/tx"
	"math/big"

	"github.com/gagliardetto/solana-go"
	addresslookuptable "github.com/gagliardetto/solana-go/programs/address-lookup-table"
	jupiter2 "github.com/ilkamo/jupiter-go/jupiter"
)

type RemainingAccountParams struct {
	UserAccounts              []*driftlib.User
	WritablePerpMarketIndexes []uint16
	WritableSpotMarketIndexes []uint16
	ReadablePerpMarketIndex   []uint16
	ReadableSpotMarketIndex   []uint16
	UseMarketLastSlotCache    bool
}

type IDriftClient interface {
	GetProgram() types.IProgram
	GetOpts() *driftgo.ConfirmOptions
	GetEventEmitter() *event.EventEmitter
	IsSubscribed() bool
	SetSubscribed(bool)
	GetUserMapKey(uint16, solana.PublicKey) string
	Subscribe() bool

	//SubscribeUsers() bool
	GetActiveSubAccountId() uint16

	FetchAccounts()
	Unsubscribe()
	GetStatePublicKey() solana.PublicKey
	GetSignerPublicKey() solana.PublicKey
	GetStateAccount() *driftlib.State
	ForceGetStateAccount() *driftlib.State
	GetPerpMarketAccount(uint16) *driftlib.PerpMarket
	ForceGetPerpMarketAccount(uint16) *driftlib.PerpMarket
	GetPerpMarketAccounts() []*driftlib.PerpMarket
	GetSpotMarketAccount(uint16) *driftlib.SpotMarket
	ForceGetSpotMarketAccount(uint16) *driftlib.SpotMarket
	GetSpotMarketAccounts() []*driftlib.SpotMarket
	GetQuoteSpotMarketAccount() *driftlib.SpotMarket
	GetOraclePriceDataAndSlot(solana.PublicKey) *accounts.DataAndSlot[*oracles.OraclePriceData]
	GetSerumV3FulfillmentConfig(solana.PublicKey) *driftlib.SerumV3FulfillmentConfig
	GetPhoenixV1FulfillmentConfig(solana.PublicKey) *driftlib.PhoenixV1FulfillmentConfig
	FetchMarketLookupTableAccount() *addresslookuptable.KeyedAddressLookupTable
	//UpdateWallet(solana.Wallet, []uint8, uint8, bool, map[string]uint8) bool
	SwitchActiveUser(uint16, *solana.PublicKey)
	AddUser(uint16, *solana.PublicKey, *driftlib.User) bool
	//InitializeUserAccount(uint8, string, *driftgo.ReferrerInfo)

	//InitializeReferrerName(string) solana.Signature
	//UpdateUserName(string, uint8) solana.Signature
	//UpdateUserCustomMarginRatio()
	//GetUpdateUserCustomMarginRatioIx
	//GetUpdateUserMarginTradingEnabledIx
	//UpdateUserMarginTradingEnabled
	//updateUserDelegate
	//updateUserAdvancedLp
	//getUpdateAdvancedDlpIx
	//updateUserReduceOnly
	//getUpdateUserReduceOnlyIx

	//FetchAllUserAccounts(bool) []driftgo.ProgramAccount[driftlib.User]
	GetUserAccountsForDelegate(solana.PublicKey) []*driftlib.User

	//getUserAccountsAndAddressesForAuthority

	GetUserAccountsForAuthority(solana.PublicKey) []*driftlib.User

	//getReferredUserStatsAccountsByReferrer
	//getReferrerNameAccountsForAuthority

	//DeleteUser(uint8, driftgo.TxParams)
	//GetUserDeletionIx(solana.PublicKey) *solana.Instruction
	//reclaimRent
	//getReclaimRentIx

	GetUser(*uint16, *solana.PublicKey) IUser
	HasUser(*uint16, *solana.PublicKey) bool
	GetUsers() []IUser

	GetUserStats() IUserStats
	//FetchReferrerNameAccount(string) *driftgo.ReferrerNameAccount
	GetUserStatsAccountPublicKey() solana.PublicKey
	GetUserAccountPublicKey(*uint16, *solana.PublicKey) solana.PublicKey
	GetUserAccount(*uint16, *solana.PublicKey) *driftlib.User
	ForceGetUserAccount(*uint16) *driftlib.User
	GetUserAccountAndSlot(*uint16) *accounts.DataAndSlot[*driftlib.User]
	GetSpotPosition(uint16, *uint16) *driftlib.SpotPosition
	GetQuoteAssetTokenAmount() *big.Int
	GetTokenAmount(uint16) *big.Int
	GetRemainingAccounts(params *RemainingAccountParams) solana.AccountMetaSlice
	//convertToSpotPrecision
	//convertToPerpPrecision
	//convertToPricePrecision
	//mustIncludeMarketsInIx

	GetOrder(uint32, *uint16) *driftlib.Order
	GetOrderByUserId(uint8, *uint16) *driftlib.Order
	BuildTransaction([]solana.Instruction, *driftgo.TxParams, []addresslookuptable.KeyedAddressLookupTable) *solana.Transaction
	SendTransaction(*solana.Transaction, *driftgo.ConfirmOptions, bool) (*tx.TxSigAndSlot, error)
	GetAssociatedTokenAccount(uint16, bool) solana.PublicKey
	CreateAssociatedTokenAccountIdempotentInstruction(solana.PublicKey, solana.PublicKey, solana.PublicKey, solana.PublicKey) solana.Instruction

	//deposit
	//getDepositInstruction

	CheckIfAccountExists(solana.PublicKey) bool
	GetWrappedSolAccountCreationIxs(*big.Int, *bool) ([]solana.Instruction, solana.PublicKey)
	GetAssociatedTokenAccountCreationIx(solana.PublicKey, solana.PublicKey) (solana.Instruction, error)

	//initializeUserAccountAndDepositCollateral
	//initializeUserAccountForDevnet

	Withdraw(*big.Int, uint16, solana.PublicKey, bool, *uint16, *driftgo.TxParams) (solana.Signature, error)
	WithdrawAllDustPositions(*uint16, *driftgo.TxParams, func(int)) (solana.Signature, error)
	GetWithdrawIx(*big.Int, uint16, solana.PublicKey, bool, *uint16) (solana.Instruction, error)

	//transferDeposit
	//getTransferDepositIx
	//updateSpotMarketCumulativeInterest
	//updateSpotMarketCumulativeInterestIx
	//settleLP
	//settleLPIx
	//removePerpLpShares
	//removePerpLpSharesInExpiringMarket
	//getRemovePerpLpSharesInExpiringMarket
	//getRemovePerpLpSharesIx
	//addPerpLpShares
	//getAddPerpLpSharesIx
	//getQuoteValuePerLpShare
	//openPosition

	//SendSignedTx(*solana.Transaction) solana.Signature

	//sendMarketOrderAndGetSignedFillTx

	PlacePerpOrder(driftgo.OptionalOrderParams, *driftgo.TxParams, *uint16) (solana.Signature, error)
	GetPlacePerpOrderIx(driftgo.OptionalOrderParams, *uint16) (solana.Instruction, error)
	//UpdateAMMs([]uint16, *driftgo.TxParams) solana.Signature
	//GetUpdateAMMsIx([]uint16) *solana.Instruction

	//SettleExpiredMarket(uint16, *TxParams) solana.Signature
	//GetSettleExpiredMarketIx(uint16) *solana.Instruction
	//settleExpiredMarketPoolsToRevenuePool

	//CancelOrder(int32, *driftgo.TxParams, *uint16) solana.Signature
	//GetCancelOrderIx(int32, *uint16) *solana.Instruction
	//CancelOrderByUserId(uint8, *driftgo.TxParams, *uint16) solana.Signature
	//GetCancelOrderByUserIdIx(uint8, *uint16) *solana.Instruction
	//CancelOrdersByIds([]int32, *driftgo.TxParams, *uint16)
	//GetCancelOrdersByIdsIx([]int32, *uint16) *solana.Instruction

	CancelOrders(*driftlib.MarketType, *uint16, *driftlib.PositionDirection, *driftgo.TxParams, *uint16) (solana.Signature, error)
	GetCancelOrdersIx(*driftlib.MarketType, *uint16, *driftlib.PositionDirection, *uint16) (solana.Instruction, error)

	//CancelAndPlaceOrders(driftgo.CancelOrderParams, []driftlib.OrderParams, *driftgo.TxParams, *uint16) solana.Signature
	//PlaceOrders([]driftlib.OrderParams, *driftgo.TxParams, *uint16) solana.Signature
	//GetPlaceOrdersIx([]driftgo.OptionalOrderParams, *uint16) *solana.Instruction
	FillPerpOrder(solana.PublicKey, *driftlib.User, *driftlib.Order, []*driftgo.MakerInfo, *driftgo.ReferrerInfo, *driftgo.TxParams, *uint16) (solana.Signature, error)
	GetFillPerpOrderIx(solana.PublicKey, *driftlib.User, *driftlib.Order, []*driftgo.MakerInfo, *driftgo.ReferrerInfo, *uint16) solana.Instruction
	GetRevertFillIx(*solana.PublicKey) solana.Instruction
	PlaceSpotOrder(driftgo.OptionalOrderParams, *driftgo.TxParams, *uint16) (solana.Signature, error)
	GetPlaceSpotOrderIx(driftgo.OptionalOrderParams, *uint16) (solana.Instruction, error)
	FillSpotOrder(solana.PublicKey, *driftlib.User, *driftlib.Order, *driftgo.FulfillmentConfig, []*driftgo.MakerInfo, *driftgo.ReferrerInfo, *driftgo.TxParams) (solana.Signature, error)
	GetFillSpotOrderIx(solana.PublicKey, *driftlib.User, *driftlib.Order, *driftgo.FulfillmentConfig, []*driftgo.MakerInfo, *driftgo.ReferrerInfo, *solana.PublicKey) (solana.Instruction, error)

	//swap
	//getJupiterSwapIx
	GetJupiterSwapIxV6(
		*jupiter.JupiterClient,
		uint16,
		uint16,
		*solana.PublicKey,
		*solana.PublicKey,
		*big.Int,
		*int,
		*jupiter2.SwapMode,
		*bool,
		*jupiter2.QuoteResponse,
		*driftlib.SwapReduceOnly,
		*solana.PublicKey) ([]solana.Instruction, []addresslookuptable.KeyedAddressLookupTable, error)
	//getSwapIx
	//stakeForMSOL
	//getStakeForMSOLIx

	TriggerOrder(solana.PublicKey, *driftlib.User, *driftlib.Order, *driftgo.TxParams, *solana.PublicKey) (solana.Signature, error)
	GetTriggerOrderIx(solana.PublicKey, *driftlib.User, *driftlib.Order, *solana.PublicKey) solana.Instruction
	ForceCancelOrders(solana.PublicKey, *driftlib.User, *driftgo.TxParams, *solana.PublicKey) (solana.Signature, error)
	GetForceCancelOrdersIx(solana.PublicKey, *driftlib.User, *solana.PublicKey) solana.Instruction

	//updateUserIdle
	//getUpdateUserIdleIx
	//updateUserOpenOrdersCount
	//getUpdateUserOpenOrdersCountIx

	//PlaceAndTakePerpOrder(driftgo.OptionalOrderParams, []*driftgo.MakerInfo, *driftgo.ReferrerInfo, *driftgo.TxParams, uint8) solana.Signature
	//PlaceAndTakePerpWithAdditionalOrders(
	//	driftgo.OptionalOrderParams,
	//	[]*driftgo.MakerInfo,
	//	*driftgo.ReferrerInfo,
	//	[]driftgo.OptionalOrderParams,
	//	*driftgo.TxParams,
	//	uint8,
	//	bool,
	//	bool,
	//	bool,
	//) (solana.Signature, solana.Instruction, solana.Instruction)
	//GetPlaceAndTakePerpOrderIx(driftgo.OptionalOrderParams, []*driftgo.MakerInfo, *driftgo.ReferrerInfo, uint8) solana.Instruction
	//PlaceAndMakePerpOrder(driftgo.OptionalOrderParams, []*driftgo.TakerInfo, *driftgo.ReferrerInfo, *driftgo.TxParams, uint8) solana.Signature
	//GetPlaceAndMakePerpOrderIx(driftgo.OptionalOrderParams, *driftgo.TakerInfo, *driftgo.ReferrerInfo, uint8) solana.Instruction
	//PlaceAndTakeSpotOrder(driftgo.OptionalOrderParams, *driftlib.SerumV3FulfillmentConfig, *driftgo.MakerInfo, *driftgo.ReferrerInfo, *driftgo.TxParams, uint8) solana.Signature
	//GetPlaceAndTakeSpotOrderIx(driftgo.OptionalOrderParams, *driftlib.SerumV3FulfillmentConfig, *driftgo.MakerInfo, *driftgo.ReferrerInfo, uint8) solana.Instruction
	//PlaceAndMakeSpotOrder(driftgo.OptionalOrderParams, *driftgo.TakerInfo, *driftlib.SerumV3FulfillmentConfig, *driftgo.ReferrerInfo, *driftgo.TxParams, uint8) solana.Signature
	//GetPlaceAndMakeSpotOrderIx(driftgo.OptionalOrderParams, *driftgo.TakerInfo, *driftlib.SerumV3FulfillmentConfig, *driftgo.ReferrerInfo, uint8) solana.Instruction

	//closePosition
	//modifyPerpOrder
	//modifyPerpOrderByUserOrderId
	//modifyOrder
	//getModifyOrderIx
	//modifyOrderByUserOrderId
	//getModifyOrderByUserIdIx

	SettlePNLs([]struct {
		SettleeUserAccountPublicKey solana.PublicKey
		SettleeUserAccount          *driftlib.User
	}, []uint16, *bool, *driftgo.TxParams) (solana.Signature, error)
	GetSettlePNLsIxs([]struct {
		SettleeUserAccountPublicKey solana.PublicKey
		SettleeUserAccount          *driftlib.User
	}, []uint16) ([]solana.Instruction, error)

	SettlePNL(solana.PublicKey, *driftlib.User, uint16, *driftgo.TxParams) (solana.Signature, error)
	SettlePNLIx(solana.PublicKey, *driftlib.User, uint16) (solana.Instruction, error)

	LiquidatePerp(solana.PublicKey, *driftlib.User, uint16, *big.Int, *big.Int, *driftgo.TxParams, *uint16) (solana.Signature, error)
	GetLiquidatePerpIx(solana.PublicKey, *driftlib.User, uint16, *big.Int, *big.Int, *uint16) (solana.Instruction, error)
	LiquidateSpot(solana.PublicKey, *driftlib.User, uint16, uint16, *big.Int, *big.Int, *driftgo.TxParams, *uint16) (solana.Signature, error)
	GetLiquidateSpotIx(solana.PublicKey, *driftlib.User, uint16, uint16, *big.Int, *big.Int, *uint16) (solana.Instruction, error)
	LiquidateBorrowForPerpPnl(solana.PublicKey, *driftlib.User, uint16, uint16, *big.Int, *big.Int, *driftgo.TxParams, *uint16) (solana.Signature, error)
	GetLiquidateBorrowForPerpPnlIx(solana.PublicKey, *driftlib.User, uint16, uint16, *big.Int, *big.Int, *uint16) (solana.Instruction, error)
	LiquidatePerpPnlForDeposit(solana.PublicKey, *driftlib.User, uint16, uint16, *big.Int, *big.Int, *driftgo.TxParams, *uint16) (solana.Signature, error)
	GetLiquidatePerpPnlForDepositIx(solana.PublicKey, *driftlib.User, uint16, uint16, *big.Int, *big.Int, *uint16) (solana.Instruction, error)

	ResolvePerpBankruptcy(solana.PublicKey, *driftlib.User, uint16, *driftgo.TxParams, *uint16) (solana.Signature, error)
	GetResolvePerpBankruptcyIx(solana.PublicKey, *driftlib.User, uint16, *uint16) (solana.Instruction, error)

	ResolveSpotBankruptcy(solana.PublicKey, *driftlib.User, uint16, *driftgo.TxParams, *uint16) (solana.Signature, error)
	GetResolveSpotBankruptcyIx(solana.PublicKey, *driftlib.User, uint16, *uint16) (solana.Instruction, error)

	//updateFundingRate
	//getUpdateFundingRateIx
	UpdatePrelaunchOracle(uint16, *driftgo.TxParams) (solana.Signature, error)
	GetUpdatePrelaunchOracleIx(uint16) (solana.Instruction, error)

	UpdatePerpBidAskTwap(uint16, [][]solana.PublicKey, *driftgo.TxParams) (solana.Signature, error)
	GetUpdatePerpBidAskTwapIx(uint16, [][]solana.PublicKey) (solana.Instruction, error)

	//settleFundingPayment
	//getSettleFundingPaymentIx

	TriggerEvent(string, ...interface{})
	GetOracleDataForPerpMarket(uint16) *oracles.OraclePriceData
	GetOracleDataForSpotMarket(uint16) *oracles.OraclePriceData

	//initializeInsuranceFundStake
	//getInitializeInsuranceFundStakeIx
	//getAddInsuranceFundStakeIx
	//addInsuranceFundStake
	//requestRemoveInsuranceFundStake
	//cancelRequestRemoveInsuranceFundStake
	//removeInsuranceFundStake
	//settleRevenueToInsuranceFund
	//resolvePerpPnlDeficit
	//getResolvePerpPnlDeficitIx
	//getDepositIntoSpotMarketRevenuePoolIx
	//depositIntoSpotMarketRevenuePool

	GetPerpMarketExtendedInfo(uint16) *driftgo.PerpMarketExtendedInfo
	GetMarketFees(driftlib.MarketType, *uint16, IUser) (int64, int64)
}
