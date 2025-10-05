package types

import (
	"driftgo/constants"
	"driftgo/utils"
	"github.com/shopspring/decimal"
	"math"
	"math/big"
)

func convertPythPrice(
	price decimal.Decimal,
	exponent int32,
	multiple *big.Int,
) *big.Int {
	exponent = int32(math.Abs(float64(exponent)))
	pythPrecision, _ := utils.DivX(utils.PowX(constants.TEN, utils.BN(exponent)), multiple).Float64()

	priceF := utils.MulF(
		price.BigFloat(),
		utils.BF(math.Pow10(int(exponent))),
		utils.BF(constants.PRICE_PRECISION.Int64()),
	)

	pythPrice, _ := utils.DivF(priceF, utils.BF(pythPrecision)).Uint64()
	return utils.BN(pythPrice)
}

var fiveBPS = big.NewInt(500)

func getStableCoinPrice(
	price *big.Int,
	confidence *big.Int,
) *big.Int {
	if utils.AbsX(utils.SubX(price, constants.QUOTE_PRECISION)).Cmp(utils.Min(confidence, fiveBPS)) < 0 {
		return utils.IntX(constants.QUOTE_PRECISION)
	} else {
		return price
	}
}
