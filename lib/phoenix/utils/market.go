package utils

import "driftgo/lib/phoenix"

const DEFAULT_L2_LADDER_DEPTH int = 10
const DEFAULT_L3_BOOK_DEPTH int = 20
const DEFAULT_MATCH_LIMIT int = 2048
const DEFAULT_SLIPPAGE_PERCENT float64 = 0.005

func GetMarketUiLadder(
	market *phoenix.Market,
	levels int,
	slot uint64,
	unixTimestamp int64,
) *phoenix.UiLadder {
	return market.GetUiLadder(levels, slot, unixTimestamp)
}
