package math

import (
	"driftgo/constants"
	"driftgo/utils"
	"math/big"
)

func ConvertToNumber(bigNumber *big.Int, precision ...*big.Int) int64 {
	if bigNumber == nil {
		return 0
	}
	var precisionx *big.Int
	if len(precision) == 0 {
		precisionx = utils.IntX(constants.PRICE_PRECISION)
	} else {
		precisionx = precision[0]
	}
	return utils.DivX(bigNumber, precisionx).Int64() + utils.ModX(bigNumber, precisionx).Int64()/precisionx.Int64()
}
