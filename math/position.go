package math

import (
	"driftgo/constants"
	"driftgo/lib/drift"
	oracles "driftgo/oracles/types"
	"driftgo/utils"
	"math/big"
)

// CalculateBaseAssetValue
/**
 * = market value of closing entire position
 * @param market
 * @param userPosition
 * @param oraclePriceData
 * @returns Base Asset Value. : Precision QUOTE_PRECISION
 */
func CalculateBaseAssetValue(
	market *drift.PerpMarket,
	userPosition *drift.PerpPosition,
	oraclePriceData *oracles.OraclePriceData,
	useSpread bool, // default true
	skipUpdate bool, // default false
) *big.Int {
	if userPosition.BaseAssetAmount == 0 {
		return utils.BN(0)
	}

	var directionToClose drift.PositionDirection = FindDirectionToClose(userPosition)
	var prepegAmm *drift.Amm

	if !skipUpdate {
		if market.Amm.BaseSpread > 0 && useSpread {
			baseAssetReserve, quoteAssetReserve, sqrtK, newPeg := CalculateUpdatedAMMSpreadReserves(
				&market.Amm,
				directionToClose,
				oraclePriceData,
			)
			prepegAmm = &drift.Amm{
				BaseAssetReserve:  utils.Uint128(baseAssetReserve),
				QuoteAssetReserve: utils.Uint128(quoteAssetReserve),
				SqrtK:             utils.Uint128(sqrtK),
				PegMultiplier:     utils.Uint128(newPeg),
			}
		} else {
			prepegAmm = CalculateUpdatedAMM(&market.Amm, oraclePriceData)
		}
	} else {
		prepegAmm = &market.Amm
	}
	newQuoteAssetReserve, _ := CalculateAmmReservesAfterSwap(
		prepegAmm,
		drift.AssetType_Base,
		utils.AbsX(utils.BN(userPosition.BaseAssetAmount)),
		GetSwapDirection(drift.AssetType_Base, directionToClose),
	)
	switch directionToClose {
	case drift.PositionDirection_Short:
		return utils.DivX(
			utils.MulX(
				utils.SubX(
					prepegAmm.QuoteAssetReserve.BigInt(),
					newQuoteAssetReserve,
				),
				prepegAmm.PegMultiplier.BigInt(),
			),
			constants.AMM_TIMES_PEG_TO_QUOTE_PRECISION_RATIO,
		)
	case drift.PositionDirection_Long:
		return utils.AddX(
			utils.DivX(
				utils.MulX(
					utils.SubX(
						newQuoteAssetReserve,
						prepegAmm.QuoteAssetReserve.BigInt(),
					),
					prepegAmm.PegMultiplier.BigInt(),
				),
				constants.AMM_TIMES_PEG_TO_QUOTE_PRECISION_RATIO,
			),
			utils.BN(1),
		)
	}
	return nil
}

// CalculatePositionPNL
/**
 * calculatePositionPNL
 * = BaseAssetAmount * (Avg Exit Price - Avg Entry Price)
 * @param market
 * @param PerpPosition
 * @param withFunding (adds unrealized funding payment pnl to result)
 * @param oraclePriceData
 * @returns BaseAssetAmount : Precision QUOTE_PRECISION
 */
func CalculatePositionPNL(
	market *drift.PerpMarket,
	perpPosition *drift.PerpPosition,
	withFunding bool, // false
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	if perpPosition.BaseAssetAmount == 0 {
		return utils.BN(perpPosition.QuoteAssetAmount)
	}

	baseAssetValue := CalculateBaseAssetValueWithOracle(
		market,
		perpPosition,
		oraclePriceData,
		false,
	)
	var baseAssetValueSign int64 = 1
	if perpPosition.BaseAssetAmount < 0 {
		baseAssetValueSign = -1
	}

	pnl := utils.AddX(utils.MulX(baseAssetValue, utils.BN(baseAssetValueSign)), utils.BN(perpPosition.QuoteAssetAmount))

	if withFunding {
		var fundingRatePnl *big.Int = CalculatePositionFundingPNL(market, perpPosition)
		pnl = utils.AddX(pnl, fundingRatePnl)
	}

	return pnl
}

func CalculateClaimablePnl(
	market *drift.PerpMarket,
	spotMarket *drift.SpotMarket,
	perpPosition *drift.PerpPosition,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	unrealizedPnl := CalculatePositionPNL(
		market,
		perpPosition,
		true,
		oraclePriceData,
	)

	unsettledPnl := utils.BN(0)
	unsettledPnl.Set(unrealizedPnl)
	if unrealizedPnl.Cmp(constants.ZERO) > 0 {
		excessPnlPool := utils.Max(
			utils.BN(0),
			utils.MulX(
				CalculateNetUserPnlImbalance(market, spotMarket, oraclePriceData),
				utils.BN(-1),
			),
		)

		maxPositivePnl := utils.AddX(
			utils.BN(max(perpPosition.QuoteAssetAmount-perpPosition.QuoteEntryAmount, 0)),
			excessPnlPool,
		)
		unsettledPnl = utils.Min(maxPositivePnl, unrealizedPnl)
	}
	return unsettledPnl
}

