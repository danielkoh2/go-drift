package math

import (
	"driftgo/constants"
	"driftgo/drift/types"
	"driftgo/lib/drift"
	oracles "driftgo/oracles/types"
	"driftgo/utils"
	"fmt"
	"math/big"
	"time"
)

func IsOrderRiskIncreasing(user types.IUser, order *drift.Order) bool {
	if order.Status == drift.OrderStatus_Init {
		return false
	}

	position := user.GetPerpPosition(order.MarketIndex)
	if position == nil {
		position = user.GetEmptyPosition(order.MarketIndex)
	}

	// if no position exists, it's risk increasing
	if position.BaseAssetAmount == 0 {
		return true
	}

	// if position is long and order is long
	if position.BaseAssetAmount > 0 && order.Direction == drift.PositionDirection_Long {
		return true
	}

	// if position is short and order is short
	if position.BaseAssetAmount < 0 && order.Direction == drift.PositionDirection_Short {
		return true
	}

	baseAssetAmountToFill := utils.BN(order.BaseAssetAmount - order.BaseAssetAmountFilled)
	// if order will flip position
	if baseAssetAmountToFill.Cmp(utils.AbsX(utils.BN(position.BaseAssetAmount*2))) > 0 {
		return true
	}

	return false
}

func IsOrderRiskIncreasingInSameDirection(user types.IUser, order *drift.Order) bool {
	if order.Status == drift.OrderStatus_Init {
		return false
	}

	position := user.GetPerpPosition(order.MarketIndex)
	if position == nil {
		position = user.GetEmptyPosition(order.MarketIndex)
	}

	// if no position exists, it's risk increasing
	if position.BaseAssetAmount == 0 {
		return true
	}

	// if position is long and order is long
	if position.BaseAssetAmount > 0 && order.Direction == drift.PositionDirection_Long {
		return true
	}

	// if position is short and order is short
	if position.BaseAssetAmount < 0 && order.Direction == drift.PositionDirection_Short {
		return true
	}

	return false
}

func IsOrderReduceOnly(user types.IUser, order *drift.Order) bool {
	if order.Status == drift.OrderStatus_Init {
		return false
	}

	position := user.GetPerpPosition(order.MarketIndex)
	if position == nil {
		position = user.GetEmptyPosition(order.MarketIndex)
	}

	// if position is long and order is long
	if position.BaseAssetAmount >= 0 && order.Direction == drift.PositionDirection_Long {
		return false
	}

	// if position is short and order is short
	if position.BaseAssetAmount <= 0 && order.Direction == drift.PositionDirection_Short {
		return false
	}

	return true
}

func StandardizeBaseAssetAmount(
	baseAssetAmount *big.Int,
	stepSize *big.Int,
) *big.Int {
	remainder := utils.ModX(baseAssetAmount, stepSize)
	return utils.SubX(baseAssetAmount, remainder)
}

func StandardizePrice(
	price *big.Int,
	tickSize *big.Int,
	direction drift.PositionDirection,
) *big.Int {
	if price.Cmp(constants.ZERO) == 0 {
		fmt.Println("price is zero")
		return price
	}

	remainder := utils.ModX(price, tickSize)
	if remainder.Cmp(constants.ZERO) == 0 {
		return price
	}

	if direction == drift.PositionDirection_Long {
		return utils.SubX(price, remainder)
	} else {
		return utils.SubX(utils.AddX(price, tickSize), remainder)
	}
}

func GetLimitPrice(
	order *drift.Order,
	oraclePriceData *oracles.OraclePriceData,
	slot uint64,
	fallbackPrice *big.Int,
) *big.Int {
	var limitPrice *big.Int
	if HasAuctionPrice(order, slot) {
		limitPrice = GetAuctionPrice(order, slot, oraclePriceData.Price)
	} else if order.OraclePriceOffset != 0 {
		limitPrice = utils.AddX(oraclePriceData.Price, utils.BN(order.OraclePriceOffset))
	} else if order.Price == 0 {
		limitPrice = fallbackPrice
	} else {
		limitPrice = utils.BN(order.Price)
	}

	return limitPrice
}

func HasLimitPrice(
	order *drift.Order,
	slot uint64,
) bool {
	return order.Price > 0 ||
		order.OraclePriceOffset != 0 ||
		!IsAuctionComplete(order, slot)
}

func HasAuctionPrice(
	order *drift.Order,
	slot uint64,
) bool {
	return !IsAuctionComplete(order, slot) &&
		(order.AuctionStartPrice != 0 || order.AuctionEndPrice != 0)
}

func IsFillableByVAMM(
	order *drift.Order,
	market *drift.PerpMarket,
	oraclePriceData *oracles.OraclePriceData,
	slot uint64,
	ts int64,
	minAuctionDuration int,
) bool {
	if market == nil {
		return false
	}
	return (IsFallbackAvailableLiquiditySource(order, minAuctionDuration, slot) &&
		CalculateBaseAssetAmountForAmmToFulfill(order, market, oraclePriceData, slot).Cmp(utils.BN(market.Amm.MinOrderSize)) >= 0) ||
		IsOrderExpired(order, ts, false)
}

