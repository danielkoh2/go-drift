package math

import (
	"driftgo/constants"
	"driftgo/lib/drift"
	oracles "driftgo/oracles/types"
	"driftgo/utils"
	"math"
	"math/big"
)

func OraclePriceBands(
	market *drift.PerpMarket,
	oraclePriceData *oracles.OraclePriceData,
) (*big.Int, *big.Int) {
	maxPercentDiff := market.MarginRatioInitial - market.MarginRatioMaintenance
	offset := utils.DivX(utils.MulX(oraclePriceData.Price, big.NewInt(int64(maxPercentDiff))), constants.MARGIN_PRECISION)
	if offset.Cmp(constants.ZERO) <= 0 {
		panic("Invalid offset")
	}
	return utils.SubX(oraclePriceData.Price, offset), utils.AddX(oraclePriceData.Price, offset)
}

func GetMaxConfidenceIntervalMultiplier(market *drift.PerpMarket) *big.Int {
	var maxConfidenceIntervalMultiplier *big.Int
	if market.ContractTier == drift.ContractTier_A {
		maxConfidenceIntervalMultiplier = utils.BN(1)
	} else if market.ContractTier == drift.ContractTier_B {
		maxConfidenceIntervalMultiplier = utils.BN(1)
	} else if market.ContractTier == drift.ContractTier_C {
		maxConfidenceIntervalMultiplier = utils.BN(2)
	} else if market.ContractTier == drift.ContractTier_Speculative {
		maxConfidenceIntervalMultiplier = utils.BN(10)
	} else {
		maxConfidenceIntervalMultiplier = utils.BN(50)
	}
	return maxConfidenceIntervalMultiplier
}
func IsOracleValid(
	market *drift.PerpMarket,
	oraclePriceData *oracles.OraclePriceData,
	oracleGuardRails *drift.OracleGuardRails,
	slot uint64,
) bool {
	// checks if oracle is valid for an AMM only fill
	amm := &market.Amm
	isOraclePriceNonPositive := oraclePriceData.Price == nil || oraclePriceData.Price.Cmp(constants.ZERO) <= 0
	isOraclePriceTooVolatile := false
	oraclePriceF, _ := oraclePriceData.Price.Float64()
	oracleConfidenceF, _ := oraclePriceData.Confidence.Float64()
	if oraclePriceData.Price != nil {
		isOraclePriceTooVolatile =
			oraclePriceF / math.Max(1.0, float64(amm.HistoricalOracleData.LastOraclePriceTwap)) > float64(oracleGuardRails.Validity.TooVolatileRatio) ||
				float64(amm.HistoricalOracleData.LastOraclePriceTwap) / math.Max(1.0, oraclePriceF) > float64(oracleGuardRails.Validity.TooVolatileRatio)
		//
		//
		//isOraclePriceTooVolatile =
		//	(utils.DivX(
		//		oraclePriceData.Price,
		//		utils.Max(
		//			utils.BN(1),
		//			utils.BN(amm.HistoricalOracleData.LastOraclePriceTwap),
		//		),
		//	).Cmp(utils.BN(oracleGuardRails.Validity.TooVolatileRatio)) > 0) ||
		//		(utils.DivX(
		//			utils.BN(amm.HistoricalOracleData.LastOraclePriceTwap),
		//			utils.Max(
		//				utils.BN(1),
		//				oraclePriceData.Price,
		//			),
		//		).Cmp(utils.BN(oracleGuardRails.Validity.TooVolatileRatio)) > 0)
	}
	maxConfidenceIntervalMultiplier, _ := GetMaxConfidenceIntervalMultiplier(market).Float64()
	isConfidenceTooLarge := math.Max(1.0, oracleConfidenceF) * float64(constants.BID_ASK_SPREAD_PRECISION.Int64()) / oraclePriceF >
		float64(oracleGuardRails.Validity.ConfidenceIntervalMaxSize) * maxConfidenceIntervalMultiplier
	//isConfidenceTooLarge := utils.DivX(
	//	utils.MulX(
	//		utils.Max(
	//			utils.BN(1),
	//			oraclePriceData.Confidence,
	//		),
	//		constants.BID_ASK_SPREAD_PRECISION,
	//	),
	//	oraclePriceData.Price,
	//).Cmp(utils.MulX(
	//	utils.BN(oracleGuardRails.Validity.ConfidenceIntervalMaxSize),
	//	maxConfidenceIntervalMultiplier,
	//)) > 0
	oracleIsStale := slot-oraclePriceData.Slot > uint64(oracleGuardRails.Validity.SlotsBeforeStaleForAmm)

	return !(!oraclePriceData.HasSufficientNumberOfDataPoints ||
		oracleIsStale ||
		isOraclePriceNonPositive ||
		isOraclePriceTooVolatile ||
		isConfidenceTooLarge)
}

