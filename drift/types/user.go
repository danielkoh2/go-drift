package types

import (
	go_drift "driftgo"
	"driftgo/accounts"
	"driftgo/lib/drift"
	oracles "driftgo/oracles/types"
	"github.com/gagliardetto/solana-go"
	"math/big"
)

type IUser interface {
	Subscribed() bool
	Subscribe(userAccount *drift.User) bool
	Unsubscribe()
	UpdateData(*drift.User, uint64)
	FetchAccounts()
	GetUserAccount() *drift.User
	ForceGetUserAccount() *drift.User
	GetUserAccountAndSlot() *accounts.DataAndSlot[*drift.User]
	GetPerpPositionForUserAccount(
		userAccount *drift.User,
		marketIndex uint16,
	) *drift.PerpPosition
	GetPerpPosition(uint16) *drift.PerpPosition
	GetPerpPositionAndSlot(marketIndex uint16) *accounts.DataAndSlot[*drift.PerpPosition]
	GetSpotPositionForUserAccount(
		userAccount *drift.User,
		marketIndex uint16,
	) *drift.SpotPosition
	GetSpotPosition(uint16) *drift.SpotPosition
	GetSpotPositionAndSlot(marketIndex uint16) *accounts.DataAndSlot[*drift.SpotPosition]
	GetTokenAmount(marketIndex uint16) *big.Int
	GetEmptyPosition(uint16) *drift.PerpPosition
	GetClonedPosition(position *drift.PerpPosition) *drift.PerpPosition
	GetOrderForUserAccount(
		userAccount *drift.User,
		orderId uint32,
	) *drift.Order
	GetOrder(orderId uint32) *drift.Order
	GetOrderByUserIdForUserAccount(
		userAccount *drift.User,
		userOrderId uint8,
	) *drift.Order
	GetOrderByUserOrderId(userOrderId uint8) *drift.Order
	GetOrderByUserOrderIdAndSlot(userOrderId uint8) *accounts.DataAndSlot[*drift.Order]
	GetOpenOrdersForUserAccount(userAccount *drift.User) []*drift.Order
	GetOpenOrders() []*drift.Order
	GetUserAccountPublicKey() solana.PublicKey
	Exists() bool
	GetPerpBidAsks(marketIndex uint16) (*big.Int, *big.Int)
	GetLPBidAsks(marketIndex uint16, lpShares *big.Int) (*big.Int, *big.Int)
	GetPerpPositionWithLPSettle(
		marketIndex uint16,
		originalPosition *drift.PerpPosition,
		burnLpShares bool,
		includeRemainderInBaseAmount bool, // false
	) (*drift.PerpPosition, *big.Int, *big.Int)
	GetPerpBuyingPower(marketIndex uint16, collateralBuffer *big.Int) *big.Int

	GetFreeCollateral(marginCategory *drift.MarginRequirementType) *big.Int
	GetMarginRequirement(
		marginCategory drift.MarginRequirementType,
		liquidationBuffer *big.Int,
		strict bool, // false
		includeOpenOrders bool, // true
	) *big.Int
	GetInitialMarginRequirement() *big.Int
	GetMaintenanceMarginRequirement(liquidationBuffer *big.Int) *big.Int
	GetActivePerpPositionsForUserAccount(
		userAccount *drift.User,
	) []*drift.PerpPosition
	GetActivePerpPositions() []*drift.PerpPosition
	GetActiveSpotPositionsForUserAccount(
		userAccount *drift.User,
	) []*drift.SpotPosition
	GetActiveSpotPositions() []*drift.SpotPosition
	GetActiveSpotPositionsAndSlot() *accounts.DataAndSlot[[]*drift.SpotPosition]
	GetUnrealizedPNL(
		withFunding bool,
		marketIndex *uint16,
		withWeightMarginCategory *drift.MarginRequirementType,
		strict bool, // false
	) *big.Int
	GetUnrealizedFundingPNL(marketIndex *uint16) *big.Int
	GetSpotMarketAssetAndLiabilityValue(
		marketIndex *uint16,
		marginCategory *drift.MarginRequirementType,
		liquidationBuffer *big.Int,
		includeOpenOrders bool,
		strict bool, // false
		now int64,
	) (*big.Int, *big.Int)
	GetSpotMarketLiabilityValue(
		marketIndex *uint16,
		marginCategory *drift.MarginRequirementType,
		liquidationBuffer *big.Int,
		includeOpenOrders bool,
		strict bool, // false
		now int64,
	) *big.Int
	GetSpotLiabilityValue(
		tokenAmount *big.Int,
		strictOraclePrice *oracles.StrictOraclePrice,
		spotMarketAccount *drift.SpotMarket,
		marginCategory *drift.MarginRequirementType,
		liquidationBuffer *big.Int,
	) *big.Int
	GetSpotMarketAssetValue(
		marketIndex *uint16,
		marginCategory *drift.MarginRequirementType,
		includeOpenOrders bool,
		strict bool,
		now int64,
	) *big.Int
	GetSpotAssetValue(
		tokenAmount *big.Int,
		strictOraclePrice *oracles.StrictOraclePrice,
		spotMarketAccount *drift.SpotMarket,
		marginCategory *drift.MarginRequirementType,
	) *big.Int
	GetSpotPositionValue(
		marketIndex uint16,
		marginCategory *drift.MarginRequirementType,
		includeOpenOrders bool,
		strict bool,
		now int64,
	) *big.Int
	GetNetSpotMarketValue(withWeightMarginCategory *drift.MarginRequirementType) *big.Int
	GetTotalCollateral(
		marginCategory *drift.MarginRequirementType,
		strict bool,
	) *big.Int
	GetHealth() int64
	CalculateWeightedPerpPositionValue(
		perpPosition *drift.PerpPosition,
		marginCategory *drift.MarginRequirementType,
		liquidationBuffer *big.Int,
		includeOpenOrders bool,
		strict bool,
	) *big.Int
	GetPerpMarketLiabilityValue(
		marketIndex uint16,
		marginCategory *drift.MarginRequirementType,
		liquidationBuffer *big.Int,
		includeOpenOrders bool,
		strict bool,
	) *big.Int
	GetTotalPerpPositionValue(
		marginCategory *drift.MarginRequirementType,
		liquidationBuffer *big.Int,
		includeOpenOrders bool,
		strict bool,
	) *big.Int
	GetPerpPositionValue(
		marketIndex uint16,
		oraclePriceData *oracles.OraclePriceData,
		includeOpenOrders bool,
	) *big.Int
	GetPositionSide(
		currentPosition *drift.PerpPosition,
	) drift.PositionDirection
	GetPositionEstimatedExitPriceAndPnl(
		position *drift.PerpPosition,
		amountToClose *big.Int,
		useAMMClose bool,
	) (*big.Int, *big.Int)
	GetLeverage(includeOpenOrders bool) *big.Int
	CalculateLeverageFromComponents(
		perpLiabilityValue *big.Int,
		perpPnl *big.Int,
		spotAssetValue *big.Int,
		spotLiabilityValue *big.Int,
	) *big.Int
	GetLeverageComponents(
		includeOpenOrders bool,
		marginCategory *drift.MarginRequirementType,
	) (*big.Int, *big.Int, *big.Int, *big.Int)
	GetTotalLiabilityValue(
		marginCategory *drift.MarginRequirementType,
	) *big.Int
	GetTotalAssetValue(marginCategory *drift.MarginRequirementType) *big.Int
	GetNetUsdValue() *big.Int
	CanBeLiquidated() (bool, *big.Int, *big.Int)
	IsBeingLiquidated() bool
	GetSafestTiers() (uint8, uint8)
	IsBankrupt() bool
	GetOracleDataForPerpMarket(marketIndex uint16) *oracles.OraclePriceData
	GetOracleDataForSpotMarket(marketIndex uint16) *oracles.OraclePriceData
	GetUserFeeTier(drift.MarketType, *int64) *drift.FeeTier
}

type IUserStats interface {
	Subscribe(*drift.UserStats) bool
	FetchAccounts()
	Unsubscribe()
	GetAccountAndSlot() *accounts.DataAndSlot[*drift.UserStats]
	GetAccount() *drift.UserStats
	GetReferrerInfo() *go_drift.ReferrerInfo
	GetOldestActionTs(account *drift.UserStats) int64
	GetSubscriber() accounts.IUserStatsAccountSubscriber
	GetPublicKey() solana.PublicKey
}
