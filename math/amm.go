package math

import (
	"driftgo/constants"
	"driftgo/lib/drift"
	oracles "driftgo/oracles/types"
	"driftgo/utils"
	"fmt"
	"math"
	"math/big"
	"time"
)

func CalculatePegFromTargetPrice(
	targetPrice *big.Int,
	baseAssetReserve *big.Int,
	quoteAssetReserve *big.Int,
) *big.Int {
	return utils.Max(
		utils.DivX(
			utils.AddX(
				utils.DivX(utils.MulX(targetPrice, baseAssetReserve), quoteAssetReserve),
				utils.DivX(constants.PRICE_DIV_PEG, utils.BN(2)),
			), constants.PRICE_DIV_PEG),
		utils.BN(1),
	)
}

// line 51 OK
func CalculateOptimalPegAndBudget(amm *drift.Amm, oraclePriceData *oracles.OraclePriceData) (*big.Int, *big.Int, *big.Int, bool) {
	reservePriceBefore := CalculatePrice(
		amm.BaseAssetReserve.BigInt(),
		amm.QuoteAssetReserve.BigInt(),
		amm.PegMultiplier.BigInt(),
	)
	reservePriceBefore = utils.Max(utils.BN(1), reservePriceBefore)
	targetPrice := oraclePriceData.Price
	newPeg := CalculatePegFromTargetPrice(
		targetPrice,
		amm.BaseAssetReserve.BigInt(),
		amm.QuoteAssetReserve.BigInt(),
	)
	prePegCost := CalculateRepegCost(amm, newPeg)

	totalFeeLB := utils.DivX(amm.TotalExchangeFee.BigInt(), utils.BN(2))
	budget := utils.Max(utils.BN(0), utils.SubX(amm.TotalFeeMinusDistributions.BigInt(), totalFeeLB))

	checkLowerBound := true
	if budget.Cmp(prePegCost) < 0 {
		halfMaxPriceSpread := utils.DivX(utils.MulX(utils.DivX(utils.BN(int64(amm.MaxSpread)), utils.BN(2)), targetPrice), constants.BID_ASK_SPREAD_PRECISION)

		var newTargetPrice *big.Int
		var newOptimalPeg *big.Int
		var newBudget *big.Int
		targetPriceGap := utils.SubX(reservePriceBefore, targetPrice)

		if utils.AbsX(targetPriceGap).Cmp(halfMaxPriceSpread) > 0 {
			markAdj := utils.SubX(utils.AbsX(targetPriceGap), halfMaxPriceSpread)

			if targetPriceGap.Cmp(constants.ZERO) < 0 {
				newTargetPrice = utils.AddX(reservePriceBefore, markAdj)
			} else {
				newTargetPrice = utils.SubX(reservePriceBefore, markAdj)
			}

			newOptimalPeg = CalculatePegFromTargetPrice(
				newTargetPrice,
				amm.BaseAssetReserve.BigInt(),
				amm.QuoteAssetReserve.BigInt(),
			)

			newBudget = CalculateRepegCost(amm, newOptimalPeg)
			checkLowerBound = false

			return newTargetPrice, newOptimalPeg, newBudget, false
		} else if amm.TotalFeeMinusDistributions.BigInt().Cmp(utils.DivX(amm.TotalExchangeFee.BigInt(), utils.BN(2))) < 0 {
			checkLowerBound = false
		}
	}

	return targetPrice, newPeg, budget, checkLowerBound
}

// line 112 OK
func CalculateNewAmm(amm *drift.Amm, oraclePriceData *oracles.OraclePriceData) (*big.Int, *big.Int, *big.Int, *big.Int) {
	pKNumer := utils.BN(1)
	pKDenom := utils.BN(1)

	targetPrice, _newPeg, budget, _ := CalculateOptimalPegAndBudget(amm, oraclePriceData)
	prePegCost := CalculateRepegCost(amm, _newPeg)
	newPeg := _newPeg

	if prePegCost.Cmp(budget) >= 0 && prePegCost.Cmp(constants.ZERO) > 0 {
		pKNumer = utils.BN(999)
		pKDenom = utils.BN(1000)
		deficitMadeup := CalculateAdjustKCost(amm, pKNumer, pKDenom)
		if deficitMadeup.Cmp(constants.ZERO) > 0 {
			panic("Invalid deficit Madeup")
		}
		prePegCost = utils.AddX(budget, utils.AbsX(deficitMadeup))
		newAmm := amm
		newAmm.BaseAssetReserve = utils.Uint128(utils.DivX(utils.MulX(newAmm.BaseAssetReserve.BigInt(), pKNumer), pKDenom))
		newAmm.SqrtK = utils.Uint128(utils.DivX(utils.MulX(newAmm.SqrtK.BigInt(), pKNumer), pKDenom))
		invariant := utils.MulX(newAmm.SqrtK.BigInt(), newAmm.SqrtK.BigInt())
		newAmm.QuoteAssetReserve = utils.Uint128(utils.DivX(invariant, newAmm.BaseAssetReserve.BigInt()))

		var directionToClose drift.PositionDirection
		if amm.BaseAssetAmountWithAmm.BigInt().Cmp(constants.ZERO) > 0 {
			directionToClose = drift.PositionDirection_Short
		} else {
			directionToClose = drift.PositionDirection_Long
		}

		newQuoteAssetReserve, _ := CalculateAmmReservesAfterSwap(
			newAmm,
			drift.AssetType_Base,
			utils.AbsX(amm.BaseAssetAmountWithAmm.BigInt()),
			GetSwapDirection(drift.AssetType_Base, directionToClose),
		)

		newAmm.TerminalQuoteAssetReserve = utils.Uint128(newQuoteAssetReserve)
		newPeg = CalculateBudgetedPeg(newAmm, prePegCost, targetPrice)
		prePegCost = CalculateRepegCost(newAmm, newPeg)
	}

	return prePegCost, pKNumer, pKDenom, newPeg
}

