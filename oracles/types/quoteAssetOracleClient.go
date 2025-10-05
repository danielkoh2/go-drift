package types

import (
	"driftgo/constants"
	"driftgo/utils"
	"github.com/gagliardetto/solana-go"
)

var QUOTE_ORACLE_PRICE_DATA = OraclePriceData{
	Price:                           constants.PRICE_PRECISION,
	Slot:                            0,
	Confidence:                      utils.BN(1),
	HasSufficientNumberOfDataPoints: true,
	Twap:                            nil,
	TwapConfidence:                  nil,
}

type QuoteAssetOracleClient struct {
	IOracleClient
}

func CreateQuoteAssetOracleClient() *QuoteAssetOracleClient {
	return &QuoteAssetOracleClient{}
}

func (p *QuoteAssetOracleClient) GetOraclePriceData(key solana.PublicKey) *OraclePriceData {
	return &QUOTE_ORACLE_PRICE_DATA
}

func (p *QuoteAssetOracleClient) GetOraclePriceDataFromBuffer(buffer []byte) *OraclePriceData {
	return &QUOTE_ORACLE_PRICE_DATA
}
