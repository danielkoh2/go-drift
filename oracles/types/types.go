package types

import (
	"driftgo/lib/drift"
	"github.com/gagliardetto/solana-go"
	"math/big"
)

type OraclePriceData struct {
	Price                           *big.Int
	Slot                            uint64
	Confidence                      *big.Int
	HasSufficientNumberOfDataPoints bool
	Twap                            *big.Int
	TwapConfidence                  *big.Int
	MaxPrice                        *big.Int
}

type OracleInfo struct {
	PublicKey solana.PublicKey
	Source    drift.OracleSource
}

type IOracleClient interface {
	GetOraclePriceDataFromBuffer(buffer []byte) *OraclePriceData
	GetOraclePriceData(key solana.PublicKey) *OraclePriceData
}