// line 154 OK
func CalculateUpdatedAMM(amm *drift.Amm, oraclePriceData *oracles.OraclePriceData) *drift.Amm {
	newAmm := *amm
	if amm.CurveUpdateIntensity == 0 || oraclePriceData == nil {
		return &newAmm
	}
	prepegCost, pKNumer, pKDenom, newPeg := CalculateNewAmm(
		amm,
		oraclePriceData,
	)

	//if pKDenom.Cmp(constants.ZERO) == 0 {
	//	spew.Dump(amm)
	//}
	newAmm.BaseAssetReserve = utils.Uint128(
		utils.DivX(
			utils.MulX(
				newAmm.BaseAssetReserve.BigInt(),
				pKNumer,
			),
			pKDenom,
		),
	)
	newAmm.SqrtK = utils.Uint128(
		utils.DivX(
			utils.MulX(
				newAmm.SqrtK.BigInt(),
				pKNumer,
			),
			pKDenom,
		),
	)
	invariant := utils.MulX(newAmm.SqrtK.BigInt(), newAmm.SqrtK.BigInt())
	newAmm.QuoteAssetReserve = utils.Uint128(utils.DivX(invariant, newAmm.BaseAssetReserve.BigInt()))
	newAmm.PegMultiplier = utils.Uint128(newPeg)

	var directionToClose drift.PositionDirection
	if amm.BaseAssetAmountWithAmm.BigInt().Cmp(constants.ZERO) > 0 {
		directionToClose = drift.PositionDirection_Short
	} else {
		directionToClose = drift.PositionDirection_Long
	}

	newQuoteAssetReserve, _ := CalculateAmmReservesAfterSwap(
		&newAmm,
		drift.AssetType_Base,
		utils.AbsX(amm.BaseAssetAmountWithAmm.BigInt()),
		GetSwapDirection(drift.AssetType_Base, directionToClose),
	)

	newAmm.TerminalQuoteAssetReserve = utils.Uint128(newQuoteAssetReserve)

	newAmm.TotalFeeMinusDistributions = utils.Int128(utils.SubX(newAmm.TotalFeeMinusDistributions.BigInt(), prepegCost))
	newAmm.NetRevenueSinceLastFunding = newAmm.NetRevenueSinceLastFunding - prepegCost.Int64()
	return &newAmm
}

// line 194 OK
func CalculateUpdatedAMMSpreadReserves(
	amm *drift.Amm,
	direction drift.PositionDirection,
	oraclePriceData *oracles.OraclePriceData,
) (
	*big.Int,
	*big.Int,
	*big.Int,
	*big.Int,
) {
	newAmm := CalculateUpdatedAMM(amm, oraclePriceData)
	shortReserves, longReserves := CalculateSpreadReserves(newAmm, oraclePriceData, 0)
	dirReserves := shortReserves
	if direction == drift.PositionDirection_Long {
		dirReserves = longReserves
	} else {
		dirReserves = shortReserves
	}
	return dirReserves.Base, dirReserves.Quote, newAmm.SqrtK.BigInt(), newAmm.PegMultiplier.BigInt()
}

// line 209 OK

func CalculateBidAskPrice(amm *drift.Amm, oraclePriceData *oracles.OraclePriceData, withUpdate bool) (*big.Int, *big.Int) {
	var newAmm *drift.Amm
	if withUpdate {
		newAmm = CalculateUpdatedAMM(amm, oraclePriceData)
	} else {
		newAmm = amm
	}

	bidReserves, askReserves := CalculateSpreadReserves(newAmm, oraclePriceData, 0)

	askPrice := CalculatePrice(
		askReserves.Base,
		askReserves.Quote,
		newAmm.PegMultiplier.BigInt(),
	)
	askPrice = utils.Max(utils.BN(1), askPrice)
	bidPrice := CalculatePrice(
		bidReserves.Base,
		bidReserves.Quote,
		newAmm.PegMultiplier.BigInt(),
	)
	bidPrice = utils.Max(utils.BN(1), bidPrice)
	return bidPrice, askPrice
}

// line 259 OK
func CalculatePrice(baseAssetReserves *big.Int, quoteAssetReserves *big.Int, pegMultiplier *big.Int) *big.Int {
	if utils.AbsX(baseAssetReserves).Cmp(constants.ZERO) <= 0 {
		return utils.BN(0)
	}

	u := utils.MulX(quoteAssetReserves, constants.PRICE_PRECISION, pegMultiplier)
	p := utils.DivX(u, constants.PEG_PRECISION, baseAssetReserves)
	return p
}

