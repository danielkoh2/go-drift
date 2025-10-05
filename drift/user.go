package drift

import (
	"context"
	"driftgo/accounts"
	"driftgo/constants"
	"driftgo/drift/types"
	"driftgo/lib/drift"
	"driftgo/lib/event"
	"driftgo/math"
	oracles "driftgo/oracles/types"
	"driftgo/utils"
	math2 "math"
	"math/big"
	"time"

	"github.com/gagliardetto/solana-go"
)

type User struct {
	types.IUser
	driftClient          types.IDriftClient
	userAccountPublicKey solana.PublicKey
	accountSubscriber    accounts.IUserAccountSubscriber
	_isSubscribed        bool
	eventEmitter         *event.EventEmitter
}

func NewUser(config UserConfig) *User {
	user := &User{
		driftClient:          config.DriftClient,
		userAccountPublicKey: config.UserAccountPublicKey,
		eventEmitter:         config.EventEmitter,
	}
	if config.AccountSubscription.UseCustom {
		user.accountSubscriber = config.AccountSubscription.CustomSubscriber
	} else {
		user.accountSubscriber = accounts.CreateGeyserUserAccountSubscriber(
			config.DriftClient.GetProgram(),
			config.UserAccountPublicKey,
			config.AccountSubscription.ResubTimeoutMs,
			config.AccountSubscription.Commitment,
		)
	}
	return user
}

func (p *User) Subscribed() bool {
	return p._isSubscribed && p.accountSubscriber.Subscribed()
}

func (p *User) Subscribe(userAccount *drift.User) bool {
	if p._isSubscribed {
		return true
	}
	p._isSubscribed = p.accountSubscriber.Subscribe(userAccount)
	return p._isSubscribed
}

func (p *User) Unsubscribe() {
	if !p._isSubscribed {
		return
	}
	p.accountSubscriber.Unsubscribe()
	p._isSubscribed = false
}

func (p *User) UpdateData(userAccount *drift.User, slot uint64) {
	p.accountSubscriber.UpdateData(userAccount, slot)
}
func (p *User) FetchAccounts() {
	p.accountSubscriber.Fetch()
}

func (p *User) GetUserAccount() *drift.User {
	dataAndSlot := p.accountSubscriber.GetUserAccountAndSlot()
	if dataAndSlot == nil {
		return nil
	}
	return dataAndSlot.Data
}

func (p *User) ForceGetUserAccount() *drift.User {
	p.accountSubscriber.Fetch()
	dataAndSlot := p.accountSubscriber.GetUserAccountAndSlot()
	if dataAndSlot == nil {
		return nil
	}
	return dataAndSlot.Data
}

func (p *User) GetUserAccountAndSlot() *accounts.DataAndSlot[*drift.User] {
	return p.accountSubscriber.GetUserAccountAndSlot()
}

func (p *User) GetPerpPositionForUserAccount(
	userAccount *drift.User,
	marketIndex uint16,
) *drift.PerpPosition {
	for _, perpPosition := range p.GetActivePerpPositionsForUserAccount(userAccount) {
		if perpPosition.MarketIndex == marketIndex {
			return perpPosition
		}
	}
	return nil
}

func (p *User) GetPerpPosition(marketIndex uint16) *drift.PerpPosition {
	return p.GetPerpPositionForUserAccount(p.GetUserAccount(), marketIndex)
}

func (p *User) GetPerpPositionAndSlot(marketIndex uint16) *accounts.DataAndSlot[*drift.PerpPosition] {
	userAccount := p.GetUserAccountAndSlot()
	perpPosition := p.GetPerpPositionForUserAccount(
		userAccount.Data,
		marketIndex,
	)
	return &accounts.DataAndSlot[*drift.PerpPosition]{
		Data: perpPosition,
		Slot: userAccount.Slot,
	}
}

func (p *User) GetSpotPositionForUserAccount(
	userAccount *drift.User,
	marketIndex uint16,
) *drift.SpotPosition {
	for _, spotPosition := range p.GetActiveSpotPositionsForUserAccount(userAccount) {
		if spotPosition.MarketIndex == marketIndex {
			return spotPosition
		}
	}
	return nil
}

func (p *User) GetSpotPosition(marketIndex uint16) *drift.SpotPosition {
	return p.GetSpotPositionForUserAccount(p.GetUserAccount(), marketIndex)
}

func (p *User) GetSpotPositionAndSlot(marketIndex uint16) *accounts.DataAndSlot[*drift.SpotPosition] {
	userAccount := p.GetUserAccountAndSlot()
	spotPosition := p.GetSpotPositionForUserAccount(
		userAccount.Data,
		marketIndex,
	)
	return &accounts.DataAndSlot[*drift.SpotPosition]{
		Data: spotPosition,
		Slot: userAccount.Slot,
	}
}

func (p *User) GetEmptySpotPosition(marketIndex uint16) *drift.SpotPosition {
	return &drift.SpotPosition{
		MarketIndex:        marketIndex,
		ScaledBalance:      0,
		BalanceType:        drift.SpotBalanceType_Deposit,
		OpenBids:           0,
		OpenAsks:           0,
		CumulativeDeposits: 0,
		OpenOrders:         0,
		Padding:            [4]uint8{},
	}
}

// GetTokenAmount
/**
 * Returns the token amount for a given market. The spot market precision is based on the token mint decimals.
 * Positive if it is a deposit, negative if it is a borrow.
 *
 * @param marketIndex
 */
func (p *User) GetTokenAmount(marketIndex uint16) *big.Int {
	spotPosition := p.GetSpotPosition(marketIndex)
	if spotPosition == nil {
		return utils.BN(0)
	}
	var spotMarket *drift.SpotMarket = p.driftClient.GetSpotMarketAccount(marketIndex)
	return math.GetSignedTokenAmount(
		math.GetTokenAmount(
			utils.BN(spotPosition.ScaledBalance),
			spotMarket,
			spotPosition.BalanceType,
		),
		spotPosition.BalanceType,
	)
}

func (p *User) GetEmptyPosition(marketIndex uint16) *drift.PerpPosition {
	return &drift.PerpPosition{
		MarketIndex:               marketIndex,
		LastCumulativeFundingRate: 0,
		BaseAssetAmount:           0,
		QuoteAssetAmount:          0,
		QuoteBreakEvenAmount:      0,
		QuoteEntryAmount:          0,
		OpenBids:                  0,
		OpenAsks:                  0,
		SettledPnl:                0,
		LpShares:                  0,
		LastBaseAssetAmountPerLp:  0,
		LastQuoteAssetAmountPerLp: 0,
		//RemainderBaseAssetAmount:  0,
		OpenOrders: 0,
		PerLpBase:  0,
	}
}

func (p *User) GetClonedPosition(position *drift.PerpPosition) *drift.PerpPosition {
	perpPosition := *position
	return &perpPosition
}

func (p *User) GetOrderForUserAccount(
	userAccount *drift.User,
	orderId uint32,
) *drift.Order {
	var order *drift.Order = nil
	for idx := 0; idx < len(userAccount.Orders); idx++ {
		if userAccount.Orders[idx].OrderId == orderId {
			order = &userAccount.Orders[idx]
			break
		}
	}
	return order
}

