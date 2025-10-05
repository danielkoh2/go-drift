package drift

import (
	drift "driftgo"
	drift2 "driftgo/lib/drift"
	"driftgo/utils"
	"reflect"
)

func ApplyOrderParams[T drift2.OrderParams | drift.OptionalOrderParams, V drift2.OrderParams | drift.OptionalOrderParams](dest *T, override *V) {
	orderParamsType := reflect.TypeOf((*drift2.OrderParams)(nil))
	optionOrderParamsType := reflect.TypeOf((*drift.OptionalOrderParams)(nil))
	paramType := reflect.TypeOf(dest)
	overrideType := reflect.TypeOf(override)
	if paramType == orderParamsType && overrideType == orderParamsType {
		nDest := (reflect.ValueOf(dest).Interface()).(*drift2.OrderParams)
		nOverride := (reflect.ValueOf(override).Interface()).(*drift2.OrderParams)
		nDest.OrderType = nOverride.OrderType
		nDest.MarketType = nOverride.MarketType
		nDest.Direction = nOverride.Direction
		nDest.UserOrderId = nOverride.UserOrderId
		nDest.BaseAssetAmount = nOverride.BaseAssetAmount
		nDest.Price = nOverride.Price
		nDest.MarketIndex = nOverride.MarketIndex
		nDest.ReduceOnly = nOverride.ReduceOnly
		nDest.PostOnly = nOverride.PostOnly
		//nDest.ImmediateOrCancel = nOverride.ImmediateOrCancel
		if nOverride.MaxTs != nil {
			nDest.MaxTs = utils.NewPtrp(nOverride.MaxTs)
		}
		if nOverride.TriggerPrice != nil {
			nDest.TriggerPrice = utils.NewPtrp(nOverride.TriggerPrice)
		}
		nDest.TriggerCondition = nOverride.TriggerCondition
		if nOverride.OraclePriceOffset != nil {
			nDest.OraclePriceOffset = utils.NewPtrp(nOverride.OraclePriceOffset)
		}
		if nOverride.AuctionDuration != nil {
			nDest.AuctionDuration = utils.NewPtrp(nOverride.AuctionDuration)
		}
		if nOverride.AuctionStartPrice != nil {
			nDest.AuctionStartPrice = utils.NewPtrp(nOverride.AuctionStartPrice)
		}
		if nOverride.AuctionEndPrice != nil {
			nDest.AuctionEndPrice = utils.NewPtrp(nOverride.AuctionEndPrice)
		}
	} else if paramType == optionOrderParamsType && overrideType == orderParamsType {
		nDest := (reflect.ValueOf(dest).Interface()).(*drift.OptionalOrderParams)
		nOverride := (reflect.ValueOf(override).Interface()).(*drift2.OrderParams)
		nDest.OrderType = utils.NewPtr(nOverride.OrderType)
		nDest.MarketType = utils.NewPtr(nOverride.MarketType)
		nDest.Direction = utils.NewPtr(nOverride.Direction)
		nDest.UserOrderId = utils.NewPtr(nOverride.UserOrderId)
		nDest.BaseAssetAmount = utils.NewPtr(nOverride.BaseAssetAmount)
		nDest.Price = utils.NewPtr(nOverride.Price)
		nDest.MarketIndex = utils.NewPtr(nOverride.MarketIndex)
		nDest.ReduceOnly = utils.NewPtr(nOverride.ReduceOnly)
		nDest.PostOnly = utils.NewPtr(nOverride.PostOnly)
		//nDest.ImmediateOrCancel = utils.NewPtr(nOverride.ImmediateOrCancel)
		if nOverride.MaxTs != nil {
			nDest.MaxTs = utils.NewPtrp(nOverride.MaxTs)
		}
		if nOverride.TriggerPrice != nil {
			nDest.TriggerPrice = utils.NewPtrp(nOverride.TriggerPrice)
		}
		nDest.TriggerCondition = utils.NewPtr(nOverride.TriggerCondition)
		if nOverride.OraclePriceOffset != nil {
			nDest.OraclePriceOffset = utils.NewPtrp(nOverride.OraclePriceOffset)
		}
		if nOverride.AuctionDuration != nil {
			nDest.AuctionDuration = utils.NewPtrp(nOverride.AuctionDuration)
		}
		if nOverride.AuctionStartPrice != nil {
			nDest.AuctionStartPrice = utils.NewPtrp(nOverride.AuctionStartPrice)
		}
		if nOverride.AuctionEndPrice != nil {
			nDest.AuctionEndPrice = utils.NewPtrp(nOverride.AuctionEndPrice)
		}
	} else if paramType == orderParamsType && overrideType == optionOrderParamsType {
		nDest := (reflect.ValueOf(dest).Interface()).(*drift2.OrderParams)
		nOverride := (reflect.ValueOf(override).Interface()).(*drift.OptionalOrderParams)
		if nOverride.OrderType != nil {
			nDest.OrderType = *nOverride.OrderType
		}
		if nOverride.MarketType != nil {
			nDest.MarketType = *nOverride.MarketType
		}
		if nOverride.Direction != nil {
			nDest.Direction = *nOverride.Direction
		}
		if nOverride.UserOrderId != nil {
			nDest.UserOrderId = *nOverride.UserOrderId
		}
		if nOverride.BaseAssetAmount != nil {
			nDest.BaseAssetAmount = *nOverride.BaseAssetAmount
		}
		if nOverride.Price != nil {
			nDest.Price = *nOverride.Price
		}
		if nOverride.MarketIndex != nil {
			nDest.MarketIndex = *nOverride.MarketIndex
		}
		if nOverride.ReduceOnly != nil {
			nDest.ReduceOnly = *nOverride.ReduceOnly
		}
		if nOverride.PostOnly != nil {
			nDest.PostOnly = *nOverride.PostOnly
		}
		if nOverride.ImmediateOrCancel != nil {
			//nDest.ImmediateOrCancel = *nOverride.ImmediateOrCancel
		}
		if nOverride.MaxTs != nil {
			nDest.MaxTs = utils.NewPtrp(nOverride.MaxTs)
		}
		if nOverride.TriggerPrice != nil {
			nDest.TriggerPrice = utils.NewPtrp(nOverride.TriggerPrice)
		}
		if nOverride.TriggerCondition != nil {
			nDest.TriggerCondition = *nOverride.TriggerCondition
		}
		if nOverride.OraclePriceOffset != nil {
			nDest.OraclePriceOffset = utils.NewPtrp(nOverride.OraclePriceOffset)
		}
		if nOverride.AuctionDuration != nil {
			nDest.AuctionDuration = utils.NewPtrp(nOverride.AuctionDuration)
		}
		if nOverride.AuctionStartPrice != nil {
			nDest.AuctionStartPrice = utils.NewPtrp(nOverride.AuctionStartPrice)
		}
		if nOverride.AuctionEndPrice != nil {
			nDest.AuctionEndPrice = utils.NewPtrp(nOverride.AuctionEndPrice)
		}
	} else if paramType == optionOrderParamsType && overrideType == optionOrderParamsType {
		nDest := (reflect.ValueOf(dest).Interface()).(*drift.OptionalOrderParams)
		nOverride := (reflect.ValueOf(override).Interface()).(*drift.OptionalOrderParams)
		if nOverride.OrderType != nil {
			nDest.OrderType = utils.NewPtrp(nOverride.OrderType)
		}
		if nOverride.MarketType != nil {
			nDest.MarketType = utils.NewPtrp(nOverride.MarketType)
		}
		if nOverride.Direction != nil {
			nDest.Direction = utils.NewPtrp(nOverride.Direction)
		}
		if nOverride.UserOrderId != nil {
			nDest.UserOrderId = utils.NewPtrp(nOverride.UserOrderId)
		}
		if nOverride.BaseAssetAmount != nil {
			nDest.BaseAssetAmount = utils.NewPtrp(nOverride.BaseAssetAmount)
		}
		if nOverride.Price != nil {
			nDest.Price = utils.NewPtrp(nOverride.Price)
		}
		if nOverride.MarketIndex != nil {
			nDest.MarketIndex = utils.NewPtrp(nOverride.MarketIndex)
		}
		if nOverride.ReduceOnly != nil {
			nDest.ReduceOnly = utils.NewPtrp(nOverride.ReduceOnly)
		}
		if nOverride.PostOnly != nil {
			nDest.PostOnly = utils.NewPtrp(nOverride.PostOnly)
		}
		if nOverride.ImmediateOrCancel != nil {
			nDest.ImmediateOrCancel = utils.NewPtrp(nOverride.ImmediateOrCancel)
		}
		if nOverride.MaxTs != nil {
			nDest.MaxTs = utils.NewPtrp(nOverride.MaxTs)
		}
		if nOverride.TriggerPrice != nil {
			nDest.TriggerPrice = utils.NewPtrp(nOverride.TriggerPrice)
		}
		if nOverride.TriggerCondition != nil {
			nDest.TriggerCondition = utils.NewPtrp(nOverride.TriggerCondition)
		}
		if nOverride.OraclePriceOffset != nil {
			nDest.OraclePriceOffset = utils.NewPtrp(nOverride.OraclePriceOffset)
		}
		if nOverride.AuctionDuration != nil {
			nDest.AuctionDuration = utils.NewPtrp(nOverride.AuctionDuration)
		}
		if nOverride.AuctionStartPrice != nil {
			nDest.AuctionStartPrice = utils.NewPtrp(nOverride.AuctionStartPrice)
		}
		if nOverride.AuctionEndPrice != nil {
			nDest.AuctionEndPrice = utils.NewPtrp(nOverride.AuctionEndPrice)
		}
	}
}