// line 286 OK
func CalculateAmmReservesAfterSwap(
	amm *drift.Amm,
	inputAssetType drift.AssetType,
	swapAmount *big.Int,
	swapDirection drift.SwapDirection,
) (*big.Int, *big.Int) {
	if swapAmount.Cmp(constants.ZERO) < 0 {
		panic("swapAmount must be greater than 0")
	}

	var newQuoteAssetReserve *big.Int
	var newBaseAssetReserve *big.Int

	if inputAssetType == drift.AssetType_Quote {
		swapAmount = utils.DivX(utils.MulX(swapAmount, constants.AMM_TIMES_PEG_TO_QUOTE_PRECISION_RATIO), amm.PegMultiplier.BigInt())

		newQuoteAssetReserve, newBaseAssetReserve = CalculateSwapOutput(
			amm.QuoteAssetReserve.BigInt(),
			swapAmount,
			swapDirection,
			utils.MulX(amm.SqrtK.BigInt(), amm.SqrtK.BigInt()),
		)
	} else {
		newBaseAssetReserve, newQuoteAssetReserve = CalculateSwapOutput(
			amm.BaseAssetReserve.BigInt(),
			swapAmount,
			swapDirection,
			utils.MulX(amm.SqrtK.BigInt(), amm.SqrtK.BigInt()),
		)
	}

	return newQuoteAssetReserve, newBaseAssetReserve
}

// line 323 OK
func CalculateMarketOpenBidAsk(
	baseAssetReserve *big.Int,
	minBaseAssetReserve *big.Int,
	maxBaseAssetReserve *big.Int,
	stepSize *big.Int,
) (*big.Int, *big.Int) {
	var openAsks *big.Int
	if minBaseAssetReserve.Cmp(baseAssetReserve) < 0 {
		openAsks = utils.MulX(utils.SubX(baseAssetReserve, minBaseAssetReserve), utils.BN(-1))

		if stepSize != nil && utils.DivX(utils.AbsX(openAsks), utils.BN(2)).Cmp(stepSize) < 0 {
			openAsks = utils.BN(0)
		}
	} else {
		openAsks = utils.BN(0)
	}

	var openBids *big.Int
	if maxBaseAssetReserve.Cmp(baseAssetReserve) > 0 {
		openBids = utils.SubX(maxBaseAssetReserve, baseAssetReserve)

		if stepSize != nil && utils.DivX(openBids, utils.BN(2)).Cmp(stepSize) < 0 {
			openBids = utils.BN(0)
		}
	} else {
		openBids = utils.BN(0)
	}

	return openBids, openAsks
}

func CalculateInventoryLiquidityRatio(
	baseAssetAmountWithAmm *big.Int,
	baseAssetReserve *big.Int,
	minBaseAssetReserve *big.Int,
	maxBaseAssetReserve *big.Int,
) *big.Int {
	openBids, openAsks := CalculateMarketOpenBidAsk(
		baseAssetReserve,
		minBaseAssetReserve,
		maxBaseAssetReserve,
		nil,
	)

	minSideLiquidity := utils.Min(utils.AbsX(openBids), utils.AbsX(openAsks))

	inventoryScaleBN := utils.Min(
		utils.AbsX(
			utils.DivX(
				utils.MulX(
					baseAssetAmountWithAmm,
					constants.PERCENTAGE_PRECISION,
				),
				utils.Max(minSideLiquidity, utils.BN(1)),
			),
		),
		constants.PERCENTAGE_PRECISION,
	)
	return inventoryScaleBN
}

// line 355 OK
func CalculateInventoryScale(
	baseAssetAmountWithAmm *big.Int,
	baseAssetReserve *big.Int,
	minBaseAssetReserve *big.Int,
	maxBaseAssetReserve *big.Int,
	directionalSpread int64,
	maxSpread int64,
) int64 {
	if baseAssetAmountWithAmm.Cmp(constants.ZERO) == 0 {
		return 1
	}
	MAX_BID_ASK_INVENTORY_SKEW_FACTOR := utils.MulX(constants.BID_ASK_SPREAD_PRECISION, utils.BN(10))

	inventoryScaleBN := CalculateInventoryLiquidityRatio(
		baseAssetAmountWithAmm,
		baseAssetReserve,
		minBaseAssetReserve,
		maxBaseAssetReserve,
	)
	if directionalSpread < 1 {
		directionalSpread = 1
	}

	inventoryScaleMaxBN := utils.Max(
		MAX_BID_ASK_INVENTORY_SKEW_FACTOR,
		utils.DivX(utils.MulX(utils.BN(maxSpread), constants.BID_ASK_SPREAD_PRECISION), utils.BN(directionalSpread)),
	)

	inventoryScaleCapped :=
		utils.Min(
			inventoryScaleMaxBN,
			utils.AddX(
				constants.BID_ASK_SPREAD_PRECISION,
				utils.DivX(
					utils.MulX(
						inventoryScaleMaxBN,
						inventoryScaleBN,
					),
					constants.PERCENTAGE_PRECISION,
				),
			),
		).Int64() / constants.BID_ASK_SPREAD_PRECISION.Int64()

	return inventoryScaleCapped
}

