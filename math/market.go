package math

import (
	"driftgo/constants"
	"driftgo/dlob/types"
	"driftgo/lib/drift"
	oracles "driftgo/oracles/types"
	"driftgo/utils"
	"math/big"
)

func CalculateReservePrice(
	market *drift.PerpMarket,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	newAmm := CalculateUpdatedAMM(&market.Amm, oraclePriceData)
	return CalculatePrice(
		newAmm.BaseAssetReserve.BigInt(),
		newAmm.QuoteAssetReserve.BigInt(),
		newAmm.PegMultiplier.BigInt(),
	)
}

func CalculateBidPrice(
	market *drift.PerpMarket,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	baseAssetReserve, quoteAssetReserve, _, newPeg := CalculateUpdatedAMMSpreadReserves(
		&market.Amm,
		drift.PositionDirection_Short,
		oraclePriceData,
	)
	return CalculatePrice(baseAssetReserve, quoteAssetReserve, newPeg)
}

func CalculateAskPrice(
	market *drift.PerpMarket,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	baseAssetReserve, quoteAssetReserve, _, newPeg := CalculateUpdatedAMMSpreadReserves(
		&market.Amm,
		drift.PositionDirection_Long,
		oraclePriceData,
	)
	return CalculatePrice(baseAssetReserve, quoteAssetReserve, newPeg)
}

func CalculateNewMarketAfterTrade(
	baseAssetAmount *big.Int,
	direction drift.PositionDirection,
	market *drift.PerpMarket,
) *drift.PerpMarket {
	newQuoteAssetReserve, newBaseAssetReserve := CalculateAmmReservesAfterSwap(
		&market.Amm,
		drift.AssetType_Base,
		utils.AbsX(baseAssetAmount),
		GetSwapDirection(drift.AssetType_Base, direction),
	)
	newMarket := *market
	newMarket.Amm = market.Amm
	newMarket.Amm.QuoteAssetReserve = utils.Uint128(newQuoteAssetReserve)
	newMarket.Amm.BaseAssetReserve = utils.Uint128(newBaseAssetReserve)

	return &newMarket
}

func CalculateOracleReserveSpread(
	market *drift.PerpMarket,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	reservePrice := CalculateReservePrice(market, oraclePriceData)
	return CalculateOracleSpread(reservePrice, oraclePriceData)
}

func CalculateOracleSpread(
	price *big.Int,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	return utils.SubX(price, oraclePriceData.Price)
}

func CalculateMarketMarginRatio(
	market *drift.PerpMarket,
	size *big.Int,
	marginCategory drift.MarginRequirementType,
	customMarginRatio int64,
) int64 {
	var marginRatio int64
	switch marginCategory {
	case drift.MarginRequirementType_Initial:
		// use lowest leverage between max allowed and optional user custom max
		marginRatio = max(
			CalculateSizePremiumLiabilityWeight(
				size,
				utils.BN(market.ImfFactor),
				utils.BN(market.MarginRatioInitial),
				constants.MARGIN_PRECISION,
			).Int64(),
			customMarginRatio,
		)
	case drift.MarginRequirementType_Maintenance:
		marginRatio = CalculateSizePremiumLiabilityWeight(
			size,
			utils.BN(market.ImfFactor),
			utils.BN(market.MarginRatioInitial),
			constants.MARGIN_PRECISION,
		).Int64()
	}
	return marginRatio
}

func CalculateUnrealizedAssetWeight(
	market *drift.PerpMarket,
	quoteSpotMarket *drift.SpotMarket,
	unrealizedPnl *big.Int,
	marginCategory drift.MarginRequirementType,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	var assetWeight *big.Int
	switch marginCategory {
	case drift.MarginRequirementType_Initial:
		assetWeight = utils.BN(market.UnrealizedPnlInitialAssetWeight)

		if market.UnrealizedPnlMaxImbalance > 0 {
			var netUnsettledPnl *big.Int = CalculateNetUserPnlImbalance(
				market,
				quoteSpotMarket,
				oraclePriceData,
			)
			if netUnsettledPnl.Cmp(utils.BN(market.UnrealizedPnlMaxImbalance)) > 0 {
				assetWeight = utils.DivX(utils.MulX(assetWeight, utils.BN(market.UnrealizedPnlMaxImbalance)), netUnsettledPnl)
			}
		}

		assetWeight = CalculateSizeDiscountAssetWeight(
			unrealizedPnl,
			utils.BN(market.UnrealizedPnlImfFactor),
			assetWeight,
		)
	case drift.MarginRequirementType_Maintenance:
		assetWeight = utils.BN(market.UnrealizedPnlMaintenanceAssetWeight)
	}

	return assetWeight
}