func (p *User) GetOrder(orderId uint32) *drift.Order {
	userAccount := p.GetUserAccount()
	return p.GetOrderForUserAccount(userAccount, orderId)
}

func (p *User) GetOrderByUserIdForUserAccount(
	userAccount *drift.User,
	userOrderId uint8,
) *drift.Order {
	var order *drift.Order = nil
	for idx := 0; idx < len(userAccount.Orders); idx++ {
		if userAccount.Orders[idx].UserOrderId == userOrderId {
			order = &userAccount.Orders[idx]
			break
		}
	}
	return order
}

func (p *User) GetOrderByUserOrderId(userOrderId uint8) *drift.Order {
	userAccount := p.GetUserAccount()
	return p.GetOrderByUserIdForUserAccount(userAccount, userOrderId)
}

func (p *User) GetOrderByUserOrderIdAndSlot(userOrderId uint8) *accounts.DataAndSlot[*drift.Order] {
	userAccount := p.GetUserAccountAndSlot()
	order := p.GetOrderByUserIdForUserAccount(userAccount.Data, userOrderId)
	return &accounts.DataAndSlot[*drift.Order]{
		Data: order,
		Slot: userAccount.Slot,
	}
}

func (p *User) GetOpenOrdersForUserAccount(userAccount *drift.User) []*drift.Order {
	var orders []*drift.Order
	for idx := 0; idx < len(userAccount.Orders); idx++ {
		order := &userAccount.Orders[idx]
		if order.Status == drift.OrderStatus_Open {
			orders = append(orders, order)
		}
	}
	return orders
}

func (p *User) GetOpenOrders() []*drift.Order {
	userAccount := p.GetUserAccount()
	return p.GetOpenOrdersForUserAccount(userAccount)
}

func (p *User) GetUserAccountPublicKey() solana.PublicKey {
	return p.userAccountPublicKey
}

func (p *User) Exists() bool {
	userAccountInfo, err := p.driftClient.GetProgram().GetProvider().GetConnection().GetAccountInfo(context.TODO(), p.userAccountPublicKey)
	if err != nil || userAccountInfo == nil || userAccountInfo.Value == nil {
		return false
	}
	return true
}

// GetPerpBidAsks
/**
 * calculates the total open bids/asks in a perp market (including lps)
 * @returns : open bids
 * @returns : open asks
 */
func (p *User) GetPerpBidAsks(marketIndex uint16) (*big.Int, *big.Int) {
	position := p.GetPerpPosition(marketIndex)

	lpOpenBids, lpOpenAsks := p.GetLPBidAsks(marketIndex, nil)

	totalOpenBids := utils.AddX(lpOpenBids, utils.BN(position.OpenBids))
	totalOpenAsks := utils.AddX(lpOpenAsks, utils.BN(position.OpenAsks))

	return totalOpenBids, totalOpenAsks
}

// GetLPBidAsks
/**
 * calculates the open bids and asks for an lp
 * optionally pass in lpShares to see what bid/asks a user *would* take on
 * @returns : lp open bids
 * @returns : lp open asks
 */
func (p *User) GetLPBidAsks(marketIndex uint16, lpShares *big.Int) (*big.Int, *big.Int) {
	position := p.GetPerpPosition(marketIndex)

	lpSharesToCalc := utils.TT(lpShares != nil, lpShares, utils.BN(position.LpShares))

	if lpSharesToCalc == nil || lpSharesToCalc.Cmp(constants.ZERO) == 0 {
		return utils.BN(0), utils.BN(0)
	}

	var market *drift.PerpMarket = p.driftClient.GetPerpMarketAccount(marketIndex)
	marketOpenBids, marketOpenAsks := math.CalculateMarketOpenBidAsk(
		market.Amm.BaseAssetReserve.BigInt(),
		market.Amm.MinBaseAssetReserve.BigInt(),
		market.Amm.MaxBaseAssetReserve.BigInt(),
		utils.BN(market.Amm.OrderStepSize),
	)

	lpOpenBids := utils.DivX(utils.MulX(marketOpenBids, lpSharesToCalc), market.Amm.SqrtK.BigInt())
	lpOpenAsks := utils.DivX(utils.MulX(marketOpenAsks, lpSharesToCalc), market.Amm.SqrtK.BigInt())

	return lpOpenBids, lpOpenAsks
}

// GetPerpPositionWithLPSettle
/**
 * calculates the market position if the lp position was settled
 * @returns : the settled userPosition
 * @returns : the dust base asset amount (ie, < stepsize)
 * @returns : pnl from settle
 */