func CalculateReferencePriceOffset(
	reservePrice *big.Int,
	last24hAvgFundingRate *big.Int,
	liquidityFraction *big.Int,
	oracleTwapFast *big.Int,
	markTwapFast *big.Int,
	oracleTwapSlow *big.Int,
	markTwapSlow *big.Int,
	maxOffsetPct int64,
) *big.Int {
	if last24hAvgFundingRate.Cmp(constants.ZERO) == 0 {
		return utils.BN(0)
	}

	maxOffsetInPrice := utils.DivX(utils.MulX(utils.BN(maxOffsetPct), reservePrice), constants.PERCENTAGE_PRECISION)

	// Calculate quote denominated market premium
	markPremiumMinute := utils.ClampBN(
		utils.SubX(markTwapFast, oracleTwapFast),
		utils.MulX(maxOffsetInPrice, utils.BN(-1)),
		maxOffsetInPrice,
	)

	markPremiumHour := utils.ClampBN(
		utils.SubX(markTwapSlow, oracleTwapSlow),
		utils.MulX(maxOffsetInPrice, utils.BN(-1)),
		maxOffsetInPrice,
	)

	// Convert last24hAvgFundingRate to quote denominated premium
	markPremiumDay := utils.ClampBN(
		utils.MulX(utils.DivX(last24hAvgFundingRate, constants.FUNDING_RATE_BUFFER_PRECISION), utils.BN(24)),
		utils.MulX(maxOffsetInPrice, utils.BN(-1)),
		maxOffsetInPrice,
	)

	// Take average clamped premium as the price-based offset
	markPremiumAvg := utils.AddX(markPremiumMinute, markPremiumHour, markPremiumDay)
	markPremiumAvg = utils.DivX(markPremiumAvg, utils.BN(3))

	markPremiumAvgPct := utils.DivX(utils.MulX(markPremiumAvg, constants.PRICE_PRECISION), reservePrice)

	inventoryPct := utils.ClampBN(
		utils.DivX(utils.MulX(liquidityFraction, utils.BN(maxOffsetPct)), constants.PERCENTAGE_PRECISION),
		utils.MulX(maxOffsetInPrice, utils.BN(-1)),
		maxOffsetInPrice,
	)

	// Only apply when inventory is consistent with recent and 24h market premium
	offsetPct := utils.AddX(markPremiumAvgPct, inventoryPct)

	if inventoryPct.Sign() != markPremiumAvgPct.Sign() {
		offsetPct = utils.BN(0)
	}

	clampedOffsetPct := utils.ClampBN(
		offsetPct,
		utils.BN(-maxOffsetPct),
		utils.BN(maxOffsetPct),
	)
	return clampedOffsetPct
}

// 405 OK
func CalculateEffectiveLeverage(
	baseSpread int64,
	quoteAssetReserve *big.Int,
	terminalQuoteAssetReserve *big.Int,
	pegMultiplier *big.Int,
	netBaseAssetAmount *big.Int,
	reservePrice *big.Int,
	totalFeeMinusDistributions *big.Int) float64 {
	netBaseAssetValue := utils.DivX(
		utils.MulX(
			utils.SubX(quoteAssetReserve, terminalQuoteAssetReserve),
			pegMultiplier,
		),
		constants.AMM_TIMES_PEG_TO_QUOTE_PRECISION_RATIO,
	)

	localBaseAssetValue := utils.DivX(
		utils.MulX(netBaseAssetAmount, reservePrice),
		utils.MulX(constants.AMM_TO_QUOTE_PRECISION_RATIO, constants.PRICE_PRECISION),
	)

	l, _ := utils.SubX(localBaseAssetValue, netBaseAssetValue).Float64()
	effectiveGap := max(0.0, l)

	t, _ := totalFeeMinusDistributions.Float64()
	d, _ := constants.QUOTE_PRECISION.Float64()
	effectiveLeverage := effectiveGap/(max(float64(0.0), t)+1) + 1.0/d

	return effectiveLeverage
}

func CalculateMaxSpread(marginRatioInitial float64) float64 {
	t, _ := constants.BID_ASK_SPREAD_PRECISION.Float64()
	d, _ := constants.MARGIN_PRECISION.Float64()
	return marginRatioInitial * t / d
}

// line 444 OK
func CalculateVolSpreadBN(
	lastOracleConfPct *big.Int,
	reservePrice *big.Int,
	markStd *big.Int,
	oracleStd *big.Int,
	longIntensity *big.Int,
	shortIntensity *big.Int,
	volume24H *big.Int,
) (*big.Int, *big.Int) {
	marketAvgStdPct := utils.DivX(
		utils.MulX(
			utils.AddX(markStd, oracleStd),
			constants.PERCENTAGE_PRECISION,
		),
		reservePrice,
		utils.BN(2),
	)
	volSpread := utils.Max(lastOracleConfPct, utils.DivX(marketAvgStdPct, utils.BN(2)))

	clampMin := utils.DivX(constants.PERCENTAGE_PRECISION, utils.BN(100))
	clampMax := utils.DivX(utils.MulX(constants.PERCENTAGE_PRECISION, utils.BN(16)), utils.BN(10))

	//temp := utils.Max(
	//	utils.BN(1),
	//	volume24H,
	//)
	//if temp.Cmp(constants.ZERO) == 0 {
	//	spew.Dump(volume24H.Int64(), temp.Int64())
	//	panic("ZERO")
	//}
	longVolSpreadFactor := utils.ClampBN(
		utils.DivX(
			utils.MulX(
				longIntensity,
				constants.PERCENTAGE_PRECISION,
			),
			utils.Max(
				utils.BN(1),
				volume24H,
			),
		),
		clampMin,
		clampMax,
	)
	shortVolSpreadFactor := utils.ClampBN(
		utils.DivX(
			utils.MulX(
				shortIntensity,
				constants.PERCENTAGE_PRECISION,
			),
			utils.Max(
				utils.BN(1),
				volume24H,
			),
		),
		clampMin,
		clampMax,
	)

	// only consider confidence interval at full value when above 25 bps
	confComonent := lastOracleConfPct

	if lastOracleConfPct.Cmp(utils.DivX(constants.PRICE_PRECISION, utils.BN(400))) <= 0 {
		confComonent = utils.DivX(lastOracleConfPct, utils.BN(10))
	}
	longVolSpread := utils.Max(
		confComonent,
		utils.DivX(utils.MulX(volSpread, longVolSpreadFactor), constants.PERCENTAGE_PRECISION),
	)
	shortVolSpread := utils.Max(
		confComonent,
		utils.DivX(utils.MulX(volSpread, shortVolSpreadFactor), constants.PERCENTAGE_PRECISION),
	)

	return longVolSpread, shortVolSpread
}

