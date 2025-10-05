package dlob

import (
	"driftgo/assert"
	"driftgo/common"
	"driftgo/constants"
	"driftgo/dlob/types"
	"driftgo/lib/drift"
	"driftgo/math"
	oracles "driftgo/oracles/types"
	"driftgo/utils"
	"math/big"
	"slices"
	"time"
)

// GetL2GeneratorFromDLOBNodes
/**
 * Get an {@link Generator<L2Level>} generator from a {@link Generator<DLOBNode>}
 * @param dlobNodes e.g. {@link DLOB#getRestingLimitAsks} or {@link DLOB#getRestingLimitBids}
 * @param oraclePriceData
 * @param slot
 */
func GetL2GeneratorFromDLOBNodes(
	dlobNodes *common.Generator[types.IDLOBNode, int],
	oraclePriceData *oracles.OraclePriceData,
	slot uint64,
) *common.Generator[*types.L2Level, int] {
	return common.NewGenerator(func(yield common.YieldFn[*types.L2Level, int]) {
		dlobNodes.Each(func(dlobNode types.IDLOBNode, key int) bool {
			size := utils.BN(dlobNode.GetOrder().BaseAssetAmount - dlobNode.GetOrder().BaseAssetAmountFilled)
			yield(&types.L2Level{
				Price: dlobNode.GetPrice(oraclePriceData, slot),
				Size:  size,
				Sources: map[types.LiquiditySource]*big.Int{
					types.LiquiditySourceDlob: size,
				},
			}, key)
			return false
		})
	})
}

type GeneratorL2LevelItem struct {
	Next      *types.L2Level
	Done      bool
	Generator *common.Generator[*types.L2Level, int]
}

func MergeL2LevelGenerators(
	l2LevelGenerators []*common.Generator[*types.L2Level, int],
	compare func(*types.L2Level, *types.L2Level) bool,
) *common.Generator[*types.L2Level, int] {
	return common.NewGenerator(func(yield common.YieldFn[*types.L2Level, int]) {
		idx := 0
		var generators []*GeneratorL2LevelItem
		for _, generator := range l2LevelGenerators {
			nextItem, _, done := generator.Next()
			generators = append(generators, &GeneratorL2LevelItem{
				Next:      nextItem,
				Done:      done,
				Generator: generator,
			})
		}
		for {
			var best *GeneratorL2LevelItem
			for _, next1 := range generators {
				if next1.Done {
					continue
				}
				if best == nil {
					best = next1
					continue
				}
				if compare(next1.Next, best.Next) {
					best = next1
				}
			}
			if best != nil {
				yield(best.Next, idx)
				idx++
				best.Next, _, best.Done = best.Generator.Next()
			} else {
				break
			}
		}
	})
}

func CreateL2Levels(
	generator *common.Generator[*types.L2Level, int],
	depth int,
) []*types.L2Level {
	var levels []*types.L2Level
	generator.Each(func(level *types.L2Level, key int) bool {
		price := level.Price
		size := level.Size
		if len(levels) > 0 && levels[len(levels)-1].Price.Cmp(price) == 0 {
			currentLevel := levels[len(levels)-1]
			currentLevel.Size = utils.AddX(currentLevel.Size, size)
			for source, size1 := range level.Sources {
				_, exists := currentLevel.Sources[source]
				if exists {
					currentLevel.Sources[source] = utils.AddX(currentLevel.Sources[source], size1)
				} else {
					currentLevel.Sources[source] = size1
				}
			}
		} else if len(levels) == depth {
			return true
		} else {
			levels = append(levels, level)
		}
		return false
	})
	return levels
}

