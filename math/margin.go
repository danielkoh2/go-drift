package math

import (
	"driftgo/constants"
	"driftgo/lib/drift"
	oracles "driftgo/oracles/types"
	"driftgo/utils"
	"math/big"
)

func CalculateSizePremiumLiabilityWeight(
	size *big.Int,
	imfFactor *big.Int,
	liabilityWeight *big.Int,
	precision *big.Int,
) *big.Int {
	if imfFactor.Cmp(constants.ZERO) == 0 {
		return liabilityWeight
	}

	sizeSqrt := utils.SquareRootBN(utils.AddX(utils.MulX(utils.AbsX(size), utils.BN(10)), utils.BN(1))) //1e9 -> 1e10 -> 1e5

	liabilityWeightNumerator := utils.SubX(liabilityWeight, utils.DivX(liabilityWeight, utils.BN(5)))

	denom := utils.DivX(
		utils.MulX(
			utils.BN(100000),
			constants.SPOT_MARKET_IMF_PRECISION,
		),
		precision,
	)
	// assert(denom.gt(ZERO))

	sizePremiumLiabilityWeight := utils.AddX(liabilityWeightNumerator, utils.DivX(utils.MulX(sizeSqrt, imfFactor), denom))

	maxLiabilityWeight := utils.Max(liabilityWeight, sizePremiumLiabilityWeight)
	return maxLiabilityWeight
}

func CalculateSizeDiscountAssetWeight(
	size *big.Int,
	imfFactor *big.Int,
	assetWeight *big.Int,
) *big.Int {
	if imfFactor.Cmp(constants.ZERO) == 0 {
		return assetWeight
	}
	sizeSqrt := utils.SquareRootBN(utils.AddX(utils.MulX(utils.AbsX(size), utils.BN(10)), utils.BN(1))) //1e9 -> 1e10 -> 1e5
	imfNumerator := utils.AddX(
		constants.SPOT_MARKET_IMF_PRECISION,
		utils.DivX(
			constants.SPOT_MARKET_IMF_PRECISION,
			utils.BN(10),
		),
	)

	sizeDiscountAssetWeight := utils.DivX(
		utils.MulX(
			imfNumerator,
			constants.SPOT_MARKET_IMF_PRECISION,
		),
		utils.AddX(
			constants.SPOT_MARKET_IMF_PRECISION,
			utils.DivX(
				utils.MulX(
					sizeSqrt,
					imfFactor,
				),
				utils.BN(100000),
			),
		),
	)

	minAssetWeight := utils.Min(assetWeight, sizeDiscountAssetWeight)

	return minAssetWeight
}

func CalculateOraclePriceForPerpMargin(
	perpPosition *drift.PerpPosition,
	market *drift.PerpMarket,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	oraclePriceOffset := utils.Min(
		utils.DivX(
			utils.MulX(
				utils.BN(market.Amm.MaxSpread),
				oraclePriceData.Price,
			),
			constants.BID_ASK_SPREAD_PRECISION,
		),
		utils.AddX(
			oraclePriceData.Confidence,
			utils.DivX(
				utils.MulX(
					utils.BN(market.Amm.BaseSpread),
					oraclePriceData.Price,
				),
				constants.BID_ASK_SPREAD_PRECISION,
			),
		),
	)

	var marginPrice *big.Int
	if perpPosition.BaseAssetAmount > 0 {
		marginPrice = utils.SubX(oraclePriceData.Price, oraclePriceOffset)
	} else {
		marginPrice = utils.AddX(oraclePriceData.Price, oraclePriceOffset)
	}

	return marginPrice
}
func CalculateBaseAssetValueWithOracle(
	market *drift.PerpMarket,
	perpPosition *drift.PerpPosition,
	oraclePriceData *oracles.OraclePriceData,
	includeOpenOrders bool, //false
) *big.Int {
	price := oraclePriceData.Price
	if market.Status == drift.MarketStatus_Settlement {
		price = utils.BN(market.ExpiryPrice)
	}

	baseAssetAmount := utils.TTM[*big.Int](includeOpenOrders, func() *big.Int { return CalculateWorstCaseBaseAssetAmount(perpPosition) }, utils.BN(perpPosition.BaseAssetAmount))

	return utils.DivX(utils.MulX(utils.AbsX(baseAssetAmount), price), constants.AMM_RESERVE_PRECISION)
}

func CalculateWorstCaseBaseAssetAmount(
	perpPosition *drift.PerpPosition,
) *big.Int {
	allBids := utils.BN(perpPosition.BaseAssetAmount + perpPosition.OpenBids)
	allAsks := utils.BN(perpPosition.BaseAssetAmount + perpPosition.OpenAsks)

	if utils.AbsX(allBids).Cmp(utils.AbsX(allAsks)) > 0 {
		return allBids
	} else {
		return allAsks
	}
}