func CalculateSpreadBN(
	baseSpread uint32,
	lastOracleReservePriceSpreadPct *big.Int,
	lastOracleConfPct *big.Int,
	maxSpread uint32,
	quoteAssetReserve *big.Int,
	terminalQuoteAssetReserve *big.Int,
	pegMultiplier *big.Int,
	baseAssetAmountWithAmm *big.Int,
	reservePrice *big.Int,
	totalFeeMinusDistributions *big.Int,
	netRevenueSinceLastFunding *big.Int,
	baseAssetReserve *big.Int,
	minBaseAssetReserve *big.Int,
	maxBaseAssetReserve *big.Int,
	markStd *big.Int,
	oracleStd *big.Int,
	longIntensity *big.Int,
	shortIntensity *big.Int,
	volume24H *big.Int,
) (float64, float64) {
	spreadTerms := CalculateSpreadBNTerms(
		baseSpread,
		lastOracleReservePriceSpreadPct,
		lastOracleConfPct,
		maxSpread,
		quoteAssetReserve,
		terminalQuoteAssetReserve,
		pegMultiplier,
		baseAssetAmountWithAmm,
		reservePrice,
		totalFeeMinusDistributions,
		netRevenueSinceLastFunding,
		baseAssetReserve,
		minBaseAssetReserve,
		maxBaseAssetReserve,
		markStd,
		oracleStd,
		longIntensity,
		shortIntensity,
		volume24H,
	)
	return spreadTerms.longSpread, spreadTerms.shortSpread
}

// line 486 OK
func CalculateSpreadBNTerms(
	baseSpread uint32,
	lastOracleReservePriceSpreadPct *big.Int,
	lastOracleConfPct *big.Int,
	maxSpread uint32,
	quoteAssetReserve *big.Int,
	terminalQuoteAssetReserve *big.Int,
	pegMultiplier *big.Int,
	baseAssetAmountWithAmm *big.Int,
	reservePrice *big.Int,
	totalFeeMinusDistributions *big.Int,
	netRevenueSinceLastFunding *big.Int,
	baseAssetReserve *big.Int,
	minBaseAssetReserve *big.Int,
	maxBaseAssetReserve *big.Int,
	markStd *big.Int,
	oracleStd *big.Int,
	longIntensity *big.Int,
	shortIntensity *big.Int,
	volume24H *big.Int,
) (spreadTerms SpreadTerms) {

	longVolSpread, shortVolSpread := CalculateVolSpreadBN(
		lastOracleConfPct,
		reservePrice,
		markStd,
		oracleStd,
		longIntensity,
		shortIntensity,
		volume24H,
	)
	spreadTerms.longVolSpread, _ = longVolSpread.Float64()
	spreadTerms.shortVolSpread, _ = shortVolSpread.Float64()

	longSpread := max(float64(baseSpread/2), utils.Float64(longVolSpread))
	shortSpread := max(float64(baseSpread/2), utils.Float64(shortVolSpread))

	if lastOracleReservePriceSpreadPct.Cmp(constants.ZERO) > 0 {
		shortSpread = max(shortSpread, float64(utils.AbsX(lastOracleReservePriceSpreadPct).Int64()+shortVolSpread.Int64()))
	} else if lastOracleReservePriceSpreadPct.Cmp(constants.ZERO) < 0 {
		longSpread = max(longSpread, float64(utils.AbsX(lastOracleReservePriceSpreadPct).Int64()+longVolSpread.Int64()))
	}

	spreadTerms.longSpreadwPS = longSpread
	spreadTerms.shortSpreadwPS = shortSpread

	maxSpreadBaseline := min(
		max(
			utils.Float64(utils.AbsX(lastOracleReservePriceSpreadPct)),
			utils.Float64(utils.MulX(lastOracleConfPct, utils.BN(2))),
			utils.Float64(utils.DivX(utils.MulX(utils.Max(markStd, oracleStd), constants.PERCENTAGE_PRECISION), reservePrice)),
		),
		utils.Float64(constants.BID_ASK_SPREAD_PRECISION),
	)

	maxTargetSpread := math.Floor(
		min(
			float64(maxSpread),
			maxSpreadBaseline,
		),
	)

	inventorySpreadScale := CalculateInventoryScale(
		baseAssetAmountWithAmm,
		baseAssetReserve,
		minBaseAssetReserve,
		maxBaseAssetReserve,
		utils.TT(baseAssetAmountWithAmm.Cmp(constants.ZERO) > 0, int64(longSpread), int64(shortSpread)),
		int64(maxTargetSpread),
	)

	if baseAssetAmountWithAmm.Cmp(constants.ZERO) > 0 {
		longSpread *= float64(inventorySpreadScale)
	} else if baseAssetAmountWithAmm.Cmp(constants.ZERO) < 0 {
		shortSpread *= float64(inventorySpreadScale)
	}
	spreadTerms.maxTargetSpread = maxTargetSpread
	spreadTerms.inventorySpreadScale = float64(inventorySpreadScale)
	spreadTerms.longSpreadwInvScale = longSpread
	spreadTerms.shortSpreadwInvScale = shortSpread

	MAX_SPREAD_SCALE := float64(10)
	if totalFeeMinusDistributions.Cmp(constants.ZERO) > 0 {
		effectiveLeverage := CalculateEffectiveLeverage(
			int64(baseSpread),
			quoteAssetReserve,
			terminalQuoteAssetReserve,
			pegMultiplier,
			baseAssetAmountWithAmm,
			reservePrice,
			totalFeeMinusDistributions,
		)
		spreadTerms.effectiveLeverage = effectiveLeverage

		spreadScale := min(MAX_SPREAD_SCALE, 1+effectiveLeverage)
		spreadTerms.effectiveLeverageCapped = spreadScale

		if baseAssetAmountWithAmm.Cmp(constants.ZERO) > 0 {
			longSpread *= spreadScale
			longSpread = math.Floor(longSpread)
		} else {
			shortSpread *= spreadScale
			shortSpread = math.Floor(shortSpread)
		}
	} else {
		longSpread *= MAX_SPREAD_SCALE
		shortSpread *= MAX_SPREAD_SCALE
	}

	if netRevenueSinceLastFunding.Cmp(
		constants.DEFAULT_REVENUE_SINCE_LAST_FUNDING_SPREAD_RETREAT,
	) < 0 {
		maxRetreat := maxTargetSpread / 10
		revenueRetreatAmount := maxRetreat
		if netRevenueSinceLastFunding.Cmp(
			utils.MulX(
				constants.DEFAULT_REVENUE_SINCE_LAST_FUNDING_SPREAD_RETREAT,
				utils.BN(1000),
			),
		) >= 0 {
			revenueRetreatAmount = min(
				maxRetreat,
				math.Floor(
					float64(baseSpread)*utils.Float64(utils.AbsX(netRevenueSinceLastFunding))/
						utils.Float64(utils.AbsX(constants.DEFAULT_REVENUE_SINCE_LAST_FUNDING_SPREAD_RETREAT)),
				),
			)

		}

		halfRevenueRetreatAmount := math.Floor(revenueRetreatAmount / 2)

		spreadTerms.revenueRetreatAmount = revenueRetreatAmount
		spreadTerms.halfRevenueRetreatAmount = halfRevenueRetreatAmount

		if baseAssetAmountWithAmm.Cmp(constants.ZERO) > 0 {
			longSpread += revenueRetreatAmount
			shortSpread += halfRevenueRetreatAmount
		} else if baseAssetAmountWithAmm.Cmp(constants.ZERO) < 0 {
			longSpread += halfRevenueRetreatAmount
			shortSpread += revenueRetreatAmount
		} else {
			longSpread += halfRevenueRetreatAmount
			shortSpread += halfRevenueRetreatAmount
		}
	}

	spreadTerms.longSpreadwRevRetreat = longSpread
	spreadTerms.shortSpreadwRevRetreat = shortSpread

	totalSpread := longSpread + shortSpread
	if totalSpread > maxTargetSpread {
		if longSpread > shortSpread {
			longSpread = math.Ceil(longSpread * maxTargetSpread / totalSpread)
			shortSpread = math.Floor(maxTargetSpread - longSpread)
		} else {
			shortSpread = math.Ceil(shortSpread * maxTargetSpread / totalSpread)
			longSpread = math.Floor(maxTargetSpread - shortSpread)
		}
	}

	spreadTerms.totalSpread = totalSpread
	spreadTerms.longSpread = longSpread
	spreadTerms.shortSpread = shortSpread
	return spreadTerms
}

