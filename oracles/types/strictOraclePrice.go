package types

import (
	"driftgo/utils"
	"math/big"
)

type StrictOraclePrice struct {
	Current *big.Int
	Twap    *big.Int
}

func (p *StrictOraclePrice) Max() *big.Int {
	if p.Twap != nil {
		return utils.Max(p.Twap, p.Current)
	}
	return p.Current
}

func (p *StrictOraclePrice) Min() *big.Int {
	if p.Twap != nil {
		return utils.Min(p.Twap, p.Current)
	}
	return p.Current
}
