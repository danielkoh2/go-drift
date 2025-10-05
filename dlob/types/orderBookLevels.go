package types

import (
	"driftgo/common"
	"driftgo/constants"
	"driftgo/utils"
	"github.com/gagliardetto/solana-go"
	"math/big"
)

type LiquiditySource string

const (
	LiquiditySourceSerum   LiquiditySource = "serum"
	LiquiditySourceVamm    LiquiditySource = "vamm"
	LiquiditySourceDlob    LiquiditySource = "dlob"
	LiquiditySourcePhoenix LiquiditySource = "phoenix"
)

type L2Level struct {
	Price   *big.Int
	Size    *big.Int
	Sources map[LiquiditySource]*big.Int
}

type L2OrderBook struct {
	Asks []*L2Level
	Bids []*L2Level
	Slot uint64
}

type L2OrderBookGenerator struct {
	GetL2Asks func() *common.Generator[*L2Level, int]
	GetL2Bids func() *common.Generator[*L2Level, int]
}

type L3Level struct {
	Price   *big.Int
	Size    *big.Int
	Maker   solana.PublicKey
	OrderId uint32
}

type L3OrderBook struct {
	Asks []L3Level
	Bids []L3Level
	Slot uint64
}

var DEFAULT_TOP_OF_BOOK_QUOTE_AMOUNTS = []*big.Int{
	utils.MulX(utils.BN(500), constants.QUOTE_PRECISION),
	utils.MulX(utils.BN(1000), constants.QUOTE_PRECISION),
	utils.MulX(utils.BN(2000), constants.QUOTE_PRECISION),
	utils.MulX(utils.BN(5000), constants.QUOTE_PRECISION),
}