func GetLimitOrderParams(
	orderParams drift.OptionalOrderParams,
	price uint64,
) drift.OptionalOrderParams {
	var limitOrderParams drift.OptionalOrderParams
	ApplyOrderParams(&limitOrderParams, &orderParams)
	limitOrderParams.OrderType = utils.NewPtr(drift2.OrderType_Limit)
	limitOrderParams.Price = utils.NewPtr(price)
	return limitOrderParams
}

func GetTriggerMarketOrderParams(
	orderParams drift.OptionalOrderParams,
	triggerCondition drift2.OrderTriggerCondition,
	triggerPrice uint64,
) drift.OptionalOrderParams {
	var triggerOrderParams drift.OptionalOrderParams
	ApplyOrderParams(&triggerOrderParams, &orderParams)
	triggerOrderParams.OrderType = utils.NewPtr(drift2.OrderType_TriggerMarket)
	triggerOrderParams.TriggerPrice = &triggerPrice
	triggerOrderParams.TriggerCondition = utils.NewPtr(triggerCondition)
	return triggerOrderParams
}

func GetTriggerLimitOrderParams(
	orderParams drift.OptionalOrderParams,
	triggerCondition drift2.OrderTriggerCondition,
	triggerPrice uint64,
	price uint64,
) drift.OptionalOrderParams {
	var triggerLimitOrderParams drift.OptionalOrderParams
	ApplyOrderParams(&triggerLimitOrderParams, &orderParams)
	triggerLimitOrderParams.OrderType = utils.NewPtr(drift2.OrderType_TriggerLimit)
	triggerLimitOrderParams.Price = utils.NewPtr(price)
	triggerLimitOrderParams.TriggerPrice = &triggerPrice
	triggerLimitOrderParams.TriggerCondition = utils.NewPtr(triggerCondition)
	return triggerLimitOrderParams
}