// line 681 OK
func CalculateSpread(
	amm *drift.Amm,
	oraclePriceData *oracles.OraclePriceData,
	now int64,
	reservePrice *big.Int,
) (float64, float64) {
	if amm.BaseSpread == 0 || amm.CurveUpdateIntensity == 0 {
		return float64(amm.BaseSpread) / 2, float64(amm.BaseSpread) / 2
	}
	if reservePrice == nil {
		reservePrice = CalculatePrice(
			amm.BaseAssetReserve.BigInt(),
			amm.QuoteAssetReserve.BigInt(),
			amm.PegMultiplier.BigInt(),
		)
	}
	reservePrice = utils.Max(utils.BN(1), reservePrice)
	targetPrice := utils.TTM[*big.Int](
		oraclePriceData != nil && oraclePriceData.Price.Cmp(constants.ZERO) != 0,
		func() *big.Int { return oraclePriceData.Price },
		reservePrice,
	)
	targetMarkSpreadPct := utils.DivX(
		utils.MulX(
			utils.SubX(
				reservePrice,
				targetPrice,
			),
			constants.BID_ASK_SPREAD_PRECISION,
		),
		reservePrice,
	)

	now = utils.TT(now > 0, now, time.Now().Unix())
	liveOracleStd := CalculateLiveOracleStd(amm, oraclePriceData, now)
	confIntervalPct := GetNewOracleConfPct(
		amm,
		oraclePriceData,
		reservePrice,
		now,
	)

	return CalculateSpreadBN(
		amm.BaseSpread,
		targetMarkSpreadPct,
		confIntervalPct,
		amm.MaxSpread,
		amm.QuoteAssetReserve.BigInt(),
		amm.TerminalQuoteAssetReserve.BigInt(),
		amm.PegMultiplier.BigInt(),
		amm.BaseAssetAmountWithAmm.BigInt(),
		reservePrice,
		amm.TotalFeeMinusDistributions.BigInt(),
		utils.BN(amm.NetRevenueSinceLastFunding),
		amm.BaseAssetReserve.BigInt(),
		amm.MinBaseAssetReserve.BigInt(),
		amm.MaxBaseAssetReserve.BigInt(),
		utils.BN(amm.MarkStd),
		liveOracleStd,
		utils.BN(amm.LongIntensityVolume),
		utils.BN(amm.ShortIntensityVolume),
		utils.BN(amm.Volume24H),
	)
}

