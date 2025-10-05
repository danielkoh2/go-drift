package types

import (
	"context"
	"driftgo/anchor/types"
	"driftgo/lib/drift"
	"driftgo/utils"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type PrelaunchOracleClient struct {
	IOracleClient
	connection *rpc.Client
	program    types.IProgram
}

func CreatePrelaunchOracleClient(
	connection *rpc.Client,
	program types.IProgram,
) *PrelaunchOracleClient {
	return &PrelaunchOracleClient{
		connection: connection,
		program:    program,
	}
}

func (p *PrelaunchOracleClient) GetOraclePriceData(
	pricePublicKey solana.PublicKey,
) *OraclePriceData {
	out, err := p.connection.GetAccountInfo(context.TODO(), pricePublicKey)
	if err != nil {
		return nil
	}
	return p.GetOraclePriceDataFromBuffer(out.GetBinary())
}

func (p *PrelaunchOracleClient) GetOraclePriceDataFromBuffer(buffer []byte) *OraclePriceData {
	var prelaunchOracle drift.PrelaunchOracle

	err := bin.NewBinDecoder(buffer).Decode(&prelaunchOracle)
	if err != nil {
		return nil
	}

	return &OraclePriceData{
		Price:                           utils.BN(prelaunchOracle.Price),
		Slot:                            prelaunchOracle.AmmLastUpdateSlot,
		Confidence:                      utils.BN(prelaunchOracle.Confidence),
		HasSufficientNumberOfDataPoints: true,
		Twap:                            nil,
		TwapConfidence:                  nil,
		MaxPrice:                        utils.BN(prelaunchOracle.MaxPrice),
	}
}