func GetMarketOrderParams(
	orderParams drift.OptionalOrderParams,
) drift.OptionalOrderParams {
	var marketOrderParams drift.OptionalOrderParams
	ApplyOrderParams(&marketOrderParams, &orderParams)
	marketOrderParams.OrderType = utils.NewPtr(drift2.OrderType_Market)
	return marketOrderParams
}

func GetOrderParams(
	optionalOrderParams drift.OptionalOrderParams,
	overridingParams *drift.OptionalOrderParams,
) drift2.OrderParams {
	var orderParams drift2.OrderParams = drift.DefaultOrderParams
	//ApplyOrderParams(&orderParams, &drift.DefaultOrderParams)
	ApplyOrderParams(&orderParams, &optionalOrderParams)
	if overridingParams != nil {
		ApplyOrderParams(&orderParams, overridingParams)
	}
	return orderParams
}

func ToOptionalOrderParams(orderParams *drift2.OrderParams) *drift.OptionalOrderParams {
	return &drift.OptionalOrderParams{
		OrderType:       utils.NewPtr(orderParams.OrderType),
		MarketType:      utils.NewPtr(orderParams.MarketType),
		Direction:       utils.NewPtr(orderParams.Direction),
		UserOrderId:     utils.NewPtr(orderParams.UserOrderId),
		BaseAssetAmount: utils.NewPtr(orderParams.BaseAssetAmount),
		Price:           utils.NewPtr(orderParams.Price),
		MarketIndex:     utils.NewPtr(orderParams.MarketIndex),
		ReduceOnly:      utils.NewPtr(orderParams.ReduceOnly),
		PostOnly:        utils.NewPtr(orderParams.PostOnly),
		//ImmediateOrCancel: utils.NewPtr(orderParams.ImmediateOrCancel),
		MaxTs:             utils.NewPtrp(orderParams.MaxTs),
		TriggerPrice:      utils.NewPtrp(orderParams.TriggerPrice),
		TriggerCondition:  utils.NewPtr(orderParams.TriggerCondition),
		OraclePriceOffset: utils.NewPtrp(orderParams.OraclePriceOffset),
		AuctionDuration:   utils.NewPtrp(orderParams.AuctionDuration),
		AuctionStartPrice: utils.NewPtrp(orderParams.AuctionStartPrice),
		AuctionEndPrice:   utils.NewPtrp(orderParams.AuctionEndPrice),
	}
}