// line 737 OK
func CalculateSpreadReserves(
	amm *drift.Amm,
	oraclePriceData *oracles.OraclePriceData,
	now int64,
) (*AssetReserve, *AssetReserve) {
	calculateSpreadReserve := func(
		spread float64,
		direction drift.PositionDirection,
		amm *drift.Amm,
	) *AssetReserve {
		if spread == 0.0 {
			return &AssetReserve{
				Base:  amm.BaseAssetReserve.BigInt(),
				Quote: amm.QuoteAssetReserve.BigInt(),
			}
		}
		spreadFraction := utils.BN(int64(spread / 2))

		// make non-zero
		if spreadFraction.Cmp(constants.ZERO) == 0 {
			spreadFraction = utils.TT(spread >= 0, utils.BN(1), utils.BN(-1))
		}
		quoteAssetReserveDelta := utils.DivX(
			amm.QuoteAssetReserve.BigInt(),
			utils.DivX(constants.BID_ASK_SPREAD_PRECISION, spreadFraction),
		)

		var quoteAssetReserve *big.Int
		if quoteAssetReserveDelta.Cmp(constants.ZERO) >= 0 {
			quoteAssetReserve = utils.AddX(amm.QuoteAssetReserve.BigInt(), utils.AbsX(quoteAssetReserveDelta))
		} else {
			quoteAssetReserve = utils.SubX(amm.QuoteAssetReserve.BigInt(), utils.AbsX(quoteAssetReserveDelta))
		}
		baseAssetReserve := utils.DivX(utils.MulX(amm.SqrtK.BigInt(), amm.SqrtK.BigInt()), quoteAssetReserve)
		return &AssetReserve{
			Base:  baseAssetReserve,
			Quote: quoteAssetReserve,
		}
	}

	reservePrice := CalculatePrice(
		amm.BaseAssetReserve.BigInt(),
		amm.QuoteAssetReserve.BigInt(),
		amm.PegMultiplier.BigInt(),
	)
	reservePrice = utils.Max(utils.BN(1), reservePrice)
	// always allow 10 bps of price offset, up to a fifth of the market's max_spread
	var maxOffset int64 = 0
	referencePriceOffset := utils.BN(0)
	if amm.CurveUpdateIntensity > 100 {
		maxOffset = max(
			int64(amm.MaxSpread)/5,
			(constants.PERCENTAGE_PRECISION.Int64()/10000)*int64(amm.CurveUpdateIntensity-100),
		)

		liquidityFraction := CalculateInventoryLiquidityRatio(
			amm.BaseAssetAmountWithAmm.BigInt(),
			amm.BaseAssetReserve.BigInt(),
			amm.MinBaseAssetReserve.BigInt(),
			amm.MaxBaseAssetReserve.BigInt(),
		)

		liquidityFractionSigned := utils.MulX(
			liquidityFraction,
			utils.SigNum(
				utils.AddX(
					amm.BaseAssetAmountWithAmm.BigInt(),
					amm.BaseAssetAmountWithUnsettledLp.BigInt(),
				),
			),
		)
		referencePriceOffset = CalculateReferencePriceOffset(
			reservePrice,
			utils.BN(amm.Last24HAvgFundingRate),
			liquidityFractionSigned,
			utils.BN(amm.HistoricalOracleData.LastOraclePriceTwap5Min),
			utils.BN(amm.LastMarkPriceTwap5Min),
			utils.BN(amm.HistoricalOracleData.LastOraclePriceTwap),
			utils.BN(amm.LastMarkPriceTwap),
			maxOffset,
		)
	}

	longSpread, shortSpread := CalculateSpread(
		amm,
		oraclePriceData,
		now,
		reservePrice,
	)
	//spew.Dump("longSpread", longSpread, "shortSpread", shortSpread)
	askReserves := calculateSpreadReserve(
		longSpread+float64(referencePriceOffset.Int64()),
		drift.PositionDirection_Long,
		amm,
	)
	bidReserves := calculateSpreadReserve(
		-shortSpread+float64(referencePriceOffset.Int64()),
		drift.PositionDirection_Short,
		amm,
	)

	return bidReserves, askReserves
}

// CalculateSwapOutput
/**
 * Helper function calculating constant product curve output. Agnostic to whether input asset is quote or base
 *
 * @param inputAssetReserve
 * @param swapAmount
 * @param swapDirection
 * @param invariant
 * @returns newInputAssetReserve and newOutputAssetReserve after swap. : Precision AMM_RESERVE_PRECISION
 */
func CalculateSwapOutput(
	inputAssetReserve *big.Int,
	swapAmount *big.Int,
	swapDirection drift.SwapDirection,
	invariant *big.Int,
) (*big.Int, *big.Int) {
	var newInputAssetReserve *big.Int
	if swapDirection == drift.SwapDirection_Add {
		newInputAssetReserve = utils.AddX(inputAssetReserve, swapAmount)
	} else {
		newInputAssetReserve = utils.SubX(inputAssetReserve, swapAmount)
	}
	newOutputAssetReserve := utils.DivX(invariant, newInputAssetReserve)
	return newInputAssetReserve, newOutputAssetReserve
}

// line 821 OK
func GetSwapDirection(
	inputAssetType drift.AssetType,
	positionDirection drift.PositionDirection,
) drift.SwapDirection {
	if positionDirection == drift.PositionDirection_Long && inputAssetType == drift.AssetType_Base {
		return drift.SwapDirection_Remove
	}

	if positionDirection == drift.PositionDirection_Short && inputAssetType == drift.AssetType_Quote {
		return drift.SwapDirection_Remove
	}

	return drift.SwapDirection_Add
}