//func (p *User) GetPerpPositionWithLPSettle(
//	marketIndex uint16,
//	originalPosition *drift.PerpPosition,
//	burnLpShares bool,
//	includeRemainderInBaseAmount bool, // false
//) (*drift.PerpPosition, *big.Int, *big.Int) {
//	if originalPosition == nil {
//		originalPosition = p.GetPerpPosition(marketIndex)
//	}
//	if originalPosition == nil {
//		originalPosition = p.GetEmptyPosition(marketIndex)
//	}
//
//	if originalPosition.LpShares == 0 {
//		return originalPosition, utils.BN(0), utils.BN(0)
//	}
//
//	position := p.GetClonedPosition(originalPosition)
//	var market *drift.PerpMarket = p.driftClient.GetPerpMarketAccount(position.MarketIndex)
//
//	if market.Amm.PerLpBase != position.PerLpBase {
//		// perLpBase = 1 => per 10 LP shares, perLpBase = -1 => per 0.1 LP shares
//		expoDiff := market.Amm.PerLpBase - position.PerLpBase
//		marketPerLpRebaseScalar := int64(math2.Pow(10, math2.Abs(float64(expoDiff))))
//
//		if expoDiff > 0 {
//			position.LastBaseAssetAmountPerLp *= marketPerLpRebaseScalar
//			position.LastQuoteAssetAmountPerLp *= marketPerLpRebaseScalar
//		} else {
//			position.LastBaseAssetAmountPerLp /= marketPerLpRebaseScalar
//			position.LastQuoteAssetAmountPerLp /= marketPerLpRebaseScalar
//		}
//
//		position.PerLpBase += expoDiff
//	}
//
//	nShares := utils.BN(position.LpShares)
//
//	// incorp unsettled funding on pre settled position
//	var quoteFundingPnl *big.Int = math.CalculatePositionFundingPNL(market, position)
//
//	baseUnit := utils.IntX(constants.AMM_RESERVE_PRECISION)
//	if market.Amm.PerLpBase == position.PerLpBase {
//		if position.PerLpBase >= 0 && int64(position.PerLpBase) <= constants.AMM_RESERVE_PRECISION.Int64() {
//			marketPerLpRebase := utils.PowX(utils.BN(10), utils.BN(market.Amm.PerLpBase))
//			baseUnit = utils.MulX(baseUnit, marketPerLpRebase)
//		} else if position.PerLpBase < 0 && int64(position.PerLpBase) >= -constants.AMM_RESERVE_PRECISION.Int64() {
//			marketPerLpRebase := utils.PowX(utils.BN(10), utils.BN(market.Amm.PerLpBase))
//			baseUnit = utils.DivX(baseUnit, marketPerLpRebase)
//		} else {
//			panic("cannot calculate position per lp base")
//		}
//	} else {
//		panic("market.amm.perLpBase != position.perLpBase")
//	}
//
//	deltaBaa := utils.DivX(
//		utils.MulX(
//			utils.SubX(
//				market.Amm.BaseAssetAmountPerLp.BigInt(),
//				utils.BN(position.LastBaseAssetAmountPerLp),
//			),
//			nShares,
//		),
//		baseUnit,
//	)
//	deltaQaa := utils.DivX(
//		utils.MulX(
//			utils.SubX(
//				market.Amm.QuoteAssetAmountPerLp.BigInt(),
//				utils.BN(position.LastQuoteAssetAmountPerLp),
//			),
//			nShares,
//		),
//		baseUnit,
//	)
//
//	sign := func(v *big.Int) *big.Int {
//		return utils.TT(v.Sign() < 0, utils.BN(-1), utils.BN(1))
//	}
//
//	signN := func(v int64) int64 {
//		return utils.TT(v < 0, int64(-1), 1)
//	}
//	standardize := func(amount *big.Int, stepSize *big.Int) (*big.Int, *big.Int) {
//		remainder := utils.MulX(sign(amount), utils.ModX(utils.AbsX(amount), stepSize))
//		standardizedAmount := utils.SubX(amount, remainder)
//		return standardizedAmount, remainder
//	}
//
//	standardizedBaa, remainderBaa := standardize(deltaBaa, utils.BN(market.Amm.OrderStepSize))
//
//	position.RemainderBaseAssetAmount += int32(remainderBaa.Int64())
//
//	if int64(math2.Abs(float64(position.RemainderBaseAssetAmount))) > int64(market.Amm.OrderStepSize) {
//		newStandardizedBaa, newRemainderBaa := standardize(
//			utils.BN(position.RemainderBaseAssetAmount),
//			utils.BN(market.Amm.OrderStepSize),
//		)
//		position.BaseAssetAmount += newStandardizedBaa.Int64()
//		position.RemainderBaseAssetAmount = int32(newRemainderBaa.Int64())
//	}
//
//	dustBaseAssetValue := utils.BN(0)
//	if burnLpShares && position.RemainderBaseAssetAmount != 0 {
//		var oraclePriceData *oracles.OraclePriceData = p.driftClient.GetOracleDataForPerpMarket(position.MarketIndex)
//		dustBaseAssetValue = utils.AddX(
//			utils.BN(1),
//			utils.DivX(
//				utils.MulX(
//					utils.AbsX(
//						utils.BN(position.RemainderBaseAssetAmount),
//					),
//					oraclePriceData.Price,
//				),
//				constants.AMM_RESERVE_PRECISION,
//			),
//		)
//	}
//
//	var updateType string
//	if position.BaseAssetAmount == 0 {
//		updateType = "open"
//	} else if signN(position.BaseAssetAmount) == sign(deltaBaa).Int64() {
//		updateType = "increase"
//	} else if utils.AbsX(utils.BN(position.BaseAssetAmount)).Cmp(utils.AbsX(deltaBaa)) > 0 {
//		updateType = "reduce"
//	} else if utils.AbsX(utils.BN(position.BaseAssetAmount)).Cmp(utils.AbsX(deltaBaa)) == 0 {
//		updateType = "close"
//	} else {
//		updateType = "flip"
//	}
//
//	var newQuoteEntry int64
//	var pnl *big.Int
//	if updateType == "open" || updateType == "increase" {
//		newQuoteEntry = position.QuoteEntryAmount + deltaQaa.Int64()
//		pnl = utils.BN(0)
//	} else if updateType == "reduce" || updateType == "close" {
//		newQuoteEntry = position.QuoteEntryAmount - (position.QuoteEntryAmount * utils.AbsX(deltaBaa).Int64() / utils.AbsN(position.BaseAssetAmount))
//		pnl = utils.AddX(utils.BN(position.QuoteEntryAmount-newQuoteEntry), deltaQaa)
//	} else {
//		newQuoteEntry = deltaQaa.Int64() - deltaQaa.Int64()*utils.AbsN(position.BaseAssetAmount)/utils.AbsX(deltaBaa).Int64()
//		pnl = utils.AddX(utils.BN(position.QuoteEntryAmount), utils.SubX(deltaQaa, utils.BN(newQuoteEntry)))
//	}
//	position.QuoteEntryAmount = newQuoteEntry
//	position.BaseAssetAmount += standardizedBaa.Int64()
//	position.QuoteAssetAmount += deltaQaa.Int64() + quoteFundingPnl.Int64() - dustBaseAssetValue.Int64()
//	position.QuoteBreakEvenAmount += deltaQaa.Int64() + quoteFundingPnl.Int64() - dustBaseAssetValue.Int64()
//
//	// update open bids/asks
//	marketOpenBids, marketOpenAsks := math.CalculateMarketOpenBidAsk(
//		market.Amm.BaseAssetReserve.BigInt(),
//		market.Amm.MinBaseAssetReserve.BigInt(),
//		market.Amm.MaxBaseAssetReserve.BigInt(),
//		utils.BN(market.Amm.OrderStepSize),
//	)
//	lpOpenBids := utils.DivX(utils.MulX(marketOpenBids, utils.BN(position.LpShares)), market.Amm.SqrtK.BigInt())
//	lpOpenAsks := utils.DivX(utils.MulX(marketOpenAsks, utils.BN(position.LpShares)), market.Amm.SqrtK.BigInt())
//	position.OpenBids += lpOpenBids.Int64()
//	position.OpenAsks += lpOpenAsks.Int64()
//
//	// eliminate counting funding on settled position
//	if position.BaseAssetAmount > 0 {
//		position.LastCumulativeFundingRate = market.Amm.CumulativeFundingRateLong.BigInt().Int64()
//	} else if position.BaseAssetAmount < 0 {
//		position.LastCumulativeFundingRate = market.Amm.CumulativeFundingRateShort.BigInt().Int64()
//	} else {
//		position.LastCumulativeFundingRate = 0
//	}
//
//	remainderBeforeRemoval := position.RemainderBaseAssetAmount
//
//	if includeRemainderInBaseAmount {
//		position.BaseAssetAmount += int64(remainderBeforeRemoval)
//		position.RemainderBaseAssetAmount = 0
//	}
//	return position, utils.BN(remainderBeforeRemoval), pnl
//}

func (p *User) GetPerpBuyingPower(marketIndex uint16, collateralBuffer *big.Int) *big.Int {
	perpPosition, _, _ := p.GetPerpPositionWithLPSettle(
		marketIndex,
		nil,
		true,
		false,
	)
	worstCaseBaseAssetAmount := utils.TTM[*big.Int](perpPosition != nil, func() *big.Int { return math.CalculateWorstCaseBaseAssetAmount(perpPosition) }, utils.BN(0))

	freeCollateral := utils.SubX(p.GetFreeCollateral(utils.NewPtr(drift.MarginRequirementType_Initial)), collateralBuffer)

	return p.getPerpBuyingPowerFromFreeCollateralAndBaseAssetAmount(
		marketIndex,
		freeCollateral,
		worstCaseBaseAssetAmount,
	)
}

