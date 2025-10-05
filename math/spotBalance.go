package math

import (
	"driftgo/constants"
	"driftgo/lib/drift"
	oracles "driftgo/oracles/types"
	"driftgo/utils"
	"math/big"
)

//GetBalance
/**
 * Calculates the balance of a given token amount including any accumulated interest. This
 * is the same as `SpotPosition.scaledBalance`.
 *
 * @param {BN} tokenAmount - the amount of tokens
 * @param {SpotMarketAccount} spotMarket - the spot market account
 * @param {SpotBalanceType} balanceType - the balance type ('deposit' or 'borrow')
 * @return {BN} the calculated balance, scaled by `SPOT_MARKET_BALANCE_PRECISION`
 */
func GetBalance(
	tokenAmount *big.Int,
	spotMarket *drift.SpotMarket,
	balanceType drift.SpotBalanceType,
) *big.Int {
	precisionIncrease := utils.PowX(utils.BN(10), utils.BN(19-int64(spotMarket.Decimals)))

	cumulativeInterest := spotMarket.CumulativeDepositInterest
	if balanceType == drift.SpotBalanceType_Borrow {
		cumulativeInterest = spotMarket.CumulativeBorrowInterest
	}

	balance := utils.DivX(utils.MulX(tokenAmount, precisionIncrease), cumulativeInterest.BigInt())

	if balance.Cmp(constants.ZERO) != 0 && balanceType == drift.SpotBalanceType_Borrow {
		balance = utils.AddX(balance, utils.BN(1))
	}
	return balance
}

//GetTokenAmount
/**
 * Calculates the spot token amount including any accumulated interest.
 *
 * @param {BN} balanceAmount - The balance amount, typically from `SpotPosition.scaledBalance`
 * @param {SpotMarketAccount} spotMarket - The spot market account details
 * @param {SpotBalanceType} balanceType - The balance type to be used for calculation
 * @returns {BN} The calculated token amount, scaled by `SpotMarketConfig.precision`
 */
func GetTokenAmount(
	balanceAmount *big.Int,
	spotMarket *drift.SpotMarket,
	balanceType drift.SpotBalanceType,
) *big.Int {
	precisionIncrease := utils.PowX(utils.BN(10), utils.BN(19-int64(spotMarket.Decimals)))
	if balanceType == drift.SpotBalanceType_Deposit {
		return utils.DivX(utils.MulX(balanceAmount, spotMarket.CumulativeDepositInterest.BigInt()), precisionIncrease)
	} else {
		return utils.DivCeilX(utils.MulX(balanceAmount, spotMarket.CumulativeBorrowInterest.BigInt()), precisionIncrease)
	}
}

//GetSignedTokenAmount
/**
 * Returns the signed (positive for deposit,negative for borrow) token amount based on the balance type.
 *
 * @param {BN} tokenAmount - The token amount to convert (from `getTokenAmount`)
 * @param {SpotBalanceType} balanceType - The balance type to determine the sign of the token amount.
 * @returns {BN} - The signed token amount, scaled by `SpotMarketConfig.precision`
 */
func GetSignedTokenAmount(
	tokenAmount *big.Int,
	balanceType drift.SpotBalanceType,
) *big.Int {
	if balanceType == drift.SpotBalanceType_Deposit {
		return tokenAmount
	} else {
		return utils.NegX(utils.AbsX(tokenAmount))
	}
}

//GetStrictTokenValue
/**
 * Calculates the value of a given token amount using the worst of the provided oracle price and its TWAP.
 *
 * @param {BN} tokenAmount - The amount of tokens to calculate the value for (from `getTokenAmount`)
 * @param {number} spotDecimals - The number of decimals in the token.
 * @param {StrictOraclePrice} strictOraclePrice - Contains oracle price and 5min twap.
 * @return {BN} The calculated value of the given token amount, scaled by `PRICE_PRECISION`
 */
func GetStrictTokenValue(
	tokenAmount *big.Int,
	spotDecimals int64,
	strictOraclePrice *oracles.StrictOraclePrice,
) *big.Int {
	if tokenAmount.Cmp(constants.ZERO) == 0 {
		return utils.BN(0)
	}

	var price *big.Int
	if tokenAmount.Cmp(constants.ZERO) >= 0 {
		price = strictOraclePrice.Min()
	} else {
		price = strictOraclePrice.Max()
	}

	precisionDecrease := utils.PowX(utils.BN(10), utils.BN(spotDecimals))

	return utils.DivX(utils.MulX(tokenAmount, price), precisionDecrease)
}

// GetTokenValue
/**
 * Calculates the value of a given token amount in relation to an oracle price data
 *
 * @param {BN} tokenAmount - The amount of tokens to calculate the value for (from `getTokenAmount`)
 * @param {number} spotDecimals - The number of decimal places of the token.
 * @param {OraclePriceData} oraclePriceData - The oracle price data (typically a token/USD oracle).
 * @return {BN} The value of the token based on the oracle, scaled by `PRICE_PRECISION`
 */
