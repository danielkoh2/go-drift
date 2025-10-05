package math

import "math/big"

type AssetReserve struct {
	Base  *big.Int
	Quote *big.Int
}

type SpreadTerms struct {
	longVolSpread            float64
	shortVolSpread           float64
	longSpreadwPS            float64
	shortSpreadwPS           float64
	maxTargetSpread          float64
	inventorySpreadScale     float64
	longSpreadwInvScale      float64
	shortSpreadwInvScale     float64
	effectiveLeverage        float64
	effectiveLeverageCapped  float64
	longSpreadwEL            float64
	shortSpreadwEL           float64
	revenueRetreatAmount     float64
	halfRevenueRetreatAmount float64
	longSpreadwRevRetreat    float64
	shortSpreadwRevRetreat   float64
	longSpreadwOffsetShrink  float64
	shortSpreadwOffsetShrink float64
	totalSpread              float64
	longSpread               float64
	shortSpread              float64
}