func (p *User) getPerpBuyingPowerFromFreeCollateralAndBaseAssetAmount(
	marketIndex uint16,
	freeCollateral *big.Int,
	baseAssetAmount *big.Int,
) *big.Int {
	marginRatio := math.CalculateMarketMarginRatio(
		p.driftClient.GetPerpMarketAccount(marketIndex),
		baseAssetAmount,
		drift.MarginRequirementType_Initial,
		int64(p.GetUserAccount().MaxMarginRatio),
	)
	return utils.DivX(utils.MulX(freeCollateral, constants.MARGIN_PRECISION), utils.BN(marginRatio))
}

// GetFreeCollateral
/**
 * calculates Free Collateral = Total collateral - margin requirement
 * @returns : Precision QUOTE_PRECISION
 */
func (p *User) GetFreeCollateral(marginCategory *drift.MarginRequirementType) *big.Int {
	if marginCategory == nil {
		marginCategory = utils.NewPtr(drift.MarginRequirementType_Initial)
	}
	totalCollateral := p.GetTotalCollateral(marginCategory, true)
	marginRequirement := utils.TTM[*big.Int](
		*marginCategory == drift.MarginRequirementType_Initial,
		func() *big.Int { return p.GetInitialMarginRequirement() },
		func() *big.Int { return p.GetMaintenanceMarginRequirement(nil) },
	)
	freeCollateral := utils.SubX(totalCollateral, marginRequirement)
	return utils.TT(freeCollateral.Cmp(constants.ZERO) >= 0, freeCollateral, utils.BN(0))
}

func (p *User) GetMarginRequirement(
	marginCategory drift.MarginRequirementType,
	liquidationBuffer *big.Int,
	strict bool, // false
	includeOpenOrders bool, // true
) *big.Int {
	return utils.AddX(p.GetTotalPerpPositionValue(
		&marginCategory,
		liquidationBuffer,
		includeOpenOrders,
		strict,
	), p.GetSpotMarketLiabilityValue(
		nil,
		&marginCategory,
		liquidationBuffer,
		includeOpenOrders,
		strict,
		0,
	),
	)
}

// GetInitialMarginRequirement
/**
 * @returns The initial margin requirement in USDC. : QUOTE_PRECISION
 */
func (p *User) GetInitialMarginRequirement() *big.Int {
	return p.GetMarginRequirement(drift.MarginRequirementType_Initial, nil, true, true)
}

// GetMaintenanceMarginRequirement
/**
 * @returns The maintenance margin requirement in USDC. : QUOTE_PRECISION
 */
func (p *User) GetMaintenanceMarginRequirement(liquidationBuffer *big.Int) *big.Int {
	return p.GetMarginRequirement(drift.MarginRequirementType_Maintenance, liquidationBuffer, false, true)
}

func (p *User) GetActivePerpPositionsForUserAccount(
	userAccount *drift.User,
) []*drift.PerpPosition {
	var positions []*drift.PerpPosition
	for idx := 0; idx < len(userAccount.PerpPositions); idx++ {
		pos := &userAccount.PerpPositions[idx]
		if pos.BaseAssetAmount != 0 || pos.QuoteAssetAmount != 0 || pos.OpenOrders != 0 || pos.LpShares != 0 {
			positions = append(positions, pos)
		}
	}
	return positions
}

func (p *User) GetActivePerpPositions() []*drift.PerpPosition {
	userAccount := p.GetUserAccount()
	return p.GetActivePerpPositionsForUserAccount(userAccount)
}

func (p *User) GetActivePerpPositionsAndSlot() *accounts.DataAndSlot[[]*drift.PerpPosition] {
	userAccount := p.GetUserAccountAndSlot()
	positions := p.GetActivePerpPositionsForUserAccount(userAccount.Data)
	return &accounts.DataAndSlot[[]*drift.PerpPosition]{
		Data: positions,
		Slot: userAccount.Slot,
	}
}

func (p *User) GetActiveSpotPositionsForUserAccount(
	userAccount *drift.User,
) []*drift.SpotPosition {
	var positions []*drift.SpotPosition
	for idx := 0; idx < len(userAccount.SpotPositions); idx++ {
		pos := &userAccount.SpotPositions[idx]
		if !math.IsSpotPositionAvailable(pos) {
			positions = append(positions, pos)
		}
	}
	return positions
}

func (p *User) GetActiveSpotPositions() []*drift.SpotPosition {
	userAccount := p.GetUserAccount()
	return p.GetActiveSpotPositionsForUserAccount(userAccount)
}

func (p *User) GetActiveSpotPositionsAndSlot() *accounts.DataAndSlot[[]*drift.SpotPosition] {
	userAccount := p.GetUserAccountAndSlot()
	positions := p.GetActiveSpotPositionsForUserAccount(userAccount.Data)
	return &accounts.DataAndSlot[[]*drift.SpotPosition]{
		Data: positions,
		Slot: userAccount.Slot,
	}
}

// GetUnrealizedPNL
/**
 * calculates unrealized position price pnl
 * @returns : Precision QUOTE_PRECISION
 */
func (p *User) GetUnrealizedPNL(
	withFunding bool,
	marketIndex *uint16,
	withWeightMarginCategory *drift.MarginRequirementType,
	strict bool, // false
) *big.Int {
	unrealizedPnl := utils.BN(0)

	for _, perpPosition := range p.GetActivePerpPositions() {
		if marketIndex == nil || perpPosition.MarketIndex == *marketIndex {
			market := p.driftClient.GetPerpMarketAccount(perpPosition.MarketIndex)
			oraclePriceData := p.GetOracleDataForPerpMarket(market.MarketIndex)

			quoteSpotMarket := p.driftClient.GetSpotMarketAccount(market.QuoteSpotMarketIndex)
			quoteOraclePriceData := p.GetOracleDataForSpotMarket(market.QuoteSpotMarketIndex)

			if perpPosition.LpShares > 0 {
				perpPosition, _, _ = p.GetPerpPositionWithLPSettle(
					perpPosition.MarketIndex,
					nil,
					withWeightMarginCategory != nil,
					false,
				)
			}

			positionUnrealizedPnl := math.CalculatePositionPNL(
				market,
				perpPosition,
				withFunding,
				oraclePriceData,
			)

			var quotePrice *big.Int
			if strict && positionUnrealizedPnl.Cmp(constants.ZERO) > 0 {
				quotePrice = utils.Min(
					quoteOraclePriceData.Price,
					utils.BN(quoteSpotMarket.HistoricalOracleData.LastOraclePriceTwap5Min),
				)
			} else if strict && positionUnrealizedPnl.Cmp(constants.ZERO) < 0 {
				quotePrice = utils.Max(
					quoteOraclePriceData.Price,
					utils.BN(quoteSpotMarket.HistoricalOracleData.LastOraclePriceTwap5Min),
				)
			} else {
				quotePrice = quoteOraclePriceData.Price
			}

			positionUnrealizedPnl = utils.DivX(
				utils.MulX(
					positionUnrealizedPnl,
					quotePrice,
				),
				constants.PRICE_PRECISION,
			)

			if withWeightMarginCategory != nil {
				if positionUnrealizedPnl.Cmp(constants.ZERO) > 0 {
					positionUnrealizedPnl = utils.DivX(
						utils.MulX(
							positionUnrealizedPnl,
							math.CalculateUnrealizedAssetWeight(
								market,
								quoteSpotMarket,
								positionUnrealizedPnl,
								*withWeightMarginCategory,
								oraclePriceData,
							),
						),
						constants.SPOT_MARKET_WEIGHT_PRECISION,
					)
				}
			}
			unrealizedPnl = utils.AddX(unrealizedPnl, positionUnrealizedPnl)
		}
	}
	return unrealizedPnl
}

