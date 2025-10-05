package math

import (
	"driftgo/constants"
	"driftgo/lib/drift"
	"driftgo/utils"
	"math/big"
)

func IsAuctionComplete(order *drift.Order, slot uint64) bool {
	if order.AuctionDuration == 0 {
		return true
	}

	return slot-order.Slot > uint64(order.AuctionDuration)
}

func IsFallbackAvailableLiquiditySource(
	order *drift.Order,
	minAuctionDuration int,
	slot uint64,
) bool {
	if minAuctionDuration == 0 {
		return true
	}

	return slot-order.Slot > uint64(minAuctionDuration)
}

func GetAuctionPrice(
	order *drift.Order,
	slot uint64,
	oraclePrice *big.Int,
) *big.Int {
	switch order.OrderType {
	case drift.OrderType_Market, drift.OrderType_TriggerMarket, drift.OrderType_Limit, drift.OrderType_TriggerLimit:
		return GetAuctionPriceForFixedAuction(order, slot)
	case drift.OrderType_Oracle:
		return GetAuctionPriceForOracleOffsetAuction(order, slot, oraclePrice)
	default:
		//log.Error("Can't get auction price for order type %d", order.OrderType)
		return nil
		//default:
		//		panic("Cant get auction price for order type")
	}
}

func GetAuctionPriceForFixedAuction(order *drift.Order, slot uint64) *big.Int {
	slotsElapsed := uint64(slot) - order.Slot

	deltaDenominator := uint64(order.AuctionDuration)
	deltaNumerator := min(slotsElapsed, deltaDenominator)

	if deltaDenominator == 0 {
		return utils.BN(order.AuctionEndPrice)
	}

	var priceDelta *big.Int
	if order.Direction == drift.PositionDirection_Long {
		priceDelta = utils.DivX(
			utils.MulX(
				utils.BN(order.AuctionEndPrice-order.AuctionStartPrice),
				utils.BN(deltaNumerator),
			),
			utils.BN(deltaDenominator),
		)
	} else {
		priceDelta = utils.DivX(
			utils.MulX(
				utils.BN(order.AuctionStartPrice-order.AuctionEndPrice),
				utils.BN(deltaNumerator),
			),
			utils.BN(deltaDenominator),
		)
	}

	var price *big.Int
	if order.Direction == drift.PositionDirection_Long {
		price = utils.AddX(utils.BN(order.AuctionStartPrice), priceDelta)
	} else {
		price = utils.SubX(utils.BN(order.AuctionStartPrice), priceDelta)
	}

	return price
}

func GetAuctionPriceForOracleOffsetAuction(
	order *drift.Order,
	slot uint64,
	oraclePrice *big.Int,
) *big.Int {
	slotsElapsed := slot - order.Slot

	deltaDenominator := uint64(order.AuctionDuration)
	deltaNumerator := min(slotsElapsed, deltaDenominator)

	if deltaDenominator == 0 {
		return utils.AddX(oraclePrice, utils.BN(order.AuctionEndPrice))
	}

	var priceOffsetDelta *big.Int
	if order.Direction == drift.PositionDirection_Long {
		priceOffsetDelta = utils.DivX(
			utils.MulX(
				utils.BN(order.AuctionEndPrice-order.AuctionStartPrice),
				utils.BN(deltaNumerator),
			),
			utils.BN(deltaDenominator),
		)
	} else {
		priceOffsetDelta = utils.DivX(
			utils.MulX(
				utils.BN(order.AuctionStartPrice-order.AuctionEndPrice),
				utils.BN(deltaNumerator),
			),
			utils.BN(deltaDenominator),
		)
	}

	var priceOffset *big.Int
	if order.Direction == drift.PositionDirection_Long {
		priceOffset = utils.AddX(utils.BN(order.AuctionStartPrice), priceOffsetDelta)
	} else {
		priceOffset = utils.SubX(utils.BN(order.AuctionStartPrice), priceOffsetDelta)
	}

	return utils.AddX(oraclePrice, priceOffset)
}

func DeriveOracleAuctionParams(
	direction drift.PositionDirection,
	oraclePrice *big.Int,
	auctionStartPrice *big.Int,
	auctionEndPrice *big.Int,
	limitPrice *big.Int,
) (*big.Int, *big.Int, int64) {
	var oraclePriceOffset *big.Int

	if limitPrice.Cmp(constants.ZERO) == 0 || oraclePrice.Cmp(constants.ZERO) == 0 {
		oraclePriceOffset = utils.BN(0)
	} else {
		oraclePriceOffset = utils.SubX(limitPrice, oraclePrice)
	}

	if oraclePriceOffset.Cmp(constants.ZERO) == 0 {
		oraclePriceOffset = utils.TTM[*big.Int](
			direction == drift.PositionDirection_Long,
			func() *big.Int {
				return utils.AddX(utils.SubX(auctionEndPrice, oraclePrice), constants.ONE)
			},
			func() *big.Int {
				return utils.SubX(utils.SubX(auctionEndPrice, oraclePrice), constants.ONE)
			},
		)
	}

	oraclePriceOffsetNum := oraclePriceOffset.Int64()
	return utils.SubX(auctionStartPrice, oraclePrice),
		utils.SubX(auctionEndPrice, oraclePrice),
		oraclePriceOffsetNum
}
