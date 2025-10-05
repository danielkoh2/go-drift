package math

import (
	"driftgo/lib/drift"
	oracles "driftgo/oracles/types"
	"math/big"
)

func IsSpotPositionAvailable(position *drift.SpotPosition) bool {
	return position.ScaledBalance == 0 && position.OpenOrders == 0
}

type OrderFillSimulation struct {
	TokenAmount                *big.Int
	OrdersValue                *big.Int
	TokenValue                 *big.Int
	Weight                     *big.Int
	WeightedTokenValue         *big.Int
	FreeCollateralContribution *big.Int
}

func GetWorstCaseTokenAmounts(
	spotPosition *drift.SpotPosition,
	spotMarketAccount *drift.SpotMarket,
	strictOraclePrice *oracles.StrictOraclePrice,
	marginCategory drift.MarginRequirementType,
	customMarginRatio int64,
) *OrderFillSimulation {
	return nil
}