//GetUnrealizedFundingPNL
/**
 * calculates unrealized funding payment pnl
 * @returns : Precision QUOTE_PRECISION
 */
func (p *User) GetUnrealizedFundingPNL(marketIndex *uint16) *big.Int {
	pnl := utils.BN(0)
	for idx := 0; idx < len(p.GetUserAccount().PerpPositions); idx++ {
		perpPosition := &p.GetUserAccount().PerpPositions[idx]
		if marketIndex == nil || perpPosition.MarketIndex == *marketIndex {
			market := p.driftClient.GetPerpMarketAccount(
				perpPosition.MarketIndex,
			)

			pnl = utils.AddX(pnl, math.CalculatePositionFundingPNL(market, perpPosition))
		}
	}
	return pnl
}

func (p *User) GetSpotMarketAssetAndLiabilityValue(
	marketIndex *uint16,
	marginCategory *drift.MarginRequirementType,
	liquidationBuffer *big.Int,
	includeOpenOrders bool,
	strict bool, // false
	now int64,
) (*big.Int, *big.Int) {
	if now <= 0 {
		now = time.Now().Unix()
	}
	netQuoteValue := utils.BN(0)
	totalAssetValue := utils.BN(0)
	totalLiabilityValue := utils.BN(0)
	for idx := 0; idx < len(p.GetUserAccount().SpotPositions); idx++ {
		spotPosition := &p.GetUserAccount().SpotPositions[idx]
		countForBase := marketIndex == nil || spotPosition.MarketIndex == *marketIndex

		countForQuote := marketIndex == nil ||
			*marketIndex == constants.QUOTE_SPOT_MARKET_INDEX ||
			(includeOpenOrders && spotPosition.OpenOrders != 0)
		if math.IsSpotPositionAvailable(spotPosition) ||
			(!countForBase && !countForQuote) {
			continue
		}

		spotMarketAccount := p.driftClient.GetSpotMarketAccount(spotPosition.MarketIndex)

		oraclePriceData := p.GetOracleDataForSpotMarket(
			spotPosition.MarketIndex,
		)
		twap5min := utils.BN(0)
		if strict {
			twap5min = math.CalculateLiveOracleTwap(
				&spotMarketAccount.HistoricalOracleData,
				oraclePriceData,
				now,
				constants.FIVE_MINUTE.Int64(),
			)
		}
		strictOraclePrice := &oracles.StrictOraclePrice{
			oraclePriceData.Price,
			twap5min,
		}

		if spotPosition.MarketIndex == constants.QUOTE_SPOT_MARKET_INDEX && countForQuote {
			tokenAmount := math.GetSignedTokenAmount(
				math.GetTokenAmount(
					utils.BN(spotPosition.ScaledBalance),
					spotMarketAccount,
					spotPosition.BalanceType,
				),
				spotPosition.BalanceType,
			)
			if spotPosition.BalanceType == drift.SpotBalanceType_Borrow {
				weightedTokenValue := utils.AbsX(p.GetSpotLiabilityValue(
					tokenAmount,
					strictOraclePrice,
					spotMarketAccount,
					marginCategory,
					liquidationBuffer,
				))

				netQuoteValue = utils.SubX(netQuoteValue, weightedTokenValue)
			} else {
				weightedTokenValue := p.GetSpotAssetValue(
					tokenAmount,
					strictOraclePrice,
					spotMarketAccount,
					marginCategory,
				)

				netQuoteValue = utils.AddX(netQuoteValue, weightedTokenValue)
			}
			continue
		}

		if !includeOpenOrders && countForBase {
			if spotPosition.BalanceType == drift.SpotBalanceType_Borrow {
				tokenAmount := math.GetSignedTokenAmount(
					math.GetTokenAmount(
						utils.BN(spotPosition.ScaledBalance),
						spotMarketAccount,
						spotPosition.BalanceType,
					),
					drift.SpotBalanceType_Borrow,
				)
				liabilityValue := utils.AbsX(p.GetSpotLiabilityValue(
					tokenAmount,
					strictOraclePrice,
					spotMarketAccount,
					marginCategory,
					liquidationBuffer,
				))
				totalLiabilityValue = utils.AddX(totalLiabilityValue, liabilityValue)
				continue
			} else {
				tokenAcmount := math.GetTokenAmount(
					utils.BN(spotPosition.ScaledBalance),
					spotMarketAccount,
					spotPosition.BalanceType,
				)
				assetValue := p.GetSpotAssetValue(
					tokenAcmount,
					strictOraclePrice,
					spotMarketAccount,
					marginCategory,
				)
				totalAssetValue = utils.AddX(totalAssetValue, assetValue)
				continue
			}
		}
		orderFillSimulation := math.GetWorstCaseTokenAmounts(
			spotPosition,
			spotMarketAccount,
			strictOraclePrice,
			*marginCategory,
			int64(p.GetUserAccount().MaxMarginRatio),
		)
		worstCaseTokenAmount := orderFillSimulation.TokenAmount
		worstCaseQuoteTokenAmount := orderFillSimulation.OrdersValue

		if worstCaseTokenAmount.Cmp(constants.ZERO) > 0 && countForBase {
			baseAssetValue := p.GetSpotAssetValue(
				worstCaseTokenAmount,
				strictOraclePrice,
				spotMarketAccount,
				marginCategory,
			)
			totalAssetValue = utils.AddX(totalAssetValue, baseAssetValue)
		}

		if worstCaseTokenAmount.Cmp(constants.ZERO) < 0 && countForBase {
			baseLiabilityValue := utils.AbsX(p.GetSpotLiabilityValue(
				worstCaseTokenAmount,
				strictOraclePrice,
				spotMarketAccount,
				marginCategory,
				nil,
			))
			totalLiabilityValue = utils.AddX(totalLiabilityValue, baseLiabilityValue)
		}

		if worstCaseQuoteTokenAmount.Cmp(constants.ZERO) > 0 && countForQuote {
			netQuoteValue = utils.AddX(netQuoteValue, worstCaseQuoteTokenAmount)
		}

		if worstCaseQuoteTokenAmount.Cmp(constants.ZERO) < 0 && countForQuote {
			weight := utils.IntX(constants.SPOT_MARKET_WEIGHT_PRECISION)
			if *marginCategory == drift.MarginRequirementType_Initial {
				weight = utils.Max(weight, utils.BN(p.GetUserAccount().MaxMarginRatio))
			}

			weightedTokenValue := utils.DivX(
				utils.MulX(
					utils.AbsX(worstCaseQuoteTokenAmount),
					weight,
				),
				constants.SPOT_MARKET_WEIGHT_PRECISION,
			)

			netQuoteValue = utils.SubX(netQuoteValue, weightedTokenValue)
		}

		totalLiabilityValue = utils.AddX(
			totalLiabilityValue,
			utils.MulX(
				constants.OPEN_ORDER_MARGIN_REQUIREMENT,
				utils.BN(spotPosition.OpenOrders),
			),
		)
	}

	if marketIndex == nil || *marketIndex == constants.QUOTE_SPOT_MARKET_INDEX {
		if netQuoteValue.Cmp(constants.ZERO) > 0 {
			totalAssetValue = utils.AddX(totalAssetValue, netQuoteValue)
		} else {
			totalLiabilityValue = utils.AddX(totalLiabilityValue, utils.AbsX(netQuoteValue))
		}
	}
	return totalAssetValue, totalLiabilityValue
}

