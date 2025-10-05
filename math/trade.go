package math

import (
	"driftgo/assert"
	"driftgo/common"
	"driftgo/constants"
	"driftgo/dlob/types"
	dloblib "driftgo/dlob/types"
	"driftgo/lib/drift"
	"driftgo/lib/serum"
	oracles "driftgo/oracles/types"
	"driftgo/utils"
	"fmt"
	"math"
	"math/big"
	"time"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

var MAXPCT = utils.BN(1000)

type PriceImpactUnit bin.BorshEnum

const (
	PriceImpactUnitEntryPrice PriceImpactUnit = iota
	PriceImpactUnitMaxPrice
	PriceImpactUnitPriceDelta
	PriceImpactUnitPriceDeltaAsNumber
	PriceImpactUnitPctAvg
	PriceImpactUnitPctMax
	PriceImpactUnitQuoteAssetAmount
	PriceImpactUnitQuoteAssetAmountPeg
	PriceImpactUnitAcquiredBaseAssetAmount
	PriceImpactUnitAcquiredQuoteAssetAmount
	PriceImpactUnitAll
)

// CalculateTradeSlippage
/**
 * Calculates avg/max slippage (price impact) for candidate trade
 *
 * @deprecated use calculateEstimatedPerpEntryPrice instead
 *
 * @param direction
 * @param amount
 * @param market
 * @param inputAssetType which asset is being traded
 * @param useSpread whether to consider spread with calculating slippage
 * @return [pctAvgSlippage, pctMaxSlippage, entryPrice, newPrice]
 *
 * 'pctAvgSlippage' =>  the percentage change to entryPrice (average est slippage in execution) : Precision PRICE_PRECISION
 *
 * 'pctMaxSlippage' =>  the percentage change to maxPrice (highest est slippage in execution) : Precision PRICE_PRECISION
 *
 * 'entryPrice' => the average price of the trade : Precision PRICE_PRECISION
 *
 * 'newPrice' => the price of the asset after the trade : Precision PRICE_PRECISION
 */
func CalculateTradeSlippage(
	direction drift.PositionDirection,
	amount *big.Int,
	market *drift.PerpMarket,
	inputAssetType drift.AssetType,
	oraclePriceData *oracles.OraclePriceData,
	useSpread bool, // true
) (*big.Int, *big.Int, *big.Int, *big.Int) {
	var oldPrice *big.Int

	if useSpread && market.Amm.BaseSpread > 0 {
		if direction == drift.PositionDirection_Long {
			oldPrice = CalculateAskPrice(market, oraclePriceData)
		} else {
			oldPrice = CalculateBidPrice(market, oraclePriceData)
		}
	} else {
		oldPrice = CalculateReservePrice(market, oraclePriceData)
	}
	if amount.Cmp(constants.ZERO) == 0 {
		return utils.BN(0), utils.BN(0), oldPrice, oldPrice
	}
	acquiredBaseReserve, acquiredQuoteReserve, acquiredQuoteAssetAmount := CalculateTradeAcquiredAmounts(
		direction,
		amount,
		market,
		inputAssetType,
		oraclePriceData,
		useSpread,
	)

	entryPrice := utils.DivX(
		utils.MulX(acquiredQuoteAssetAmount, constants.AMM_TO_QUOTE_PRECISION_RATIO, constants.PRICE_PRECISION),
		utils.AbsX(acquiredBaseReserve),
	)
	var amm *drift.Amm
	if useSpread && market.Amm.BaseSpread > 0 {
		baseAssetReserve, quoteAssetReserve, sqrtK, newPeg := CalculateUpdatedAMMSpreadReserves(
			&market.Amm,
			direction,
			oraclePriceData,
		)
		amm = &drift.Amm{
			BaseAssetReserve:  utils.Uint128(baseAssetReserve),
			QuoteAssetReserve: utils.Uint128(quoteAssetReserve),
			SqrtK:             utils.Uint128(sqrtK),
			PegMultiplier:     utils.Uint128(newPeg),
		}
	} else {
		amm = utils.NewPtr(market.Amm)
	}

	newPrice := CalculatePrice(
		utils.SubX(amm.BaseAssetReserve.BigInt(), acquiredBaseReserve),
		utils.SubX(amm.QuoteAssetReserve.BigInt(), acquiredQuoteReserve),
		amm.PegMultiplier.BigInt(),
	)

	if direction == drift.PositionDirection_Short {
		assert.Assert(newPrice.Cmp(oldPrice) <= 0)
	} else {
		assert.Assert(oldPrice.Cmp(newPrice) <= 0)
	}

	pctMaxSlippage := utils.AbsX(
		utils.DivX(
			utils.MulX(
				utils.SubX(
					newPrice,
					oldPrice,
				),
				constants.PRICE_PRECISION,
			),
			oldPrice,
		),
	)
	pctAvgSlippage := utils.AbsX(
		utils.DivX(
			utils.MulX(
				utils.SubX(
					entryPrice,
					oldPrice,
				),
				constants.PRICE_PRECISION,
			),
			oldPrice,
		),
	)
	return pctAvgSlippage, pctMaxSlippage, entryPrice, newPrice
}

// CalculateTradeAcquiredAmounts
/**
 * Calculates acquired amounts for trade executed
 * @param direction
 * @param amount
 * @param market
 * @param inputAssetType
 * @param useSpread
 * @return
 * 	| 'acquiredBase' =>  positive/negative change in user's base : BN AMM_RESERVE_PRECISION
 * 	| 'acquiredQuote' => positive/negative change in user's quote : BN TODO-PRECISION
 */
func CalculateTradeAcquiredAmounts(
	direction drift.PositionDirection,
	amount *big.Int,
	market *drift.PerpMarket,
	inputAssetType drift.AssetType, // quote
	oraclePriceData *oracles.OraclePriceData,
	useSpread bool, // true
) (*big.Int, *big.Int, *big.Int) {
	if amount.Cmp(constants.ZERO) == 0 {
		return utils.BN(0), utils.BN(0), utils.BN(0)
	}

	swapDirection := GetSwapDirection(inputAssetType, direction)

	var amm *drift.Amm
	if useSpread && market.Amm.BaseSpread > 0 {
		baseAssetReserve, quoteAssetReserve, sqrtK, newPeg := CalculateUpdatedAMMSpreadReserves(
			&market.Amm,
			direction,
			oraclePriceData,
		)
		amm = &drift.Amm{
			BaseAssetReserve:  utils.Uint128(baseAssetReserve),
			QuoteAssetReserve: utils.Uint128(quoteAssetReserve),
			SqrtK:             utils.Uint128(sqrtK),
			PegMultiplier:     utils.Uint128(newPeg),
		}
	} else {
		amm = utils.NewPtr(market.Amm)
	}

	newQuoteAssetReserve, newBaseAssetReserve := CalculateAmmReservesAfterSwap(amm, inputAssetType, amount, swapDirection)

	acquiredBase := utils.SubX(amm.BaseAssetReserve.BigInt(), newBaseAssetReserve)
	acquiredQuote := utils.SubX(amm.QuoteAssetReserve.BigInt(), newQuoteAssetReserve)
	acquiredQuoteAssetAmount := CalculateQuoteAssetAmountSwapped(
		utils.AbsX(acquiredQuote),
		amm.PegMultiplier.BigInt(),
		swapDirection,
	)
	return acquiredBase, acquiredQuote, acquiredQuoteAssetAmount
}

// CalculateTargetPriceTrade
/**
 * calculateTargetPriceTrade
 * simple function for finding arbitraging trades
 *
 * @deprecated
 *
 * @param market
 * @param targetPrice
 * @param pct optional default is 100% gap filling, can set smaller.
 * @param outputAssetType which asset to trade.
 * @param useSpread whether or not to consider the spread when calculating the trade size
 * @returns trade direction/size in order to push price to a targetPrice,
 *
 * [
 *   direction => direction of trade required, PositionDirection
 *   tradeSize => size of trade required, TODO-PRECISION
 *   entryPrice => the entry price for the trade, PRICE_PRECISION
 *   targetPrice => the target price PRICE_PRECISION
 * ]
 */
func CalculateTargetPriceTrade(
	market *drift.PerpMarket,
	targetPrice *big.Int,
	pct *big.Int, // MAXPCT
	outputAssetType drift.AssetType, // quote
	oraclePriceData *oracles.OraclePriceData,
	useSpread bool, // true
) (drift.PositionDirection, *big.Int, *big.Int, *big.Int) {
	assert.Assert(market.Amm.BaseAssetReserve.BigInt().Cmp(constants.ZERO) > 0)
	assert.Assert(targetPrice.Cmp(constants.ZERO) > 0)
	assert.Assert(pct.Cmp(MAXPCT) < 0 && pct.Cmp(constants.ZERO) > 0)

	reservePriceBefore := CalculateReservePrice(market, oraclePriceData)
	bidPriceBefore := CalculateBidPrice(market, oraclePriceData)
	askPriceBefore := CalculateAskPrice(market, oraclePriceData)

	var direction drift.PositionDirection
	if targetPrice.Cmp(reservePriceBefore) > 0 {
		priceGap := utils.SubX(targetPrice, reservePriceBefore)
		priceGapScaled := utils.DivX(utils.MulX(priceGap, pct), MAXPCT)
		targetPrice = utils.AddX(reservePriceBefore, priceGapScaled)
		direction = drift.PositionDirection_Long
	} else {
		priceGap := utils.SubX(reservePriceBefore, targetPrice)
		priceGapScaled := utils.DivX(utils.MulX(priceGap, pct), MAXPCT)
		targetPrice = utils.SubX(reservePriceBefore, priceGapScaled)
		direction = drift.PositionDirection_Short
	}

	var tradeSize, baseSize *big.Int

	var baseAssetReserveBefore, quoteAssetReserveBefore *big.Int

	peg := market.Amm.PegMultiplier.BigInt()

	if useSpread && market.Amm.BaseSpread > 0 {
		baseAssetReserve, quoteAssetReserve, _, newPeg := CalculateUpdatedAMMSpreadReserves(
			&market.Amm,
			direction,
			oraclePriceData,
		)
		baseAssetReserveBefore = utils.NewPtr(*baseAssetReserve)
		quoteAssetReserveBefore = utils.NewPtr(*quoteAssetReserve)
		peg = utils.NewPtr(*newPeg)
	} else {
		baseAssetReserveBefore = market.Amm.BaseAssetReserve.BigInt()
		quoteAssetReserveBefore = market.Amm.QuoteAssetReserve.BigInt()
	}

	invariant := utils.MulX(market.Amm.SqrtK.BigInt(), market.Amm.SqrtK.BigInt())
	k := utils.MulX(invariant, constants.PRICE_PRECISION)

	var baseAssetReserveAfter, quoteAssetReserveAfter *big.Int
	biasModifier := utils.BN(1)
	var markPriceAfter *big.Int

	if useSpread && targetPrice.Cmp(askPriceBefore) < 0 && targetPrice.Cmp(bidPriceBefore) > 0 {
		// no trade, market is at target
		if reservePriceBefore.Cmp(targetPrice) > 0 {
			direction = drift.PositionDirection_Short
		} else {
			direction = drift.PositionDirection_Long
		}
		tradeSize = utils.BN(0)
		return direction, tradeSize, targetPrice, targetPrice
	} else if reservePriceBefore.Cmp(targetPrice) > 0 {
		// overestimate y2
		baseAssetReserveAfter = utils.SubX(utils.SquareRootBN(
			utils.SubX(
				utils.DivX(
					utils.MulX(
						utils.DivX(
							k,
							targetPrice,
						),
						peg,
					),
					constants.PEG_PRECISION,
				),
				biasModifier,
			),
		), utils.BN(1))
		quoteAssetReserveAfter = utils.DivX(k, constants.PRICE_PRECISION, baseAssetReserveAfter)

		markPriceAfter = CalculatePrice(
			baseAssetReserveAfter,
			quoteAssetReserveAfter,
			peg,
		)
		direction = drift.PositionDirection_Short
		tradeSize = utils.DivX(
			utils.MulX(
				utils.SubX(
					quoteAssetReserveBefore,
					quoteAssetReserveAfter,
				),
				peg,
			),
			constants.PEG_PRECISION,
			constants.AMM_TO_QUOTE_PRECISION_RATIO,
		)
		baseSize = utils.SubX(baseAssetReserveAfter, baseAssetReserveBefore)
	} else if reservePriceBefore.Cmp(targetPrice) < 0 {
		// userestimate y2
		baseAssetReserveAfter = utils.AddX(utils.SquareRootBN(
			utils.AddX(
				utils.DivX(
					utils.MulX(
						utils.DivX(
							k,
							targetPrice,
						),
						peg,
					),
					constants.PEG_PRECISION,
				),
				biasModifier,
			),
		), utils.BN(1))
		quoteAssetReserveAfter = utils.DivX(k, constants.PRICE_PRECISION, baseAssetReserveAfter)

		markPriceAfter = CalculatePrice(
			baseAssetReserveAfter,
			quoteAssetReserveAfter,
			peg,
		)
		direction = drift.PositionDirection_Long
		tradeSize = utils.DivX(
			utils.MulX(
				utils.SubX(
					quoteAssetReserveAfter,
					quoteAssetReserveBefore,
				),
				peg,
			),
			constants.PEG_PRECISION,
			constants.AMM_TO_QUOTE_PRECISION_RATIO,
		)
		baseSize = utils.SubX(baseAssetReserveBefore, baseAssetReserveAfter)
	} else {
		// no trade, market is at target
		direction = drift.PositionDirection_Long
		tradeSize = utils.BN(0)
		return direction, tradeSize, targetPrice, targetPrice
	}

	var tp1, tp2, originalDiff *big.Int

	if direction == drift.PositionDirection_Short {
		tp1 = utils.NewPtr(*markPriceAfter)
		tp2 = utils.NewPtr(*targetPrice)
		originalDiff = utils.SubX(reservePriceBefore, targetPrice)

	} else {
		tp1 = utils.NewPtr(*targetPrice)
		tp2 = utils.NewPtr(*markPriceAfter)
		originalDiff = utils.SubX(targetPrice, reservePriceBefore)
	}

	entryPrice := utils.DivX(
		utils.MulX(
			tradeSize,
			constants.AMM_TO_QUOTE_PRECISION_RATIO,
			constants.PRICE_PRECISION,
		),
		utils.AbsX(baseSize),
	)

	assert.Assert(utils.SubX(tp1, tp2).Cmp(originalDiff) <= 0, "Target Price Calculation incorrect")
	assert.Assert(tp2.Cmp(tp1) <= 0 || utils.AbsX(utils.SubX(tp2, tp1)).Cmp(utils.BN(100000)) < 0,
		fmt.Sprintf("Target Price Calculation incorrect %s>=%s err: %s",
			tp2.String(),
			tp1.String(),
			utils.AbsX(utils.SubX(tp2, tp1)).String(),
		),
	)
	if outputAssetType == drift.AssetType_Quote {
		return direction, tradeSize, entryPrice, targetPrice
	} else {
		return direction, baseSize, entryPrice, targetPrice
	}
}

// CalculateEstimatedPerpEntryPrice
/**
 * Calculates the estimated entry price and price impact of order, in base or quote
 * Price impact is based on the difference between the entry price and the best bid/ask price (whether it's dlob or vamm)
 *
 * @param assetType
 * @param amount
 * @param direction
 * @param market
 * @param oraclePriceData
 * @param dlob
 * @param slot
 * @param usersToSkip
 */
func CalculateEstimatedPerpEntryPrice(
	assetType drift.AssetType,
	amount *big.Int,
	direction drift.PositionDirection,
	market *drift.PerpMarket,
	oraclePriceData *oracles.OraclePriceData,
	dlob types.IDLOB,
	slot uint64,
	usersToSkip map[solana.PublicKey]bool,
) (*big.Int, *big.Int, *big.Int, *big.Int, *big.Int, *big.Int) {
	if amount.Cmp(constants.ZERO) == 0 {
		return utils.BN(0), utils.BN(0), utils.BN(0), utils.BN(0), utils.BN(0), utils.BN(0)
	}

	takerIsLong := direction == drift.PositionDirection_Long
	limitOrders := utils.TTM[*common.Generator[types.IDLOBNode, int]](takerIsLong,
		func() *common.Generator[types.IDLOBNode, int] {
			return dlob.GetRestingLimitAsks(
				market.MarketIndex,
				slot,
				drift.MarketType_Perp,
				oraclePriceData,
				nil)
		},
		func() *common.Generator[types.IDLOBNode, int] {
			return dlob.GetRestingLimitBids(
				market.MarketIndex,
				slot,
				drift.MarketType_Perp,
				oraclePriceData,
				nil)

		},
	)

	swapDirection := GetSwapDirection(assetType, direction)

	baseAssetReserve, quoteAssetReserve, sqrtK, newPeg := CalculateUpdatedAMMSpreadReserves(
		&market.Amm,
		direction,
		oraclePriceData,
	)
	amm := &drift.Amm{
		BaseAssetReserve:  utils.Uint128(baseAssetReserve),
		QuoteAssetReserve: utils.Uint128(quoteAssetReserve),
		SqrtK:             utils.Uint128(sqrtK),
		PegMultiplier:     utils.Uint128(newPeg),
	}

	ammBids, ammAsks := CalculateMarketOpenBidAsk(
		market.Amm.BaseAssetReserve.BigInt(),
		market.Amm.MinBaseAssetReserve.BigInt(),
		market.Amm.MaxBaseAssetReserve.BigInt(),
		utils.BN(market.Amm.OrderStepSize),
	)

	var ammLiquidity *big.Int
	if assetType == drift.AssetType_Base {
		ammLiquidity = utils.TT(takerIsLong, utils.AbsX(ammAsks), utils.NewPtr(*ammBids))
	} else {
		afterSwapQuoteReserves, _ := CalculateAmmReservesAfterSwap(
			amm,
			drift.AssetType_Quote,
			utils.TT(takerIsLong, utils.AbsX(ammAsks), utils.NewPtr(*ammBids)),
			GetSwapDirection(drift.AssetType_Base, direction),
		)

		ammLiquidity = CalculateQuoteAssetAmountSwapped(
			utils.AbsX(utils.SubX(amm.QuoteAssetReserve.BigInt(), afterSwapQuoteReserves)),
			amm.PegMultiplier.BigInt(),
			swapDirection,
		)
	}

	invariant := utils.MulX(amm.SqrtK.BigInt(), amm.SqrtK.BigInt())

	bestPrice := CalculatePrice(
		amm.BaseAssetReserve.BigInt(),
		amm.QuoteAssetReserve.BigInt(),
		amm.PegMultiplier.BigInt(),
	)

	cumulativeBaseFilled := utils.BN(0)
	cumulativeQuoteFilled := utils.BN(0)

	limitOrder, _, _ := limitOrders.Next()
	if limitOrder != nil {
		limitOrderPrice := limitOrder.GetPrice(oraclePriceData, slot)
		bestPrice = utils.TTM[*big.Int](
			takerIsLong,
			utils.Min(limitOrderPrice, bestPrice),
			utils.Max(limitOrderPrice, bestPrice),
		)
	}

	worstPrice := utils.NewPtr(*bestPrice)

	if assetType == drift.AssetType_Base {
		for cumulativeBaseFilled.Cmp(amount) != 0 && (ammLiquidity.Cmp(constants.ZERO) > 0 || limitOrder != nil) {
			limitOrderPrice := utils.TTM[*big.Int](limitOrder == nil, nil, func() *big.Int { return limitOrder.GetPrice(oraclePriceData, slot) })

			var maxAmmFill *big.Int
			if limitOrderPrice != nil && limitOrderPrice.Cmp(constants.ZERO) != 0 {
				newBaseReserves := utils.SquareRootBN(
					utils.DivX(
						utils.MulX(
							invariant,
							constants.PRICE_PRECISION,
							amm.PegMultiplier.BigInt(),
						),
						limitOrderPrice,
						constants.PEG_PRECISION,
					),
				)
				// will be zero if the limit order price is better than the amm price
				maxAmmFill = utils.TTM[*big.Int](
					takerIsLong,
					utils.SubX(amm.BaseAssetReserve.BigInt(), newBaseReserves),
					utils.SubX(newBaseReserves, amm.BaseAssetReserve.BigInt()),
				)
			} else {
				maxAmmFill = utils.SubX(amount, cumulativeBaseFilled)
			}

			maxAmmFill = utils.Min(maxAmmFill, ammLiquidity)

			if maxAmmFill.Cmp(constants.ZERO) > 0 {
				baseFilled := utils.Min(utils.SubX(amount, cumulativeBaseFilled), maxAmmFill)
				afterSwapQuoteReserves, afterSwapBaseReserves := CalculateAmmReservesAfterSwap(
					amm,
					drift.AssetType_Base,
					baseFilled,
					swapDirection,
				)

				ammLiquidity = utils.SubX(ammLiquidity, baseFilled)

				quoteFilled := CalculateQuoteAssetAmountSwapped(
					utils.AbsX(utils.SubX(amm.QuoteAssetReserve.BigInt(), afterSwapQuoteReserves)),
					amm.PegMultiplier.BigInt(),
					swapDirection,
				)

				cumulativeBaseFilled = utils.AddX(cumulativeBaseFilled, baseFilled)
				cumulativeQuoteFilled = utils.AddX(cumulativeQuoteFilled, quoteFilled)

				amm.BaseAssetReserve = utils.Uint128(afterSwapBaseReserves)
				amm.QuoteAssetReserve = utils.Uint128(afterSwapQuoteReserves)

				worstPrice = CalculatePrice(
					amm.BaseAssetReserve.BigInt(),
					amm.QuoteAssetReserve.BigInt(),
					amm.PegMultiplier.BigInt(),
				)

				if cumulativeBaseFilled.Cmp(amount) == 0 {
					break
				}
			}

			if limitOrder == nil {
				continue
			}

			if utils.MapHas(usersToSkip, solana.MPK(limitOrder.GetUserAccount())) {
				continue
			}

			baseFilled := utils.Min(
				utils.BN(limitOrder.GetOrder().BaseAssetAmount-limitOrder.GetOrder().BaseAssetAmountFilled),
				utils.SubX(amount, cumulativeBaseFilled),
			)
			quoteFilled := utils.DivX(utils.MulX(baseFilled, limitOrderPrice), constants.BASE_PRECISION)

			cumulativeBaseFilled = utils.AddX(cumulativeBaseFilled, baseFilled)
			cumulativeQuoteFilled = utils.AddX(cumulativeQuoteFilled, quoteFilled)

			worstPrice = limitOrderPrice

			if cumulativeBaseFilled.Cmp(amount) == 0 {
				break
			}
			limitOrder, _, _ = limitOrders.Next()
		}
	} else {
		for cumulativeQuoteFilled.Cmp(amount) != 0 && (ammLiquidity.Cmp(constants.ZERO) > 0 || limitOrder != nil) {
			limitOrderPrice := utils.TTM[*big.Int](limitOrder == nil, nil, func() *big.Int { return limitOrder.GetPrice(oraclePriceData, slot) })

			var maxAmmFill *big.Int
			if limitOrderPrice != nil && limitOrderPrice.Cmp(constants.ZERO) != 0 {
				newQuoteReserves := utils.SquareRootBN(
					utils.DivX(
						utils.MulX(
							invariant,
							constants.PEG_PRECISION,
							limitOrderPrice,
						),
						amm.PegMultiplier.BigInt(),
						constants.PRICE_PRECISION,
					),
				)
				// will be zero if the limit order price is better than the amm price
				maxAmmFill = utils.TTM[*big.Int](
					takerIsLong,
					utils.SubX(newQuoteReserves, amm.BaseAssetReserve.BigInt()),
					utils.SubX(amm.BaseAssetReserve.BigInt(), newQuoteReserves),
				)
			} else {
				maxAmmFill = utils.SubX(amount, cumulativeQuoteFilled)
			}

			maxAmmFill = utils.Min(maxAmmFill, ammLiquidity)

			if maxAmmFill.Cmp(constants.ZERO) > 0 {
				quoteFilled := utils.Min(utils.SubX(amount, cumulativeQuoteFilled), maxAmmFill)
				afterSwapQuoteReserves, afterSwapBaseReserves := CalculateAmmReservesAfterSwap(
					amm,
					drift.AssetType_Quote,
					quoteFilled,
					swapDirection,
				)

				ammLiquidity = utils.SubX(ammLiquidity, quoteFilled)

				baseFilled := utils.AbsX(utils.SubX(afterSwapBaseReserves, amm.BaseAssetReserve.BigInt()))

				cumulativeBaseFilled = utils.AddX(cumulativeBaseFilled, baseFilled)
				cumulativeQuoteFilled = utils.AddX(cumulativeQuoteFilled, quoteFilled)

				amm.BaseAssetReserve = utils.Uint128(afterSwapBaseReserves)
				amm.QuoteAssetReserve = utils.Uint128(afterSwapQuoteReserves)

				worstPrice = CalculatePrice(
					amm.BaseAssetReserve.BigInt(),
					amm.QuoteAssetReserve.BigInt(),
					amm.PegMultiplier.BigInt(),
				)

				if cumulativeBaseFilled.Cmp(amount) == 0 {
					break
				}
			}

			if limitOrder == nil {
				continue
			}

			if utils.MapHas(usersToSkip, solana.MPK(limitOrder.GetUserAccount())) {
				continue
			}

			quoteFilled := utils.Min(
				utils.DivX(
					utils.MulX(
						utils.BN(limitOrder.GetOrder().BaseAssetAmount-limitOrder.GetOrder().BaseAssetAmountFilled),
						limitOrderPrice,
					),
					constants.BASE_PRECISION,
				),
				utils.SubX(amount, cumulativeQuoteFilled),
			)
			baseFilled := utils.DivX(utils.MulX(quoteFilled, constants.BASE_PRECISION), limitOrderPrice)

			cumulativeBaseFilled = utils.AddX(cumulativeBaseFilled, baseFilled)
			cumulativeQuoteFilled = utils.AddX(cumulativeQuoteFilled, quoteFilled)

			worstPrice = limitOrderPrice

			if cumulativeQuoteFilled.Cmp(amount) == 0 {
				break
			}
			limitOrder, _, _ = limitOrders.Next()
		}

	}

	entryPrice := utils.TTM[*big.Int](
		cumulativeBaseFilled != nil && cumulativeBaseFilled.Cmp(constants.ZERO) > 0,
		func() *big.Int {
			return utils.DivX(utils.MulX(cumulativeQuoteFilled, constants.BASE_PRECISION), cumulativeBaseFilled)
		},
		utils.BN(0),
	)
	priceImpact := utils.TTM[*big.Int](
		bestPrice != nil && bestPrice.Cmp(constants.ZERO) > 0,
		func() *big.Int {
			return utils.AbsX(
				utils.DivX(
					utils.MulX(
						utils.SubX(
							entryPrice,
							bestPrice,
						),
						constants.PRICE_PRECISION,
					),
					bestPrice,
				),
			)
		},
		utils.BN(0),
	)
	return entryPrice,
		priceImpact,
		bestPrice,
		worstPrice,
		cumulativeBaseFilled,
		cumulativeQuoteFilled
}

// CalculateEstimatedSpotEntryPrice
/**
 * Calculates the estimated entry price and price impact of order, in base or quote
 * Price impact is based on the difference between the entry price and the best bid/ask price (whether it's dlob or serum)
 *
 * @param assetType
 * @param amount
 * @param direction
 * @param market
 * @param oraclePriceData
 * @param dlob
 * @param serumBids
 * @param serumAsks
 * @param slot
 * @param usersToSkip
 */
func CalculateEstimatedSpotEntryPrice(
	assetType drift.AssetType,
	amount *big.Int,
	direction drift.PositionDirection,
	market *drift.SpotMarket,
	oraclePriceData *oracles.OraclePriceData,
	dlob types.IDLOB,
	serumBids *serum.Orderbook,
	serumAsks *serum.Orderbook,
	slot uint64,
	usersToSkip map[solana.PublicKey]bool,
) (*big.Int, *big.Int, *big.Int, *big.Int, *big.Int, *big.Int) {
	if amount.Cmp(constants.ZERO) == 0 {
		return utils.BN(0), utils.BN(0), utils.BN(0), utils.BN(0), utils.BN(0), utils.BN(0)
	}

	basePrecision := utils.BN(int64(math.Pow10(int(market.Decimals))))

	takerIsLong := direction == drift.PositionDirection_Long
	dlobLimitOrders := utils.TTM[*common.Generator[types.IDLOBNode, int]](takerIsLong,
		func() *common.Generator[types.IDLOBNode, int] {
			return dlob.GetRestingLimitAsks(
				market.MarketIndex,
				slot,
				drift.MarketType_Spot,
				oraclePriceData,
				nil)
		},
		func() *common.Generator[types.IDLOBNode, int] {
			return dlob.GetRestingLimitBids(
				market.MarketIndex,
				slot,
				drift.MarketType_Spot,
				oraclePriceData,
				nil)

		},
	)
	serumLimitOrders := utils.TTM[[]*dloblib.L2Level](takerIsLong,
		func() []*dloblib.L2Level {
			return serumAsks.GetL2(100)
		},
		func() []*dloblib.L2Level {
			return serumBids.GetL2(100)
		},
	)
	cumulativeBaseFilled := utils.BN(0)
	cumulativeQuoteFilled := utils.BN(0)

	dlobLimitOrder, _, _ := dlobLimitOrders.Next()

	var serumLimitOrder *dloblib.L2Level
	if len(serumLimitOrders) > 0 {
		serumLimitOrder = serumLimitOrders[0]
		serumLimitOrders = serumLimitOrders[1:]
	}

	var dlobLimitOrderPrice *big.Int
	if dlobLimitOrder != nil {
		dlobLimitOrderPrice = dlobLimitOrder.GetPrice(oraclePriceData, slot)
	}
	var serumLimitOrderPrice *big.Int
	if serumLimitOrder != nil {
		serumLimitOrderPrice = utils.MulX(serumLimitOrder.Price, constants.PRICE_PRECISION)
	}

	bestPrice := utils.TTM[*big.Int](
		takerIsLong,
		func() *big.Int {
			return utils.Min(
				utils.TT(serumLimitOrderPrice == nil, utils.IntX(constants.BN_MAX), serumLimitOrderPrice),
				utils.TT(dlobLimitOrderPrice == nil, utils.IntX(constants.BN_MAX), dlobLimitOrderPrice),
			)
		},
		func() *big.Int {
			return utils.Max(
				utils.TT(serumLimitOrderPrice == nil, utils.BN(0), serumLimitOrderPrice),
				utils.TT(dlobLimitOrderPrice == nil, utils.BN(0), dlobLimitOrderPrice),
			)
		},
	)
	worstPrice := utils.NewPtrp(bestPrice)

	if assetType == drift.AssetType_Base {
		for cumulativeBaseFilled.Cmp(amount) != 0 && (dlobLimitOrder != nil || serumLimitOrder != nil) {
			dlobLimitOrderPrice = utils.TTM[*big.Int](
				dlobLimitOrder != nil,
				func() *big.Int {
					return dlobLimitOrder.GetPrice(
						oraclePriceData,
						slot,
					)
				},
				nil,
			)
			serumLimitOrderPrice = utils.TTM[*big.Int](
				serumLimitOrder != nil,
				func() *big.Int {
					return utils.MulX(serumLimitOrder.Price, constants.PRICE_PRECISION)
				},
				nil,
			)

			useSerum := utils.TTM[bool](
				takerIsLong,
				func() bool {
					return utils.TT(
						serumLimitOrderPrice != nil,
						serumLimitOrderPrice,
						utils.IntX(constants.BN_MAX),
					).Cmp(utils.TT(
						dlobLimitOrderPrice != nil,
						dlobLimitOrderPrice,
						utils.IntX(constants.BN_MAX),
					)) < 0
				},
				func() bool {
					return utils.TT(
						serumLimitOrderPrice != nil,
						serumLimitOrderPrice,
						utils.BN(0),
					).Cmp(utils.TT(
						dlobLimitOrderPrice != nil,
						dlobLimitOrderPrice,
						utils.BN(0),
					)) > 0
				},
			)

			if !useSerum {
				if dlobLimitOrder != nil && utils.MapHas(usersToSkip, solana.MPK(dlobLimitOrder.GetUserAccount())) {
					continue
				}

				baseFilled := utils.Min(
					utils.BN(dlobLimitOrder.GetOrder().BaseAssetAmount-dlobLimitOrder.GetOrder().BaseAssetAmountFilled),
					utils.SubX(amount, cumulativeBaseFilled),
				)
				quoteFilled := utils.DivX(
					utils.MulX(
						baseFilled,
						dlobLimitOrderPrice,
					),
					basePrecision,
				)

				cumulativeBaseFilled = utils.AddX(cumulativeBaseFilled, baseFilled)
				cumulativeQuoteFilled = utils.AddX(cumulativeQuoteFilled, quoteFilled)

				worstPrice = utils.NewPtrp(dlobLimitOrderPrice)

				dlobLimitOrder, _, _ = dlobLimitOrders.Next()
			} else {
				baseFilled := utils.Min(
					utils.MulX(serumLimitOrder.Size, basePrecision),
					utils.SubX(amount, cumulativeBaseFilled),
				)
				quoteFilled := utils.DivX(
					utils.MulX(baseFilled, serumLimitOrderPrice),
					basePrecision,
				)

				cumulativeBaseFilled = utils.AddX(cumulativeBaseFilled, baseFilled)
				cumulativeQuoteFilled = utils.AddX(cumulativeQuoteFilled, quoteFilled)

				worstPrice = utils.NewPtrp(serumLimitOrderPrice)

				if len(serumLimitOrders) > 0 {
					serumLimitOrder = serumLimitOrders[0]
					serumLimitOrders = serumLimitOrders[1:]
				} else {
					serumLimitOrder = nil
				}
			}
		}
	} else {
		for cumulativeQuoteFilled.Cmp(amount) != 0 && (dlobLimitOrder != nil || serumLimitOrder != nil) {
			dlobLimitOrderPrice = utils.TTM[*big.Int](
				dlobLimitOrder != nil,
				func() *big.Int {
					return dlobLimitOrder.GetPrice(
						oraclePriceData,
						slot,
					)
				},
				nil,
			)
			serumLimitOrderPrice = utils.TTM[*big.Int](
				serumLimitOrder != nil,
				func() *big.Int {
					return utils.MulX(serumLimitOrder.Price, constants.PRICE_PRECISION)
				},
				nil,
			)

			useSerum := utils.TTM[bool](
				takerIsLong,
				func() bool {
					return utils.TT(
						serumLimitOrderPrice != nil,
						serumLimitOrderPrice,
						utils.IntX(constants.BN_MAX),
					).Cmp(utils.TT(
						dlobLimitOrderPrice != nil,
						dlobLimitOrderPrice,
						utils.IntX(constants.BN_MAX),
					)) < 0
				},
				func() bool {
					return utils.TT(
						serumLimitOrderPrice != nil,
						serumLimitOrderPrice,
						utils.BN(0),
					).Cmp(utils.TT(
						dlobLimitOrderPrice != nil,
						dlobLimitOrderPrice,
						utils.BN(0),
					)) > 0
				},
			)

			if !useSerum {
				if dlobLimitOrder != nil && utils.MapHas(usersToSkip, solana.MPK(dlobLimitOrder.GetUserAccount())) {
					continue
				}

				quoteFilled := utils.Min(
					utils.DivX(
						utils.MulX(
							utils.BN(dlobLimitOrder.GetOrder().BaseAssetAmount-dlobLimitOrder.GetOrder().BaseAssetAmountFilled),
							dlobLimitOrderPrice,
						),
						basePrecision,
					),
					utils.SubX(amount, cumulativeQuoteFilled),
				)
				baseFilled := utils.DivX(
					utils.MulX(
						quoteFilled,
						basePrecision,
					),
					dlobLimitOrderPrice,
				)

				cumulativeBaseFilled = utils.AddX(cumulativeBaseFilled, baseFilled)
				cumulativeQuoteFilled = utils.AddX(cumulativeQuoteFilled, quoteFilled)

				worstPrice = utils.NewPtrp(dlobLimitOrderPrice)

				dlobLimitOrder, _, _ = dlobLimitOrders.Next()
			} else {
				serumOrderBaseAmount := utils.MulX(serumLimitOrder.Size, basePrecision)

				quoteFilled := utils.Min(
					utils.DivX(
						utils.MulX(serumOrderBaseAmount, serumLimitOrderPrice),
						basePrecision,
					),
					utils.SubX(amount, cumulativeQuoteFilled),
				)
				baseFilled := utils.DivX(
					utils.MulX(quoteFilled, basePrecision),
					serumLimitOrderPrice,
				)

				cumulativeBaseFilled = utils.AddX(cumulativeBaseFilled, baseFilled)
				cumulativeQuoteFilled = utils.AddX(cumulativeQuoteFilled, quoteFilled)

				worstPrice = utils.NewPtrp(serumLimitOrderPrice)

				if len(serumLimitOrders) > 0 {
					serumLimitOrder = serumLimitOrders[0]
					serumLimitOrders = serumLimitOrders[1:]
				} else {
					serumLimitOrder = nil
				}
			}
		}
	}

	entryPrice := utils.TTM[*big.Int](
		cumulativeBaseFilled != nil && cumulativeBaseFilled.Cmp(constants.ZERO) > 0,
		func() *big.Int {
			return utils.DivX(utils.MulX(cumulativeQuoteFilled, basePrecision), cumulativeBaseFilled)
		},
		utils.BN(0),
	)

	priceImpact := utils.TTM[*big.Int](
		bestPrice != nil && bestPrice.Cmp(constants.ZERO) > 0,
		func() *big.Int {
			return utils.AbsX(
				utils.DivX(
					utils.MulX(
						utils.SubX(entryPrice, bestPrice),
						constants.PRICE_PRECISION,
					),
					bestPrice,
				),
			)
		},
		utils.BN(0),
	)

	return entryPrice,
		priceImpact,
		bestPrice,
		worstPrice,
		cumulativeBaseFilled,
		cumulativeQuoteFilled
}

func CalculateEstimatedEntryPriceWithL2(
	assetType drift.AssetType,
	amount *big.Int,
	direction drift.PositionDirection,
	basePrecision *big.Int,
	l2 *dloblib.L2OrderBook,
) (*big.Int, *big.Int, *big.Int, *big.Int, *big.Int, *big.Int) {
	takerIsLong := direction == drift.PositionDirection_Long

	cumulativeBaseFilled := utils.BN(0)
	cumulativeQuoteFilled := utils.BN(0)

	levels := utils.TT(takerIsLong, l2.Asks, l2.Bids)
	var nextLevel *dloblib.L2Level
	if len(levels) > 0 {
		nextLevel = levels[0]
		levels = levels[1:]
	}

	var bestPrice *big.Int
	var worstPrice *big.Int
	if nextLevel != nil {
		bestPrice = nextLevel.Price
		worstPrice = nextLevel.Price
	} else {
		bestPrice = utils.IntX(utils.TT(takerIsLong, constants.BN_MAX, constants.ZERO))
		worstPrice = utils.NewPtrp(bestPrice)
	}

	if assetType == drift.AssetType_Base {
		for cumulativeBaseFilled.Cmp(amount) != 0 && nextLevel != nil {
			price := nextLevel.Price
			size := nextLevel.Size

			worstPrice = utils.NewPtrp(price)

			baseFilled := utils.Min(size, utils.SubX(amount, cumulativeBaseFilled))
			quoteFilled := utils.DivX(utils.MulX(baseFilled, price), basePrecision)

			cumulativeBaseFilled = utils.AddX(cumulativeBaseFilled, baseFilled)
			cumulativeQuoteFilled = utils.AddX(cumulativeQuoteFilled, quoteFilled)

			if len(levels) > 0 {
				nextLevel = levels[0]
				levels = levels[1:]
			} else {
				nextLevel = nil
			}
		}
	} else {
		for cumulativeQuoteFilled.Cmp(amount) != 0 && nextLevel != nil {
			price := nextLevel.Price
			size := nextLevel.Size

			worstPrice = utils.NewPtrp(price)

			quoteFilled := utils.Min(utils.DivX(utils.MulX(size, price), basePrecision), utils.SubX(amount, cumulativeQuoteFilled))
			baseFilled := utils.DivX(utils.MulX(quoteFilled, basePrecision), price)

			cumulativeBaseFilled = utils.AddX(cumulativeBaseFilled, baseFilled)
			cumulativeQuoteFilled = utils.AddX(cumulativeQuoteFilled, quoteFilled)

			if len(levels) > 0 {
				nextLevel = levels[0]
				levels = levels[1:]
			} else {
				nextLevel = nil
			}
		}
	}
	entryPrice := utils.TTM[*big.Int](
		cumulativeBaseFilled != nil && cumulativeBaseFilled.Cmp(constants.ZERO) > 0,
		func() *big.Int {
			return utils.DivX(utils.MulX(cumulativeQuoteFilled, basePrecision), cumulativeBaseFilled)
		},
		utils.BN(0),
	)

	priceImpact := utils.TTM[*big.Int](
		bestPrice != nil && bestPrice.Cmp(constants.ZERO) > 0,
		func() *big.Int {
			return utils.AbsX(
				utils.DivX(
					utils.MulX(
						utils.SubX(entryPrice, bestPrice),
						constants.PRICE_PRECISION,
					),
					bestPrice,
				),
			)
		},
		utils.BN(0),
	)

	return entryPrice,
		priceImpact,
		bestPrice,
		worstPrice,
		cumulativeBaseFilled,
		cumulativeQuoteFilled
}

func GetUser30dRollingVolumeEstimate(
	userStatsAccount *drift.UserStats,
	now int64,
) uint64 {
	if now <= 0 {
		now = time.Now().Unix()
	}
	sinceLastTaker := uint64(max(now-userStatsAccount.LastTakerVolume30DTs, 0))
	sinceLastMaker := uint64(max(now-userStatsAccount.LastMakerVolume30DTs, 0))
	thirtyDaysInSeconds := uint64(60 * 60 * 24 * 30)
	last30dVolume := userStatsAccount.TakerVolume30D*max(thirtyDaysInSeconds-sinceLastTaker, 0)/thirtyDaysInSeconds +
		(userStatsAccount.MakerVolume30D * max(thirtyDaysInSeconds-sinceLastMaker, 0) / thirtyDaysInSeconds)
	return last30dVolume
}