func GetVammL2Generator(
	marketAccount *drift.PerpMarket,
	oraclePriceData *oracles.OraclePriceData,
	numOrders int,
	now int64,
	topOfBookQuoteAmounts []*big.Int,
) *types.L2OrderBookGenerator {
	var l2OrderBookGenerator types.L2OrderBookGenerator
	numBaseOrders := numOrders
	if len(topOfBookQuoteAmounts) > 0 {
		numBaseOrders = numOrders - len(topOfBookQuoteAmounts)
		assert.Assert(len(topOfBookQuoteAmounts) < numOrders)
	}
	updatedAmm := math.CalculateUpdatedAMM(&marketAccount.Amm, oraclePriceData)
	openBids, openAsks := math.CalculateMarketOpenBidAsk(
		updatedAmm.BaseAssetReserve.BigInt(),
		updatedAmm.MinBaseAssetReserve.BigInt(),
		updatedAmm.MaxBaseAssetReserve.BigInt(),
		utils.BN(updatedAmm.OrderStepSize),
	)

	minOrderSize := marketAccount.Amm.MinOrderSize
	if openBids.Cmp(utils.BN(minOrderSize*2)) < 0 {
		openBids = utils.BN(0)
	}

	if utils.AbsX(openAsks).Cmp(utils.BN(minOrderSize*2)) < 0 {
		openAsks = utils.BN(0)
	}

	if now <= 0 {
		now = time.Now().Unix()
	}

	bidReserves, askReserves := math.CalculateSpreadReserves(
		updatedAmm,
		oraclePriceData,
		now,
	)

	topOfBookBidSize := utils.BN(0)
	bidSize := utils.DivX(openBids, utils.BN(numBaseOrders))
	bidAmm := &drift.Amm{
		BaseAssetReserve:  utils.Uint128(bidReserves.Base),
		QuoteAssetReserve: utils.Uint128(bidReserves.Quote),
		SqrtK:             updatedAmm.SqrtK,
		PegMultiplier:     updatedAmm.PegMultiplier,
	}

	l2OrderBookGenerator.GetL2Bids = func() *common.Generator[*types.L2Level, int] {
		return common.NewGenerator(func(yield common.YieldFn[*types.L2Level, int]) {
			numBids := 0
			for numBids < numOrders && bidSize.Cmp(constants.ZERO) > 0 {
				quoteSwapped := utils.BN(0)
				baseSwapped := utils.BN(0)
				afterSwapQuoteReserves := utils.BN(0)
				afterSwapBaseReserves := utils.BN(0)

				if len(topOfBookQuoteAmounts) > 0 && numBids < len(topOfBookQuoteAmounts) {
					remainingBaseLiquidity := utils.SubX(openBids, topOfBookBidSize)
					quoteSwapped = topOfBookQuoteAmounts[numBids]
					afterSwapQuoteReserves, afterSwapBaseReserves = math.CalculateAmmReservesAfterSwap(
						bidAmm,
						drift.AssetType_Quote,
						quoteSwapped,
						drift.SwapDirection_Remove,
					)

					baseSwapped = utils.AbsX(utils.SubX(bidAmm.BaseAssetReserve.BigInt(), afterSwapBaseReserves))
					if baseSwapped.Cmp(constants.ZERO) == 0 {
						return
					}
					if remainingBaseLiquidity.Cmp(baseSwapped) < 0 {
						baseSwapped = remainingBaseLiquidity
						afterSwapQuoteReserves, afterSwapBaseReserves = math.CalculateAmmReservesAfterSwap(
							bidAmm,
							drift.AssetType_Base,
							baseSwapped,
							drift.SwapDirection_Add,
						)
						quoteSwapped = math.CalculateQuoteAssetAmountSwapped(
							utils.AbsX(utils.SubX(bidAmm.QuoteAssetReserve.BigInt(), afterSwapQuoteReserves)),
							bidAmm.PegMultiplier.BigInt(),
							drift.SwapDirection_Add,
						)

					}
					topOfBookBidSize = utils.AddX(topOfBookBidSize, baseSwapped)
					bidSize = utils.DivX(utils.SubX(openBids, topOfBookBidSize), utils.BN(numBaseOrders))
				} else {
					baseSwapped = bidSize
					afterSwapQuoteReserves, afterSwapBaseReserves = math.CalculateAmmReservesAfterSwap(
						bidAmm,
						drift.AssetType_Base,
						baseSwapped,
						drift.SwapDirection_Add,
					)
					quoteSwapped = math.CalculateQuoteAssetAmountSwapped(
						utils.AbsX(utils.SubX(bidAmm.QuoteAssetReserve.BigInt(), afterSwapQuoteReserves)),
						bidAmm.PegMultiplier.BigInt(),
						drift.SwapDirection_Add,
					)
				}
				price := utils.DivX(utils.MulX(quoteSwapped, constants.BASE_PRECISION), baseSwapped)

				bidAmm.BaseAssetReserve = utils.Uint128(afterSwapBaseReserves)
				bidAmm.QuoteAssetReserve = utils.Uint128(afterSwapQuoteReserves)
				yield(&types.L2Level{
					Price: price,
					Size:  baseSwapped,
					Sources: map[types.LiquiditySource]*big.Int{
						types.LiquiditySourceVamm: baseSwapped,
					},
				}, numBids)
				numBids++
			}
		})
	}

	topOfBookAskSize := utils.BN(0)
	askSize := utils.DivX(openAsks, utils.BN(numBaseOrders))
	askAmm := &drift.Amm{
		BaseAssetReserve:  utils.Uint128(askReserves.Base),
		QuoteAssetReserve: utils.Uint128(askReserves.Quote),
		SqrtK:             updatedAmm.SqrtK,
		PegMultiplier:     updatedAmm.PegMultiplier,
	}
	l2OrderBookGenerator.GetL2Asks = func() *common.Generator[*types.L2Level, int] {
		return common.NewGenerator(func(yield common.YieldFn[*types.L2Level, int]) {
			numAsks := 0
			for numAsks < numOrders && askSize.Cmp(constants.ZERO) > 0 {
				quoteSwapped := utils.BN(0)
				baseSwapped := utils.BN(0)
				afterSwapQuoteReserves := utils.BN(0)
				afterSwapBaseReserves := utils.BN(0)

				if len(topOfBookQuoteAmounts) > 0 && numAsks < len(topOfBookQuoteAmounts) {
					remainingBaseLiquidity := utils.SubX(utils.MulX(openAsks, utils.BN(-1)), topOfBookAskSize)
					quoteSwapped = topOfBookQuoteAmounts[numAsks]
					afterSwapQuoteReserves, afterSwapBaseReserves = math.CalculateAmmReservesAfterSwap(
						askAmm,
						drift.AssetType_Quote,
						quoteSwapped,
						drift.SwapDirection_Add,
					)

					baseSwapped = utils.AbsX(utils.SubX(askAmm.BaseAssetReserve.BigInt(), afterSwapBaseReserves))
					if baseSwapped.Cmp(constants.ZERO) == 0 {
						return
					}
					if remainingBaseLiquidity.Cmp(baseSwapped) < 0 {
						baseSwapped = remainingBaseLiquidity
						afterSwapQuoteReserves, afterSwapBaseReserves = math.CalculateAmmReservesAfterSwap(
							askAmm,
							drift.AssetType_Base,
							baseSwapped,
							drift.SwapDirection_Remove,
						)
						quoteSwapped = math.CalculateQuoteAssetAmountSwapped(
							utils.AbsX(utils.SubX(askAmm.QuoteAssetReserve.BigInt(), afterSwapQuoteReserves)),
							askAmm.PegMultiplier.BigInt(),
							drift.SwapDirection_Remove,
						)

					}
					topOfBookAskSize = utils.AddX(topOfBookAskSize, baseSwapped)
					askSize = utils.DivX(utils.SubX(utils.AbsX(openAsks), topOfBookAskSize), utils.BN(numBaseOrders))
				} else {
					baseSwapped = askSize
					afterSwapQuoteReserves, afterSwapBaseReserves = math.CalculateAmmReservesAfterSwap(
						askAmm,
						drift.AssetType_Base,
						baseSwapped,
						drift.SwapDirection_Remove,
					)
					quoteSwapped = math.CalculateQuoteAssetAmountSwapped(
						utils.AbsX(utils.SubX(askAmm.QuoteAssetReserve.BigInt(), afterSwapQuoteReserves)),
						askAmm.PegMultiplier.BigInt(),
						drift.SwapDirection_Remove,
					)
				}
				price := utils.DivX(utils.MulX(quoteSwapped, constants.BASE_PRECISION), baseSwapped)

				askAmm.BaseAssetReserve = utils.Uint128(afterSwapBaseReserves)
				askAmm.QuoteAssetReserve = utils.Uint128(afterSwapQuoteReserves)
				yield(&types.L2Level{
					Price: price,
					Size:  baseSwapped,
					Sources: map[types.LiquiditySource]*big.Int{
						types.LiquiditySourceVamm: baseSwapped,
					},
				}, numAsks)
				numAsks++
			}
		})
	}
	return &l2OrderBookGenerator
}