func (p *User) GetSpotMarketLiabilityValue(
	marketIndex *uint16,
	marginCategory *drift.MarginRequirementType,
	liquidationBuffer *big.Int,
	includeOpenOrders bool,
	strict bool, // false
	now int64,
) *big.Int {
	_, totalLiabilityValue := p.GetSpotMarketAssetAndLiabilityValue(
		marketIndex,
		marginCategory,
		liquidationBuffer,
		includeOpenOrders,
		strict,
		now,
	)
	return totalLiabilityValue
}

func (p *User) GetSpotLiabilityValue(
	tokenAmount *big.Int,
	strictOraclePrice *oracles.StrictOraclePrice,
	spotMarketAccount *drift.SpotMarket,
	marginCategory *drift.MarginRequirementType,
	liquidationBuffer *big.Int,
) *big.Int {
	liabilityValue := math.GetStrictTokenValue(
		tokenAmount,
		int64(spotMarketAccount.Decimals),
		strictOraclePrice,
	)

	if marginCategory != nil {
		weight := math.CalculateLiabilityWeight(
			tokenAmount,
			spotMarketAccount,
			*marginCategory,
		)

		if *marginCategory == drift.MarginRequirementType_Initial &&
			spotMarketAccount.MarketIndex != constants.QUOTE_SPOT_MARKET_INDEX {
			weight = utils.Max(
				weight,
				utils.AddX(
					constants.SPOT_MARKET_WEIGHT_PRECISION,
					utils.BN(p.GetUserAccount().MaxMarginRatio),
				),
			)
		}

		if liquidationBuffer != nil {
			weight = utils.AddX(weight, liquidationBuffer)
		}
		liabilityValue = utils.DivX(utils.MulX(liabilityValue, weight), constants.SPOT_MARKET_WEIGHT_PRECISION)
	}
	return liabilityValue
}

func (p *User) GetSpotMarketAssetValue(
	marketIndex *uint16,
	marginCategory *drift.MarginRequirementType,
	includeOpenOrders bool,
	strict bool,
	now int64,
) *big.Int {
	totalAssetValue, _ := p.GetSpotMarketAssetAndLiabilityValue(
		marketIndex,
		marginCategory,
		nil,
		includeOpenOrders,
		strict,
		now,
	)
	return totalAssetValue
}

func (p *User) GetSpotAssetValue(
	tokenAmount *big.Int,
	strictOraclePrice *oracles.StrictOraclePrice,
	spotMarketAccount *drift.SpotMarket,
	marginCategory *drift.MarginRequirementType,
) *big.Int {
	assetValue := math.GetStrictTokenValue(
		tokenAmount,
		int64(spotMarketAccount.Decimals),
		strictOraclePrice,
	)

	if marginCategory != nil {
		weight := math.CalculateAssetWeight(
			tokenAmount,
			strictOraclePrice.Current,
			spotMarketAccount,
			*marginCategory,
		)
		if *marginCategory == drift.MarginRequirementType_Initial &&
			spotMarketAccount.MarketIndex != constants.QUOTE_SPOT_MARKET_INDEX {
			userCustomAssetWeight := utils.Max(
				utils.BN(0),
				utils.SubX(
					constants.SPOT_MARKET_WEIGHT_PRECISION,
					utils.BN(p.GetUserAccount().MaxMarginRatio),
				),
			)
			weight = utils.Min(weight, userCustomAssetWeight)
		}
		assetValue = utils.DivX(utils.MulX(assetValue, weight), constants.SPOT_MARKET_WEIGHT_PRECISION)
	}
	return assetValue
}

func (p *User) GetSpotPositionValue(
	marketIndex uint16,
	marginCategory *drift.MarginRequirementType,
	includeOpenOrders bool,
	strict bool,
	now int64,
) *big.Int {
	totalAssetValue, totalLiabilityValue := p.GetSpotMarketAssetAndLiabilityValue(
		&marketIndex,
		marginCategory,
		nil,
		includeOpenOrders,
		strict,
		now,
	)
	return utils.SubX(totalAssetValue, totalLiabilityValue)
}

func (p *User) GetNetSpotMarketValue(withWeightMarginCategory *drift.MarginRequirementType) *big.Int {
	totalAssetValue, totalLiabilityValue := p.GetSpotMarketAssetAndLiabilityValue(
		nil,
		withWeightMarginCategory,
		nil,
		false,
		false,
		0)
	return utils.SubX(totalAssetValue, totalLiabilityValue)
}

// GetTotalCollateral
/**
 * calculates TotalCollateral: collateral + unrealized pnl
 * @returns : Precision QUOTE_PRECISION
 */
func (p *User) GetTotalCollateral(
	marginCategory *drift.MarginRequirementType,
	strict bool,
) *big.Int {
	if marginCategory == nil {
		marginCategory = utils.NewPtr(drift.MarginRequirementType_Initial)
	}
	return utils.AddX(
		p.GetSpotMarketAssetValue(
			nil,
			marginCategory,
			true,
			strict,
			0,
		),
		p.GetUnrealizedPNL(
			true,
			nil,
			marginCategory,
			strict,
		),
	)
}

// GetHealth
/**
 * calculates User Health by comparing total collateral and maint. margin requirement
 * @returns : number (value from [0, 100])
 */
func (p *User) GetHealth() int64 {
	if p.IsBeingLiquidated() {
		return 0
	}
	totalCollateral := p.GetTotalCollateral(utils.NewPtr(drift.MarginRequirementType_Maintenance), false)
	maintenanceMarginReq := p.GetMaintenanceMarginRequirement(nil)

	var health int64

	if maintenanceMarginReq.Cmp(constants.ZERO) == 0 && totalCollateral.Cmp(constants.ZERO) >= 0 {
		health = 100
	} else if totalCollateral.Cmp(constants.ZERO) <= 0 {
		health = 0
	} else {
		health = int64(math2.Round(
			min(
				100.0,
				max(
					0.0,
					(1.0-float64(maintenanceMarginReq.Int64())/float64(totalCollateral.Int64()))*100.0,
				),
			),
		))
	}
	return health
}