func CalculateMarketAvailablePNL(
	perpMarket *drift.PerpMarket,
	spotMarket *drift.SpotMarket,
) *big.Int {
	return GetTokenAmount(
		perpMarket.PnlPool.ScaledBalance.BigInt(),
		spotMarket,
		drift.SpotBalanceType_Deposit,
	)
}

func CalculateMarketMaxAvailableInsurance(
	perpMarket *drift.PerpMarket,
	spotMarket *drift.SpotMarket,
) *big.Int {
	if spotMarket.MarketIndex != uint16(constants.QUOTE_SPOT_MARKET_INDEX) {
		panic("invalid spot market")
	}

	insuranceFundAllocation := perpMarket.InsuranceClaim.QuoteMaxInsurance - perpMarket.InsuranceClaim.QuoteSettledInsurance
	var ammFeePool *big.Int = GetTokenAmount(
		perpMarket.Amm.FeePool.ScaledBalance.BigInt(),
		spotMarket,
		drift.SpotBalanceType_Deposit,
	)
	return utils.AddX(utils.BN(insuranceFundAllocation), ammFeePool)
}

func CalculateNetUserPnl(
	perpMarket *drift.PerpMarket,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	netUserPositionValue := utils.DivX(
		utils.MulX(perpMarket.Amm.BaseAssetAmountWithAmm.BigInt(), oraclePriceData.Price),
		constants.BASE_PRECISION,
		constants.PRICE_TO_QUOTE_PRECISION,
	)

	netUserCostBasis := perpMarket.Amm.QuoteAssetAmount.BigInt()

	netUserPnl := utils.AddX(netUserPositionValue, netUserCostBasis)

	return netUserPnl
}

func CalculateNetUserPnlImbalance(
	perpMarket *drift.PerpMarket,
	spotMarket *drift.SpotMarket,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	netUserPnl := CalculateNetUserPnl(perpMarket, oraclePriceData)

	var pnlPool *big.Int = GetTokenAmount(
		perpMarket.PnlPool.ScaledBalance.BigInt(),
		spotMarket,
		drift.SpotBalanceType_Deposit,
	)
	var feePool *big.Int = utils.DivX(
		GetTokenAmount(
			perpMarket.Amm.FeePool.ScaledBalance.BigInt(),
			spotMarket,
			drift.SpotBalanceType_Deposit,
		),
		utils.BN(5),
	)

	imbalance := utils.SubX(netUserPnl, utils.AddX(pnlPool, feePool))

	return imbalance
}

func CalculateAvailablePerpLiquidity(
	market *drift.PerpMarket,
	oraclePriceData *oracles.OraclePriceData,
	dlob types.IDLOB,
	slot uint64,
) (*big.Int, *big.Int) {
	bids, asks := CalculateMarketOpenBidAsk(
		market.Amm.BaseAssetReserve.BigInt(),
		market.Amm.MinBaseAssetReserve.BigInt(),
		market.Amm.MaxBaseAssetReserve.BigInt(),
		utils.BN(market.Amm.OrderStepSize),
	)

	asks = utils.AbsX(asks)
	bidGenerator := dlob.GetRestingLimitBids(
		market.MarketIndex,
		slot,
		drift.MarketType_Perp,
		oraclePriceData,
		nil,
	)
	bidGenerator.Each(func(bid types.IDLOBNode, key int) bool {
		bids = utils.AddX(bids, utils.BN(bid.GetOrder().BaseAssetAmount-bid.GetOrder().BaseAssetAmountFilled))
		return false
	})
	askGenerator := dlob.GetRestingLimitAsks(
		market.MarketIndex,
		slot,
		drift.MarketType_Perp,
		oraclePriceData,
		nil,
	)
	askGenerator.Each(func(ask types.IDLOBNode, key int) bool {
		asks = utils.AddX(asks, utils.BN(ask.GetOrder().BaseAssetAmount-ask.GetOrder().BaseAssetAmountFilled))
		return false
	})

	return bids, asks
}