func CalculateBaseAssetAmountForAmmToFulfill(
	order *drift.Order,
	market *drift.PerpMarket,
	oraclePriceData *oracles.OraclePriceData,
	slot uint64,
) *big.Int {
	if MustBeTriggered(order) && !IsTriggered(order) {
		return utils.BN(0)
	}
	limitPrice := GetLimitPrice(order, oraclePriceData, slot, nil)

	var baseAssetAmount *big.Int
	updatedAMM := CalculateUpdatedAMM(&market.Amm, oraclePriceData)
	if limitPrice != nil && limitPrice.Cmp(constants.ZERO) > 0 {
		baseAssetAmount = CalculateBaseAssetAmountToFillUpToLimitPrice(order, updatedAMM, limitPrice, oraclePriceData)
	} else {
		baseAssetAmount = utils.BN(order.BaseAssetAmount - order.BaseAssetAmountFilled)
	}

	maxBaseAssetAmount := CalculateMaxBaseAssetAmountFillable(updatedAMM, order.Direction)
	//if baseAssetAmount.Cmp(maxBaseAssetAmount) > 0 {
	//	baseAssetAmount = maxBaseAssetAmount
	//}
	return utils.Min(maxBaseAssetAmount, baseAssetAmount)
}

func CalculateBaseAssetAmountToFillUpToLimitPrice(
	order *drift.Order,
	amm *drift.Amm,
	limitPrice *big.Int,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	var adjustedLimitPrice *big.Int
	if order.Direction == drift.PositionDirection_Long {
		adjustedLimitPrice = utils.SubX(limitPrice, utils.BN(amm.OrderTickSize))
	} else {
		adjustedLimitPrice = utils.AddX(limitPrice, utils.BN(amm.OrderTickSize))
	}
	if adjustedLimitPrice.Cmp(constants.ZERO) <= 0 {
		//spew.Dump(order)
		//fmt.Println("limitPrice=", limitPrice.String())
		return utils.BN(0)
	}
	maxAmountToTrade, direction := CalculateMaxBaseAssetAmountToTrade(
		amm,
		adjustedLimitPrice,
		order.Direction,
		oraclePriceData,
		time.Now().Unix(),
	)

	baseAssetAmount := StandardizeBaseAssetAmount(maxAmountToTrade, utils.BN(amm.OrderStepSize))

	// Check that directions are the same
	sameDirection := direction == order.Direction
	if !sameDirection {
		return utils.BN(0)
	}

	baseAssetAmountUnfilled := order.BaseAssetAmount - order.BaseAssetAmountFilled
	if baseAssetAmount.Cmp(utils.BN(baseAssetAmountUnfilled)) > 0 {
		return utils.BN(baseAssetAmountUnfilled)
	} else {
		return baseAssetAmount
	}
}

// line 282 OK
func IsOrderExpired(
	order *drift.Order,
	ts int64,
	enforceBuffer bool,
) bool {
	if MustBeTriggered(order) || order.Status != drift.OrderStatus_Open || order.MaxTs == 0 {
		return false
	}

	maxTs := order.MaxTs
	if enforceBuffer && IsLimitOrder(order) {
		maxTs += 15
	}

	return ts > maxTs
}

// line 294 OK
func IsMarketOrder(order *drift.Order) bool {
	//isOneOfVariant(order.orderType, ['market', 'triggerMarket', 'oracle']);
	if order.OrderType == drift.OrderType_Market || order.OrderType == drift.OrderType_TriggerMarket || order.OrderType == drift.OrderType_Oracle {
		return true
	}
	return false
}

// line 298 OK
func IsLimitOrder(order *drift.Order) bool {
	return order.OrderType == drift.OrderType_Limit || order.OrderType == drift.OrderType_TriggerLimit
}

// line 302 OK
func MustBeTriggered(order *drift.Order) bool {
	return order.OrderType == drift.OrderType_TriggerMarket || order.OrderType == drift.OrderType_TriggerLimit
}

// line 306 OK
func IsTriggered(order *drift.Order) bool {
	return order.TriggerCondition == drift.OrderTriggerCondition_TriggeredAbove || order.TriggerCondition == drift.OrderTriggerCondition_TriggeredBelow
}

// line 313 OK
// https://github.com/drift-labs/protocol-v2/commit/dd5607fe44fa84503bd826558dec9d515479f9c9
func IsRestingLimitOrder(order *drift.Order, slot uint64) bool {
	//return IsLimitOrder(order) && (order.PostOnly || IsAuctionComplete(order, slot))
	if !IsLimitOrder(order) {
		return false
	}
	//if order.OrderType == drift.OrderType_TriggerLimit {
	//	if order.Direction == drift.PositionDirection_Long && order.TriggerPrice < order.Price {
	//		return false
	//	} else if order.Direction == drift.PositionDirection_Short && order.TriggerPrice > order.Price {
	//		return false
	//	}
	//	return IsAuctionComplete(order, slot)
	//}
	return order.PostOnly || IsAuctionComplete(order, slot)
}

func IsTakingOrder(order *drift.Order, slot uint64) bool {
	return IsMarketOrder(order) || !IsRestingLimitOrder(order, slot)
}