func (p *User) CalculateWeightedPerpPositionValue(
	perpPosition *drift.PerpPosition,
	marginCategory *drift.MarginRequirementType,
	liquidationBuffer *big.Int,
	includeOpenOrders bool,
	strict bool,
) *big.Int {
	market := p.driftClient.GetPerpMarketAccount(
		perpPosition.MarketIndex,
	)

	if perpPosition.LpShares > 0 {
		// is an lp, clone so we dont mutate the position
		perpPosition, _, _ = p.GetPerpPositionWithLPSettle(
			market.MarketIndex,
			p.GetClonedPosition(perpPosition),
			marginCategory != nil,
			false,
		)
	}

	valuationPrice := p.GetOracleDataForPerpMarket(
		market.MarketIndex,
	).Price

	if market.Status == drift.MarketStatus_Settlement {
		valuationPrice = utils.BN(market.ExpiryPrice)
	}

	baseAssetAmount := utils.BN(perpPosition.BaseAssetAmount)
	if includeOpenOrders {
		baseAssetAmount = math.CalculateWorstCaseBaseAssetAmount(perpPosition)
	}

	baseAssetValue := utils.DivX(utils.MulX(utils.AbsX(baseAssetAmount), valuationPrice), constants.BASE_PRECISION)

	if marginCategory != nil {
		marginRatio := utils.BN(math.CalculateMarketMarginRatio(
			market,
			utils.AbsX(baseAssetAmount),
			*marginCategory,
			int64(p.GetUserAccount().MaxMarginRatio),
		))

		if liquidationBuffer != nil {
			marginRatio = utils.AddX(marginRatio, liquidationBuffer)
		}

		if market.Status == drift.MarketStatus_Settlement {
			marginRatio = utils.BN(0)
		}

		quoteSpotMarket := p.driftClient.GetSpotMarketAccount(
			market.QuoteSpotMarketIndex,
		)
		quoteOraclePriceData := p.driftClient.GetOracleDataForSpotMarket(
			constants.QUOTE_SPOT_MARKET_INDEX,
		)

		var quotePrice *big.Int
		if strict {
			quotePrice = utils.Max(
				quoteOraclePriceData.Price,
				utils.BN(quoteSpotMarket.HistoricalOracleData.LastOraclePriceTwap5Min),
			)
		} else {
			quotePrice = quoteOraclePriceData.Price
		}

		baseAssetValue = utils.DivX(
			utils.MulX(
				utils.DivX(
					utils.MulX(
						baseAssetValue,
						quotePrice,
					),
					constants.PRICE_PRECISION,
				),
				marginRatio,
			),
			constants.MARGIN_PRECISION,
		)
		if includeOpenOrders {
			baseAssetValue = utils.AddX(
				baseAssetValue,
				utils.MulX(utils.BN(perpPosition.OpenOrders), constants.OPEN_ORDER_MARGIN_REQUIREMENT),
			)
		}
	}
	return baseAssetValue
}

func (p *User) GetPerpMarketLiabilityValue(
	marketIndex uint16,
	marginCategory *drift.MarginRequirementType,
	liquidationBuffer *big.Int,
	includeOpenOrders bool,
	strict bool,
) *big.Int {
	perpPosition := p.GetPerpPosition(marketIndex)
	return p.CalculateWeightedPerpPositionValue(
		perpPosition,
		marginCategory,
		liquidationBuffer,
		includeOpenOrders,
		strict,
	)
}

func (p *User) GetTotalPerpPositionValue(
	marginCategory *drift.MarginRequirementType,
	liquidationBuffer *big.Int,
	includeOpenOrders bool,
	strict bool,
) *big.Int {
	totalPerpValue := utils.BN(0)
	for _, perpPosition := range p.GetActivePerpPositions() {
		utils.AddX(
			totalPerpValue,
			p.CalculateWeightedPerpPositionValue(
				perpPosition,
				marginCategory,
				liquidationBuffer,
				includeOpenOrders,
				strict,
			),
		)
	}
	return totalPerpValue
}

func (p *User) GetPerpPositionValue(
	marketIndex uint16,
	oraclePriceData *oracles.OraclePriceData,
	includeOpenOrders bool,
) *big.Int {
	userPosition, _, _ := p.GetPerpPositionWithLPSettle(
		marketIndex,
		nil,
		false,
		true,
	)
	if userPosition == nil {
		userPosition = p.GetEmptyPosition(marketIndex)
	}
	market := p.driftClient.GetPerpMarketAccount(
		userPosition.MarketIndex,
	)
	return math.CalculateBaseAssetValueWithOracle(
		market,
		userPosition,
		oraclePriceData,
		includeOpenOrders,
	)
}

func (p *User) GetPositionSide(
	currentPosition *drift.PerpPosition,
) drift.PositionDirection {
	if currentPosition.BaseAssetAmount > 0 {
		return drift.PositionDirection_Long
	} else if currentPosition.BaseAssetAmount < 0 {
		return drift.PositionDirection_Short
	} else {
		return 99
	}
}

func (p *User) GetPositionEstimatedExitPriceAndPnl(
	position *drift.PerpPosition,
	amountToClose *big.Int,
	useAMMClose bool,
) (*big.Int, *big.Int) {
	market := p.driftClient.GetPerpMarketAccount(position.MarketIndex)

	entryPrice := math.CalculateEntryPrice(position)

	oraclePriceData := p.GetOracleDataForPerpMarket(position.MarketIndex)

	if amountToClose != nil {
		if amountToClose.Cmp(constants.ZERO) == 0 {
			return math.CalculateReservePrice(market, oraclePriceData), utils.BN(0)
		}
		position = &drift.PerpPosition{
			BaseAssetAmount:           amountToClose.Int64(),
			LastCumulativeFundingRate: position.LastCumulativeFundingRate,
			MarketIndex:               position.MarketIndex,
			QuoteAssetAmount:          position.QuoteAssetAmount,
		}
	}

	var baseAssetValue *big.Int

	if useAMMClose {
		baseAssetValue = math.CalculateBaseAssetValue(
			market,
			position,
			oraclePriceData,
			true,
			false,
		)
	} else {
		baseAssetValue = math.CalculateBaseAssetValueWithOracle(
			market,
			position,
			oraclePriceData,
			false,
		)
	}
	if position.BaseAssetAmount == 0 {
		return utils.BN(0), utils.BN(0)
	}

	exitPrice := utils.DivX(
		utils.MulX(
			baseAssetValue,
			constants.AMM_TO_QUOTE_PRECISION_RATIO,
			constants.PRICE_PRECISION,
		),
		utils.AbsX(
			utils.BN(position.BaseAssetAmount),
		),
	)

	pnlPerBase := utils.SubX(exitPrice, entryPrice)
	pnl := utils.DivX(
		utils.MulX(pnlPerBase, utils.BN(position.BaseAssetAmount)),
		constants.PRICE_PRECISION,
		constants.AMM_TO_QUOTE_PRECISION_RATIO,
	)
	return exitPrice, pnl
}

