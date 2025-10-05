package math

import (
	"driftgo/constants"
	"driftgo/lib/drift"
	"driftgo/utils"
	"fmt"
	"math/big"
)

// line 22 OK
func CalculateAdjustKCost(amm *drift.Amm, numerator *big.Int, denomenator *big.Int) *big.Int {
	x := amm.BaseAssetReserve.BigInt()
	y := amm.QuoteAssetReserve.BigInt()

	d := amm.BaseAssetAmountWithAmm.BigInt()
	Q := amm.PegMultiplier.BigInt()

	quoteScale := utils.MulX(utils.MulX(y, d), Q)

	p := utils.DivX(utils.MulX(numerator, constants.PRICE_PRECISION), denomenator)
	cost := utils.MulX(quoteScale, constants.PERCENTAGE_PRECISION, constants.PERCENTAGE_PRECISION)
	cost = utils.DivX(cost, utils.AddX(x, d))

	quoteScaleTemp := utils.MulX(p, constants.PERCENTAGE_PRECISION, constants.PERCENTAGE_PRECISION)
	quoteScaleTemp = utils.DivX(
		quoteScaleTemp,
		constants.PRICE_PRECISION,
		utils.AddX(
			utils.DivX(
				utils.MulX(x, p),
				constants.PRICE_PRECISION,
			),
			d,
		),
	)
	cost = utils.SubX(cost, quoteScaleTemp)
	cost = utils.DivX(
		cost,
		constants.PERCENTAGE_PRECISION,
		constants.PERCENTAGE_PRECISION,
		constants.AMM_TO_QUOTE_PRECISION_RATIO,
		constants.PEG_PRECISION,
	)

	return utils.MulX(cost, big.NewInt(-1))
}

// line 91 OK
func CalculateRepegCost(amm *drift.Amm, newPeg *big.Int) *big.Int {
	dqar := utils.SubX(amm.QuoteAssetReserve.BigInt(), amm.TerminalQuoteAssetReserve.BigInt())
	cost := utils.MulX(dqar, utils.SubX(newPeg, amm.PegMultiplier.BigInt()))
	cost = utils.DivX(dqar, constants.AMM_TO_QUOTE_PRECISION_RATIO, constants.PEG_PRECISION)
	return cost
}

func CalculateBudgetedKBN(
	x *big.Int,
	y *big.Int,
	budget *big.Int,
	Q *big.Int,
	d *big.Int,
) (*big.Int, *big.Int) {
	if Q.Cmp(constants.ZERO) < 0 {
		panic("Error")
	}
	C := utils.MulX(budget, big.NewInt(-1))

	dSign := utils.BN(1)
	if d.Cmp(constants.ZERO) < 0 {
		dSign = big.NewInt(-1)
	}
	pegged_y_d_d := utils.MulX(y, d, d, Q)
	pegged_y_d_d = utils.DivX(pegged_y_d_d, constants.AMM_RESERVE_PRECISION, constants.AMM_RESERVE_PRECISION, constants.PEG_PRECISION)

	numer1 := pegged_y_d_d
	numer2 := utils.MulX(C, d)
	numer2 = utils.DivX(numer2, constants.QUOTE_PRECISION)
	numer2 = utils.MulX(numer2, utils.AddX(x, d))
	numer2 = utils.DivX(numer2, constants.AMM_RESERVE_PRECISION)
	numer2 = utils.MulX(numer2, dSign)

	denom1 := utils.MulX(C, x, utils.AddX(x, d))
	denom1 = utils.DivX(denom1, constants.AMM_RESERVE_PRECISION, constants.QUOTE_PRECISION)
	denom2 := pegged_y_d_d

	if C.Cmp(constants.ZERO) < 0 {
		if utils.AbsX(denom1).Cmp(utils.AbsX(denom2)) > 0 {
			fmt.Printf("denom1 > denom2 %s %s\n", denom1.String(), denom2.String())
			fmt.Println("budget cost exceeds stable K solution")
			return big.NewInt(10000), big.NewInt(1)
		}
	}

	numerator := utils.DivX(utils.SubX(numer1, numer2), constants.AMM_TO_QUOTE_PRECISION_RATIO)
	denominator := utils.DivX(utils.AddX(denom1, denom2), constants.AMM_TO_QUOTE_PRECISION_RATIO)

	return numerator, denominator
}

func CalculateBudgetedK(amm *drift.Amm, cost *big.Int) (*big.Int, *big.Int) {
	// wolframalpha.com
	// (1/(x+d) - p/(x*p+d))*y*d*Q = C solve for p
	// p = (d(y*d*Q - C(x+d))) / (C*x(x+d) + y*d*d*Q)

	// numer
	//   =  y*d*d*Q - Cxd - Cdd
	//   =  y/x*Q*d*d - Cd - Cd/x
	//   = mark      - C/d - C/(x)
	//   =  mark/C    - 1/d - 1/x

	// denom
	// = C*x*x + C*x*d + y*d*d*Q
	// = x/d**2 + 1 / d + mark/C

	// todo: assumes k = x * y
	// otherwise use: (y(1-p) + (kp^2/(x*p+d)) - k/(x+d)) * Q = C solve for p

	x := amm.BaseAssetReserve.BigInt()
	y := amm.QuoteAssetReserve.BigInt()

	d := amm.BaseAssetAmountWithAmm.BigInt()
	Q := amm.PegMultiplier.BigInt()

	return CalculateBudgetedKBN(x, y, cost, Q, d)
}

// line 180 OK
func CalculateBudgetedPeg(amm *drift.Amm, budget *big.Int, targetPrice *big.Int) *big.Int {
	perPegCost := utils.DivX(
		utils.SubX(amm.QuoteAssetReserve.BigInt(), amm.TerminalQuoteAssetReserve.BigInt()),
		utils.DivX(constants.AMM_RESERVE_PRECISION, constants.PRICE_PRECISION),
	)

	if perPegCost.Cmp(constants.ZERO) > 0 {
		perPegCost = utils.AddX(perPegCost, utils.BN(1))
	} else if perPegCost.Cmp(constants.ZERO) < 0 {
		perPegCost = utils.SubX(perPegCost, utils.BN(1))
	}

	targetPeg := utils.DivX(
		utils.MulX(targetPrice, amm.BaseAssetReserve.BigInt()),
		amm.QuoteAssetReserve.BigInt(),
		constants.PRICE_DIV_PEG,
	)

	pegChangeDirection := utils.SubX(targetPeg, amm.PegMultiplier.BigInt())

	useTargetPeg :=
		(perPegCost.Cmp(constants.ZERO) < 0 && pegChangeDirection.Cmp(constants.ZERO) > 0) ||
			(perPegCost.Cmp(constants.ZERO) > 0 && pegChangeDirection.Cmp(constants.ZERO) < 0)

	if perPegCost.Cmp(constants.ZERO) == 0 || useTargetPeg {
		return targetPeg
	}

	budgetDeltaPeg := utils.DivX(utils.MulX(budget, constants.PEG_PRECISION), perPegCost)
	newPeg := utils.Max(utils.BN(1), utils.AddX(amm.PegMultiplier.BigInt(), budgetDeltaPeg))

	return newPeg
}