func GroupL2(
	l2 *types.L2OrderBook,
	grouping *big.Int,
	depth int,
) *types.L2OrderBook {
	return &types.L2OrderBook{
		Bids: GroupL2Levels(l2.Bids, grouping, drift.PositionDirection_Long, depth),
		Asks: GroupL2Levels(l2.Asks, grouping, drift.PositionDirection_Short, depth),
		Slot: l2.Slot,
	}
}

func CloneL2Level(level *types.L2Level) *types.L2Level {
	if level == nil {
		return level
	}
	return &types.L2Level{
		Price:   &(*level.Price),
		Size:    &(*level.Size),
		Sources: level.Sources,
	}
}

func GroupL2Levels(
	levels []*types.L2Level,
	grouping *big.Int,
	direction drift.PositionDirection,
	depth int,
) []*types.L2Level {
	var groupedLevels []*types.L2Level
	for _, level := range levels {
		price := math.StandardizePrice(level.Price, grouping, direction)
		size := level.Size
		if len(groupedLevels) > 0 &&
			groupedLevels[len(groupedLevels)-1].Price.Cmp(price) == 0 {
			// Clones things so we don't mutate the original
			currentLevel := CloneL2Level(groupedLevels[len(groupedLevels)-1])

			currentLevel.Size = utils.AddX(currentLevel.Size, size)
			for source, size1 := range level.Sources {
				_, exists := currentLevel.Sources[source]
				if exists {
					currentLevel.Sources[source] = utils.AddX(currentLevel.Sources[source], size1)
				} else {
					currentLevel.Sources[source] = size1
				}
			}
		} else {
			groupedLevel := &types.L2Level{
				Price:   price,
				Size:    size,
				Sources: level.Sources,
			}
			groupedLevels = append(groupedLevels, groupedLevel)
		}
		if len(groupedLevels) == depth {
			break
		}
	}
	return groupedLevels
}

