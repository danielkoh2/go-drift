package math

import "driftgo/lib/drift"

func GetPerpMarketTierNumber(perpMarket *drift.PerpMarket) uint8 {
	if perpMarket.ContractTier == drift.ContractTier_A {
		return 0
	} else if perpMarket.ContractTier == drift.ContractTier_B {
		return 1
	} else if perpMarket.ContractTier == drift.ContractTier_C {
		return 2
	} else if perpMarket.ContractTier == drift.ContractTier_Speculative {
		return 3
	} else if perpMarket.ContractTier == drift.ContractTier_HighlySpeculative {
		return 4
	} else {
		return 5
	}
}

func GetSpotMarketTierNumber(spotMarket *drift.SpotMarket) uint8 {
	if spotMarket.AssetTier == drift.AssetTier_Collateral {
		return 0
	} else if spotMarket.AssetTier == drift.AssetTier_Protected {
		return 1
	} else if spotMarket.AssetTier == drift.AssetTier_Cross {
		return 2
	} else if spotMarket.AssetTier == drift.AssetTier_Isolated {
		return 3
	} else if spotMarket.AssetTier == drift.AssetTier_Unlisted {
		return 4
	} else {
		return 5
	}
}

func PerpTierIsAsSafeAs(
	perpTier uint8,
	otherPerpTier uint8,
	otherSpotTier uint8,
) bool {
	asSafeAsPerp := perpTier <= otherPerpTier
	asSafeAsSpot := otherSpotTier == 4 || (otherSpotTier >= 2 && perpTier <= 2)
	return asSafeAsSpot && asSafeAsPerp
}