func GetTokenValue(
	tokenAmount *big.Int,
	spotDecimals int64,
	oraclePriceData *oracles.OraclePriceData,
) *big.Int {
	if tokenAmount.Cmp(constants.ZERO) == 0 {
		return utils.BN(0)
	}
	precisionDecrease := utils.PowX(utils.BN(10), utils.BN(spotDecimals))

	return utils.DivX(utils.MulX(tokenAmount, oraclePriceData.Price), precisionDecrease)
}

// CalculateAssetWeight
func CalculateAssetWeight(
	balanceAmount *big.Int,
	oraclePrice *big.Int,
	spotMarket *drift.SpotMarket,
	marginCategory drift.MarginRequirementType,
) *big.Int {
	sizePrecision := utils.PowX(utils.BN(10), utils.BN(spotMarket.Decimals))
	var sizeInAmmReservePrecision *big.Int
	if sizePrecision.Cmp(constants.AMM_RESERVE_PRECISION) > 0 {
		sizeInAmmReservePrecision = utils.DivX(balanceAmount, utils.DivX(sizePrecision, constants.AMM_RESERVE_PRECISION))
	} else {
		sizeInAmmReservePrecision = utils.DivX(utils.MulX(balanceAmount, constants.AMM_RESERVE_PRECISION), sizePrecision)
	}

	var assetWeight *big.Int
	switch marginCategory {
	case drift.MarginRequirementType_Initial:
		assetWeight = CalculateSizeDiscountAssetWeight(
			sizeInAmmReservePrecision,
			utils.BN(spotMarket.ImfFactor),
			CalculateScaledInitialAssetWeight(spotMarket, oraclePrice),
		)
	case drift.MarginRequirementType_Maintenance:
		assetWeight = CalculateSizeDiscountAssetWeight(
			sizeInAmmReservePrecision,
			utils.BN(spotMarket.ImfFactor),
			utils.BN(spotMarket.MaintenanceAssetWeight),
		)
	default:
		assetWeight = CalculateScaledInitialAssetWeight(spotMarket, oraclePrice)
	}
	return assetWeight
}

// CalculateScaledInitialAssetWeight
func CalculateScaledInitialAssetWeight(
	spotMarket *drift.SpotMarket,
	oraclePrice *big.Int,
) *big.Int {
	if spotMarket.ScaleInitialAssetWeightStart == 0 {
		return utils.BN(spotMarket.InitialAssetWeight)
	}

	deposits := GetTokenAmount(
		spotMarket.DepositBalance.BigInt(),
		spotMarket,
		drift.SpotBalanceType_Deposit,
	)
	depositsValue := GetTokenValue(
		deposits,
		int64(spotMarket.Decimals),
		&oracles.OraclePriceData{
			Price: oraclePrice,
		},
	)

	if depositsValue.Cmp(utils.BN(spotMarket.ScaleInitialAssetWeightStart)) < 0 {
		return utils.BN(spotMarket.InitialAssetWeight)
	} else {
		return utils.DivX(
			utils.BN(uint64(spotMarket.InitialAssetWeight)*spotMarket.ScaleInitialAssetWeightStart),
			depositsValue,
		)
	}
}

// CalculateLiabilityWeight
func CalculateLiabilityWeight(
	size *big.Int,
	spotMarket *drift.SpotMarket,
	marginCategory drift.MarginRequirementType,
) *big.Int {
	sizePrecision := utils.PowX(utils.BN(10), utils.BN(spotMarket.Decimals))
	var sizeInAmmReservePrecision *big.Int
	if sizePrecision.Cmp(constants.AMM_RESERVE_PRECISION) > 0 {
		sizeInAmmReservePrecision = utils.DivX(size, utils.DivX(sizePrecision, constants.AMM_RESERVE_PRECISION))
	} else {
		sizeInAmmReservePrecision = utils.DivX(utils.MulX(size, constants.AMM_RESERVE_PRECISION), sizePrecision)
	}

	var liabilityWeight *big.Int
	switch marginCategory {
	case drift.MarginRequirementType_Initial:
		liabilityWeight = CalculateSizePremiumLiabilityWeight(
			sizeInAmmReservePrecision,
			utils.BN(spotMarket.ImfFactor),
			utils.BN(spotMarket.InitialLiabilityWeight),
			constants.SPOT_MARKET_WEIGHT_PRECISION,
		)
	case drift.MarginRequirementType_Maintenance:
		liabilityWeight = CalculateSizePremiumLiabilityWeight(
			sizeInAmmReservePrecision,
			utils.BN(spotMarket.ImfFactor),
			utils.BN(spotMarket.MaintenanceLiabilityWeight),
			constants.SPOT_MARKET_WEIGHT_PRECISION,
		)
	default:
		liabilityWeight = utils.BN(spotMarket.InitialLiabilityWeight)
	}
	return liabilityWeight
}
