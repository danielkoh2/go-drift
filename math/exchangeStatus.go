package math

import (
	"driftgo/constants"
	"driftgo/lib/drift"
	"driftgo/types"
	"driftgo/utils"
)

func ExchangePaused(state *drift.State) bool {
	return state.ExchangeStatus != 0
}

func FillPaused(
	state *drift.State,
	market *types.MarketAccount,
) bool {
	if state.ExchangeStatus&uint8(drift.ExchangeStatus_FillPaused) == uint8(drift.ExchangeStatus_FillPaused) {
		return true
	}
	if market.PerpMarketAccount != nil {
		return IsOperationPaused(market.PerpMarketAccount.PausedOperations, uint8(drift.PerpOperation_Fill))
	} else {
		return IsOperationPaused(market.SpotMarketAccount.PausedOperations, uint8(drift.SpotOperation_Fill))
	}
}

func AmmPaused(state *drift.State, market *types.MarketAccount) bool {
	if state.ExchangeStatus&uint8(drift.ExchangeStatus_AmmPaused) == uint8(drift.ExchangeStatus_AmmPaused) {
		return true
	}
	if market.PerpMarketAccount != nil {
		operationPaused := IsOperationPaused(
			market.PerpMarketAccount.PausedOperations,
			uint8(drift.PerpOperation_AmmFill),
		)
		if operationPaused {
			return true
		}
		if isAmmDrawdownPause(market.PerpMarketAccount) {
			return true
		}
	}
	return false
}

func IsOperationPaused(pasuedOperations uint8, operation uint8) bool {
	return pasuedOperations&operation > 0
}

func isAmmDrawdownPause(market *drift.PerpMarket) bool {
	var quoteDrawdownLimitBreached bool

	if market.ContractTier == drift.ContractTier_A ||
		market.ContractTier == drift.ContractTier_B {
		quoteDrawdownLimitBreached = market.Amm.NetRevenueSinceLastFunding <=
			constants.DEFAULT_REVENUE_SINCE_LAST_FUNDING_SPREAD_RETREAT.Int64()*400
	} else {
		quoteDrawdownLimitBreached = market.Amm.NetRevenueSinceLastFunding <=
			constants.DEFAULT_REVENUE_SINCE_LAST_FUNDING_SPREAD_RETREAT.Int64()*200
	}

	if quoteDrawdownLimitBreached {
		percentDrawdown := utils.DivX(
			utils.MulX(
				utils.BN(market.Amm.NetRevenueSinceLastFunding),
				constants.PERCENTAGE_PRECISION,
			),
			utils.Max(market.Amm.TotalFeeMinusDistributions.BigInt(), utils.BN(1)),
		)

		var percentDrawdownLimitBreached bool

		if market.ContractTier == drift.ContractTier_A {
			percentDrawdownLimitBreached = percentDrawdown.Cmp(
				utils.NegX(
					utils.DivX(
						constants.PERCENTAGE_PRECISION,
						utils.BN(50),
					),
				),
			) <= 0
		} else if market.ContractTier == drift.ContractTier_B {
			percentDrawdownLimitBreached = percentDrawdown.Cmp(
				utils.NegX(
					utils.DivX(
						constants.PERCENTAGE_PRECISION,
						utils.BN(33),
					),
				),
			) <= 0
		} else if market.ContractTier == drift.ContractTier_C {
			percentDrawdownLimitBreached = percentDrawdown.Cmp(
				utils.NegX(
					utils.DivX(
						constants.PERCENTAGE_PRECISION,
						utils.BN(25),
					),
				),
			) <= 0
		} else {
			percentDrawdownLimitBreached = percentDrawdown.Cmp(
				utils.NegX(
					utils.DivX(
						constants.PERCENTAGE_PRECISION,
						utils.BN(20),
					),
				),
			) <= 0
		}

		if percentDrawdownLimitBreached {
			return true
		}
	}
	return false
}