// CalculatePositionFundingPNL
/**
 *
 * @param market
 * @param PerpPosition
 * @returns // QUOTE_PRECISION
 */
func CalculatePositionFundingPNL(
	market *drift.PerpMarket,
	perpPosition *drift.PerpPosition,
) *big.Int {
	if perpPosition.BaseAssetAmount == 0 {
		return utils.BN(0)
	}

	var ammCumulativeFundingRate *big.Int
	if perpPosition.BaseAssetAmount > 0 {
		ammCumulativeFundingRate = market.Amm.CumulativeFundingRateLong.BigInt()
	} else {
		ammCumulativeFundingRate = market.Amm.CumulativeFundingRateShort.BigInt()
	}

	perpPositionFundingRate := utils.MulX(
		utils.DivX(
			utils.DivX(
				utils.MulX(
					utils.SubX(
						ammCumulativeFundingRate,
						utils.BN(perpPosition.LastCumulativeFundingRate),
					),
					utils.BN(perpPosition.BaseAssetAmount),
				),
				constants.AMM_RESERVE_PRECISION,
			),
			constants.FUNDING_RATE_BUFFER_PRECISION,
		),
		utils.BN(-1),
	)

	return perpPositionFundingRate
}

func PositionIsAvailable(position *drift.PerpPosition) bool {
	return position.BaseAssetAmount == 0 &&
		position.OpenOrders == 0 &&
		position.QuoteAssetAmount == 0 &&
		position.LpShares == 0
}

// CalculateBreakEvenPrice
/**
 *
 * @param userPosition
 * @returns Precision: PRICE_PRECISION (10^6)
 */
func CalculateBreakEvenPrice(userPosition *drift.PerpPosition) *big.Int {
	if userPosition.BaseAssetAmount == 0 {
		return utils.BN(0)
	}

	return utils.AbsX(
		utils.DivX(
			utils.MulX(
				utils.MulX(
					utils.BN(userPosition.QuoteBreakEvenAmount),
					constants.PRICE_PRECISION,
				),
				constants.AMM_TO_QUOTE_PRECISION_RATIO,
			),
			utils.BN(userPosition.BaseAssetAmount),
		),
	)
}

//CalculateEntryPrice
/**
 *
 * @param userPosition
 * @returns Precision: PRICE_PRECISION (10^6)
 */
func CalculateEntryPrice(userPosition *drift.PerpPosition) *big.Int {
	if userPosition.BaseAssetAmount == 0 {
		return utils.BN(0)
	}

	return utils.AbsX(
		utils.DivX(
			utils.MulX(
				utils.MulX(
					utils.BN(userPosition.QuoteEntryAmount),
					constants.PRICE_PRECISION,
				),
				constants.AMM_TO_QUOTE_PRECISION_RATIO,
			),
			utils.BN(userPosition.BaseAssetAmount),
		),
	)
}

// CalculateCostBasis
/**
 *
 * @param userPosition
 * @returns Precision: PRICE_PRECISION (10^10)
 */
func CalculateCostBasis(
	userPosition *drift.PerpPosition,
	includeSettledPnl bool,
) *big.Int {
	if userPosition.BaseAssetAmount == 0 {
		return utils.BN(0)
	}

	settledPnl := utils.BN(0)
	if includeSettledPnl {
		settledPnl.SetInt64(userPosition.SettledPnl)
	}
	return utils.AbsX(
		utils.DivX(
			utils.MulX(
				utils.MulX(
					utils.AddX(
						utils.BN(userPosition.QuoteEntryAmount),
						settledPnl,
					),
					constants.PRICE_PRECISION,
				),
				constants.AMM_TO_QUOTE_PRECISION_RATIO,
			),
			utils.BN(userPosition.BaseAssetAmount),
		),
	)
}

func FindDirectionToClose(
	userPosition *drift.PerpPosition,
) drift.PositionDirection {
	if userPosition.BaseAssetAmount > 0 {
		return drift.PositionDirection_Short
	} else {
		return drift.PositionDirection_Long
	}
}

func PositionCurrentDirection(
	userPosition *drift.PerpPosition,
) drift.PositionDirection {
	if userPosition.BaseAssetAmount >= 0 {
		return drift.PositionDirection_Long
	} else {
		return drift.PositionDirection_Short
	}
}

func IsEmptyPosition(userPosition *drift.PerpPosition) bool {
	return userPosition.BaseAssetAmount == 0 && userPosition.OpenOrders == 0
}

func HasOpenOrders(position *drift.PerpPosition) bool {
	return position.OpenOrders != 0 ||
		position.OpenBids != 0 ||
		position.OpenAsks != 0
}