func IsOracleTooDivergent(
	amm *drift.Amm,
	oraclePriceData *oracles.OraclePriceData,
	oracleGuardRails *drift.OracleGuardRails,
	now int64,
) bool {
	sinceLastUpdate := now - amm.HistoricalOracleData.LastOraclePriceTwapTs
	sinceStart := max(0, 300-sinceLastUpdate)
	oracleTwap5min := big.NewInt(amm.HistoricalOracleData.LastOraclePriceTwap5Min * sinceStart)
	oracleTwap5min = utils.DivX(
		utils.MulX(
			utils.AddX(
				oracleTwap5min,
				oraclePriceData.Price,
			),
			big.NewInt(sinceLastUpdate),
		),
		big.NewInt(sinceStart+sinceLastUpdate),
	)

	oracleSpread := utils.SubX(oracleTwap5min, oraclePriceData.Price)
	oracleSpreadPct := utils.DivX(utils.MulX(oracleSpread, constants.PRICE_PRECISION), oracleTwap5min)

	maxDivergence := utils.Max(
		big.NewInt(0).SetUint64(oracleGuardRails.PriceDivergence.MarkOraclePercentDivergence),
		utils.DivX(constants.PERCENTAGE_PRECISION, utils.BN(10)),
	)
	tooDivergent := utils.AbsX(oracleSpreadPct).Cmp(maxDivergence) >= 0
	return tooDivergent
}

// line 92 OK
func CalculateLiveOracleTwap(
	histOracleData *drift.HistoricalOracleData,
	oraclePriceData *oracles.OraclePriceData,
	now int64,
	period int64,
) *big.Int {
	var oracleTwap *big.Int
	if period == 5*60 {
		oracleTwap = big.NewInt(histOracleData.LastOraclePriceTwap5Min)
	} else {
		// todo: assumes its fundingPeriod (1hr)
		// period = amm.fundingPeriod;
		oracleTwap = big.NewInt(histOracleData.LastOraclePriceTwap)
	}

	sinceLastUpdate := max(now-histOracleData.LastOraclePriceTwapTs, 1)
	sinceStart := max(0, period-sinceLastUpdate)

	clampRange := utils.DivX(oracleTwap, big.NewInt(3))

	clampedOraclePrice := utils.Min(
		utils.AddX(oracleTwap, clampRange),
		utils.Max(oraclePriceData.Price, utils.SubX(oracleTwap, clampRange)),
	)

	newOracleTwap := utils.DivX(
		utils.AddX(
			utils.MulX(oracleTwap, big.NewInt(sinceStart)),
			utils.MulX(clampedOraclePrice, big.NewInt(sinceLastUpdate)),
		),
		big.NewInt(sinceStart+sinceLastUpdate),
	)

	return newOracleTwap
}

// line 128 OK
func CalculateLiveOracleStd(
	amm *drift.Amm,
	oraclePriceData *oracles.OraclePriceData,
	now int64,
) *big.Int {
	sinceLastUpdate := max(1, now-amm.HistoricalOracleData.LastOraclePriceTwapTs)
	sinceStart := max(0, amm.FundingPeriod-sinceLastUpdate)

	liveOracleTwap := CalculateLiveOracleTwap(
		&amm.HistoricalOracleData,
		oraclePriceData,
		now,
		amm.FundingPeriod,
	)

	priceDeltaVsTwap := utils.AbsX(utils.SubX(oraclePriceData.Price, liveOracleTwap))

	oracleStd := utils.AddX(
		priceDeltaVsTwap,
		big.NewInt(int64(amm.OracleStd)*sinceStart/(sinceStart+sinceLastUpdate)),
	)

	return oracleStd
}

func GetNewOracleConfPct(
	amm *drift.Amm,
	oraclePriceData *oracles.OraclePriceData,
	reservePrice *big.Int,
	now int64,
) *big.Int {
	confInterval := utils.BN(0)
	if oraclePriceData.Confidence != nil {
		confInterval = oraclePriceData.Confidence
	}

	sinceLastUpdate := max(0, now-amm.HistoricalOracleData.LastOraclePriceTwapTs)
	lowerBoundConfPct := amm.LastOracleConfPct
	if sinceLastUpdate > 0 {
		lowerBoundConfDivisor := uint64(max(21-sinceLastUpdate, 5))
		lowerBoundConfPct = amm.LastOracleConfPct - amm.LastOracleConfPct/lowerBoundConfDivisor
	}
	confIntervalPct := utils.DivX(utils.MulX(confInterval, constants.BID_ASK_SPREAD_PRECISION), reservePrice)

	confIntervalPctResult := utils.Max(confIntervalPct, big.NewInt(0).SetUint64(lowerBoundConfPct))

	return confIntervalPctResult
}