// CalculateTerminalPrice
/**
 * Helper function calculating terminal price of amm
 *
 * @param market
 * @returns cost : Precision PRICE_PRECISION
 */
func CalculateTerminalPrice(market *drift.PerpMarket) *big.Int {
	directionToClose := utils.TT(
		market.Amm.BaseAssetAmountWithAmm.BigInt().Cmp(constants.ZERO) > 0,
		drift.PositionDirection_Short,
		drift.PositionDirection_Long,
	)
	newQuoteAssetReserve, newBaseAssetReserve := CalculateAmmReservesAfterSwap(
		&market.Amm,
		drift.AssetType_Base,
		utils.AbsX(market.Amm.BaseAssetAmountWithAmm.BigInt()),
		GetSwapDirection(drift.AssetType_Base, directionToClose),
	)
	terminalPrice := utils.MulX(newQuoteAssetReserve, constants.PRICE_PRECISION, market.Amm.PegMultiplier.BigInt())
	terminalPrice = utils.DivX(constants.PEG_PRECISION, newBaseAssetReserve)

	return terminalPrice
}

func CalculateMaxBaseAssetAmountToTrade(
	amm *drift.Amm,
	limitPrice *big.Int,
	direction drift.PositionDirection,
	oraclePriceData *oracles.OraclePriceData,
	nowTs int64,
) (*big.Int, drift.PositionDirection) {
	invariant := utils.MulX(amm.SqrtK.BigInt(), amm.SqrtK.BigInt())
	if limitPrice.Cmp(constants.ZERO) == 0 {
		fmt.Println("direction=", direction, ",orderTickSize=", amm.OrderTickSize, ",amm.SqrtK=", amm.SqrtK.BigInt().String(), ", PegMultiplier=", amm.PegMultiplier.BigInt().String(), ", limitPrice=", limitPrice.String())
		panic("CalculateMaxBaseAssetAmountToTrade error : limitPrice is zero")
	}

	newBaseAssetReserveSquared := utils.MulX(invariant, constants.PRICE_PRECISION, amm.PegMultiplier.BigInt())
	newBaseAssetReserveSquared = utils.DivX(newBaseAssetReserveSquared, limitPrice, constants.PEG_PRECISION)

	newBaseAssetReserve := utils.SquareRootBN(newBaseAssetReserveSquared)
	shortSpreadReserves, longSpreadReserves := CalculateSpreadReserves(
		amm,
		oraclePriceData,
		nowTs,
	)

	baseAssetReserveBefore := utils.TT(
		direction == drift.PositionDirection_Long,
		longSpreadReserves.Base,
		shortSpreadReserves.Base,
	)

	if newBaseAssetReserve == nil {
		fmt.Println("newBaseAssetReserveSquared = ", newBaseAssetReserveSquared.String())
		fmt.Println("direction=", direction, ",orderTickSize=", amm.OrderTickSize, ",amm.SqrtK=", amm.SqrtK.BigInt().String(), ", PegMultiplier=", amm.PegMultiplier.BigInt().String(), ", limitPrice=", limitPrice.String())
		panic("CalculateMaxBaseAssetAmountToTrade error : baseAssetReserveBefore is nil")
	}

	if newBaseAssetReserve.Cmp(baseAssetReserveBefore) > 0 {
		return utils.SubX(newBaseAssetReserve, baseAssetReserveBefore), drift.PositionDirection_Short
	} else if newBaseAssetReserve.Cmp(baseAssetReserveBefore) < 0 {
		return utils.SubX(baseAssetReserveBefore, newBaseAssetReserve), drift.PositionDirection_Long
	} else {
		fmt.Println("tradeSize Too Small")
		return utils.BN(0), drift.PositionDirection_Long
	}
}

func CalculateQuoteAssetAmountSwapped(
	quoteAssetReserves *big.Int,
	pegMultiplier *big.Int,
	swapDirection drift.SwapDirection,
) *big.Int {
	if swapDirection == drift.SwapDirection_Remove {
		quoteAssetReserves = utils.AddX(quoteAssetReserves, utils.BN(1))
	}

	quoteAssetAmount := utils.DivX(utils.MulX(quoteAssetReserves, pegMultiplier), constants.AMM_TIMES_PEG_TO_QUOTE_PRECISION_RATIO)

	if swapDirection == drift.SwapDirection_Remove {
		quoteAssetAmount = utils.AddX(quoteAssetAmount, utils.BN(1))
	}

	return quoteAssetAmount
}

func CalculateMaxBaseAssetAmountFillable(
	amm *drift.Amm,
	orderDirection drift.PositionDirection,
) *big.Int {
	maxFillSize := utils.DivX(
		amm.BaseAssetReserve.BigInt(),
		utils.BN(amm.MaxFillReserveFraction),
	)
	var maxBaseAssetAmountOnSide *big.Int
	if orderDirection == drift.PositionDirection_Long {
		maxBaseAssetAmountOnSide = utils.Max(
			utils.BN(0),
			utils.SubX(amm.BaseAssetReserve.BigInt(), amm.MinBaseAssetReserve.BigInt()),
		)
	} else {
		maxBaseAssetAmountOnSide = utils.Max(
			utils.BN(0),
			utils.SubX(amm.MaxBaseAssetReserve.BigInt(), amm.BaseAssetReserve.BigInt()),
		)
	}
	standardizedMaxBaseAssetAmountOnSide := utils.Min(maxFillSize, maxBaseAssetAmountOnSide)
	return StandardizeBaseAssetAmount(standardizedMaxBaseAssetAmountOnSide, utils.BN(amm.OrderStepSize))
}