var mergeByPrice = func(bidsOrAsks []*types.L2Level) []*types.L2Level {
	merged := make(map[string]*types.L2Level)
	for _, level := range bidsOrAsks {
		key := level.Price.String()
		existing, exists := merged[key]
		if exists {
			existing.Size = utils.AddX(existing.Size, level.Size)
			for source, size := range level.Sources {
				_, exists := existing.Sources[source]
				if exists {
					existing.Sources[source] = utils.AddX(existing.Sources[source], size)
				} else {
					existing.Sources[source] = size
				}
			}
		} else {
			merged[key] = CloneL2Level(level)
		}
	}
	return utils.MapValues(merged)
}

func UncrossL2(
	bids []*types.L2Level,
	asks []*types.L2Level,
	oraclePrice *big.Int,
	oracleTwap5Min *big.Int,
	markTwap5Min *big.Int,
	grouping *big.Int,
	userBids map[string]bool,
	userAsks map[string]bool,
) ([]*types.L2Level, []*types.L2Level) {
	// If there are no bids or asks, there is nothing to center
	if len(bids) == 0 || len(asks) == 0 {
		return bids, asks
	}

	// If the top of the book is already centered, there is nothing to do
	if bids[0].Price.Cmp(asks[0].Price) < 0 {
		return bids, asks
	}

	var newBids []*types.L2Level
	var newAsks []*types.L2Level

	updateLevels := func(newPrice *big.Int, oldLevel *types.L2Level, levels []*types.L2Level) {
		if len(levels) > 0 && levels[len(levels)-1].Price.Cmp(newPrice) == 0 {
			levels[len(levels)-1].Size = utils.AddX(levels[len(levels)-1].Size, oldLevel.Size)
			for source, size := range oldLevel.Sources {
				_, exists := levels[len(levels)-1].Sources[source]
				if exists {
					levels[len(levels)-1].Sources[source] = utils.AddX(levels[len(levels)-1].Sources[source], size)
				} else {
					levels[len(levels)-1].Sources[source] = size
				}
			}
		} else {
			levels = append(levels, &types.L2Level{
				Price:   newPrice,
				Size:    oldLevel.Size,
				Sources: oldLevel.Sources,
			})
		}
	}

	// This is the best estimate of the premium in the market vs oracle to filter crossing around
	referencePrice := utils.AddX(oraclePrice, utils.SubX(markTwap5Min, oracleTwap5Min))

	bidIndex := 0
	askIndex := 0
	var maxBid *big.Int
	var minAsk *big.Int

	getPriceAndSetBound := func(newPrice *big.Int, direction drift.PositionDirection) *big.Int {
		if direction == drift.PositionDirection_Long {
			maxBid = utils.TTM[*big.Int](maxBid != nil, func() *big.Int { return utils.Min(maxBid, newPrice) }, newPrice)
			return maxBid
		} else {
			minAsk = utils.TTM[*big.Int](minAsk != nil, func() *big.Int { return utils.Max(minAsk, newPrice) }, newPrice)
			return minAsk
		}
	}

	for bidIndex < len(bids) || askIndex < len(asks) {
		nextBid := CloneL2Level(bids[bidIndex])
		nextAsk := CloneL2Level(asks[askIndex])

		if nextBid == nil {
			newAsks = append(newAsks, nextAsk)
			askIndex++
			continue
		}

		if nextAsk == nil {
			newBids = append(newBids, nextBid)
			bidIndex++
			continue
		}

		_, exists := userBids[nextBid.Price.String()]
		if exists {
			newBids = append(newBids, nextBid)
			bidIndex++
			continue
		}

		_, exists = userAsks[nextAsk.Price.String()]
		if exists {
			newAsks = append(newAsks, nextAsk)
			askIndex++
			continue
		}

		if nextBid.Price.Cmp(nextAsk.Price) >= 0 {
			if nextBid.Price.Cmp(referencePrice) > 0 && nextAsk.Price.Cmp(referencePrice) > 0 {
				newBidPrice := utils.SubX(nextAsk.Price, grouping)
				newBidPrice = getPriceAndSetBound(newBidPrice, drift.PositionDirection_Long)
				updateLevels(newBidPrice, nextBid, newBids)
				bidIndex++
			} else if nextAsk.Price.Cmp(referencePrice) < 0 && nextBid.Price.Cmp(referencePrice) < 0 {
				newAskPrice := utils.AddX(nextBid.Price, grouping)
				newAskPrice = getPriceAndSetBound(newAskPrice, drift.PositionDirection_Short)
				updateLevels(newAskPrice, nextAsk, newAsks)
				askIndex++
			} else {
				newBidPrice := utils.SubX(referencePrice, grouping)
				newAskPrice := utils.AddX(referencePrice, grouping)
				newBidPrice = getPriceAndSetBound(newBidPrice, drift.PositionDirection_Long)
				newAskPrice = getPriceAndSetBound(newAskPrice, drift.PositionDirection_Short)
				updateLevels(newBidPrice, nextBid, newBids)
				updateLevels(newAskPrice, nextAsk, newAsks)
				bidIndex++
				askIndex++
			}
		} else {
			if minAsk != nil && nextAsk.Price.Cmp(minAsk) <= 0 {
				newAskPrice := getPriceAndSetBound(
					nextAsk.Price,
					drift.PositionDirection_Short,
				)
				updateLevels(newAskPrice, nextAsk, newAsks)
			} else {
				newAsks = append(newAsks, nextAsk)
			}
			askIndex++

			if maxBid != nil && nextBid.Price.Cmp(maxBid) >= 0 {
				newBidPrice := getPriceAndSetBound(
					nextBid.Price,
					drift.PositionDirection_Long,
				)
				updateLevels(newBidPrice, nextBid, newBids)
			} else {
				newBids = append(newBids, nextBid)
			}
			bidIndex++

		}

	}
	slices.SortFunc(newBids, func(a *types.L2Level, b *types.L2Level) int {
		return b.Price.Cmp(a.Price)
	})
	slices.SortFunc(newAsks, func(a *types.L2Level, b *types.L2Level) int {
		return a.Price.Cmp(b.Price)
	})

	finalNewBids := mergeByPrice(newBids)
	finalNewAsks := mergeByPrice(newAsks)
	return finalNewBids, finalNewAsks
}