func (p *User) GetLeverage(includeOpenOrders bool) *big.Int {
	perpLiabilityValue, perpPnl, spotAssetValue, spotLiabilityValue := p.GetLeverageComponents(includeOpenOrders, nil)
	return p.CalculateLeverageFromComponents(
		perpLiabilityValue,
		perpPnl,
		spotAssetValue,
		spotLiabilityValue,
	)
}
func (p *User) CalculateLeverageFromComponents(
	perpLiabilityValue *big.Int,
	perpPnl *big.Int,
	spotAssetValue *big.Int,
	spotLiabilityValue *big.Int,
) *big.Int {
	totalLiabilityValue := utils.AddX(perpLiabilityValue, spotLiabilityValue)
	totalAssetValue := utils.AddX(spotAssetValue, perpPnl)
	netAssetValue := utils.SubX(totalAssetValue, spotLiabilityValue)

	if netAssetValue.Cmp(constants.ZERO) == 0 {
		return utils.BN(0)
	}
	return utils.DivX(
		utils.MulX(totalLiabilityValue, constants.TEN_THOUSAND),
		netAssetValue,
	)
}

func (p *User) GetLeverageComponents(
	includeOpenOrders bool,
	marginCategory *drift.MarginRequirementType,
) (*big.Int, *big.Int, *big.Int, *big.Int) {
	perpLiability := p.GetTotalPerpPositionValue(
		marginCategory,
		nil,
		includeOpenOrders,
		false,
	)
	perpPnl := p.GetUnrealizedPNL(true, nil, marginCategory, false)

	spotAssetValue, spotLiabilityValue := p.GetSpotMarketAssetAndLiabilityValue(
		nil,
		marginCategory,
		nil,
		includeOpenOrders,
		false,
		0,
	)
	return perpLiability, perpPnl, spotAssetValue, spotLiabilityValue
}

func (p *User) GetTotalLiabilityValue(
	marginCategory *drift.MarginRequirementType,
) *big.Int {
	return utils.AddX(p.GetTotalPerpPositionValue(
		marginCategory,
		nil,
		true,
		false,
	),
		p.GetSpotMarketLiabilityValue(
			nil,
			marginCategory,
			nil,
			true,
			false,
			0,
		),
	)
}

func (p *User) GetTotalAssetValue(marginCategory *drift.MarginRequirementType) *big.Int {
	return utils.AddX(
		p.GetSpotMarketAssetValue(nil, marginCategory, true, false, 0),
		p.GetUnrealizedPNL(true, nil, marginCategory, false),
	)
}

func (p *User) GetNetUsdValue() *big.Int {
	netSpotValue := p.GetNetSpotMarketValue(nil)
	unrealizedPnl := p.GetUnrealizedPNL(true, nil, nil, false)
	return utils.AddX(netSpotValue, unrealizedPnl)
}

func (p *User) CanBeLiquidated() (bool, *big.Int, *big.Int) {
	totalCollateral := p.GetTotalCollateral(utils.NewPtr(drift.MarginRequirementType_Maintenance), false)
	// if user being liq'd, can continue to be liq'd until total collateral above the margin requirement plus buffer
	var liquidationBuffer *big.Int
	if p.IsBeingLiquidated() {
		liquidationBuffer = utils.BN(p.driftClient.GetStateAccount().LiquidationMarginBufferRatio)
	}
	marginRequirement := p.GetMaintenanceMarginRequirement(liquidationBuffer)
	canBeLiquidated := totalCollateral.Cmp(marginRequirement) < 0

	return canBeLiquidated, marginRequirement, totalCollateral
}

func (p *User) IsBeingLiquidated() bool {
	return p.GetUserAccount().Status&(uint8(drift.UserStatus_BeingLiquidated)|uint8(drift.UserStatus_Bankrupt)) > 0
}

func (p *User) GetSafestTiers() (uint8, uint8) {
	safestPerpTier := uint8(4)
	safestSpotTier := uint8(4)

	for _, perpPosition := range p.GetActivePerpPositions() {
		safestPerpTier = min(
			safestPerpTier,
			math.GetPerpMarketTierNumber(
				p.driftClient.GetPerpMarketAccount(perpPosition.MarketIndex),
			),
		)
	}

	for _, spotPosition := range p.GetActiveSpotPositions() {
		if spotPosition.BalanceType == drift.SpotBalanceType_Deposit {
			continue
		}
		safestPerpTier = min(
			safestPerpTier,
			math.GetSpotMarketTierNumber(
				p.driftClient.GetSpotMarketAccount(spotPosition.MarketIndex),
			),
		)
	}
	return safestPerpTier, safestSpotTier
}

func (p *User) IsBankrupt() bool {
	return (p.GetUserAccount().Status & uint8(drift.UserStatus_Bankrupt)) > 0
}

func (p *User) GetOracleDataForPerpMarket(marketIndex uint16) *oracles.OraclePriceData {
	return p.driftClient.GetOracleDataForPerpMarket(marketIndex)
}

func (p *User) GetOracleDataForSpotMarket(marketIndex uint16) *oracles.OraclePriceData {
	return p.driftClient.GetOracleDataForSpotMarket(marketIndex)
}

func (p *User) GetUserFeeTier(marketType drift.MarketType, nowTs *int64) *drift.FeeTier {
	ts := utils.TTM[int64](nowTs == nil, time.Now().UnixMilli(), *nowTs)
	state := p.driftClient.GetStateAccount()

	feeTierIndex := 0
	if marketType == drift.MarketType_Perp {
		userStatsAccount := p.driftClient.GetUserStats().GetAccount()

		total30dVolume := utils.BN(math.GetUser30dRollingVolumeEstimate(
			userStatsAccount,
			ts,
		))

		stakedQuoteAssetAmount := utils.BN(userStatsAccount.IfStakedQuoteAssetAmount)
		volumeTiers := []*big.Int{
			utils.MulX(utils.BN(100_000_000), constants.QUOTE_PRECISION),
			utils.MulX(utils.BN(50_000_000), constants.QUOTE_PRECISION),
			utils.MulX(utils.BN(10_000_000), constants.QUOTE_PRECISION),
			utils.MulX(utils.BN(5_000_000), constants.QUOTE_PRECISION),
			utils.MulX(utils.BN(1_000_000), constants.QUOTE_PRECISION),
		}
		stakedTiers := []*big.Int{
			utils.MulX(utils.BN(10000), constants.QUOTE_PRECISION),
			utils.MulX(utils.BN(5000), constants.QUOTE_PRECISION),
			utils.MulX(utils.BN(2000), constants.QUOTE_PRECISION),
			utils.MulX(utils.BN(1000), constants.QUOTE_PRECISION),
			utils.MulX(utils.BN(500), constants.QUOTE_PRECISION),
		}

		for i, volumeTier := range volumeTiers {
			if total30dVolume.Cmp(volumeTier) >= 0 ||
				stakedQuoteAssetAmount.Cmp(stakedTiers[i]) >= 0 {
				feeTierIndex = 5 - i
				break
			}
		}
		return &state.PerpFeeStructure.FeeTiers[feeTierIndex]
	}
	return &state.PerpFeeStructure.FeeTiers[feeTierIndex]
}
