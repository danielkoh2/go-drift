package types

import (
	"context"
	"driftgo/constants"
	"driftgo/lib/switchboard_v2"
	"driftgo/utils"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"math/big"
)

type SwitchboardClient struct {
	IOracleClient
	connection *rpc.Client
}

func convertSwitchboardDecimal(switchboardDecimal *switchboard_v2.SwitchboardDecimal) *big.Int {
	switchboardPrecision := utils.PowX(utils.BN(10), utils.BN(switchboardDecimal.Scale))
	return utils.DivX(utils.MulX(switchboardDecimal.Mantissa.BigInt(), constants.PRICE_PRECISION), switchboardPrecision)
}

func CreateSwitchboardClient(connection *rpc.Client) *SwitchboardClient {
	return &SwitchboardClient{connection: connection}
}

func (p *SwitchboardClient) GetOraclePriceData(pricePublicKey solana.PublicKey) *OraclePriceData {
	out, err := p.connection.GetAccountInfo(context.TODO(), pricePublicKey)
	if err != nil {
		return nil
	}
	return p.GetOraclePriceDataFromBuffer(out.GetBinary())
}

func (p *SwitchboardClient) GetOraclePriceDataFromBuffer(buffer []byte) *OraclePriceData {
	var aggregatorAccountData switchboard_v2.AggregatorAccountData
	err := bin.NewBorshDecoder(buffer).Decode(&aggregatorAccountData)
	if err != nil {
		return nil
	}
	price := convertSwitchboardDecimal(&aggregatorAccountData.LatestConfirmedRound.Result)

	confidence := utils.Max(convertSwitchboardDecimal(&aggregatorAccountData.LatestConfirmedRound.StdDeviation), utils.DivX(price, utils.BN(1000)))

	hasSufficientNumberOfDataPoints :=
		aggregatorAccountData.LatestConfirmedRound.NumSuccess >= aggregatorAccountData.MinOracleResults

	slot := aggregatorAccountData.LatestConfirmedRound.RoundOpenSlot

	return &OraclePriceData{
		Price:                           price,
		Slot:                            slot,
		Confidence:                      confidence,
		HasSufficientNumberOfDataPoints: hasSufficientNumberOfDataPoints,
	}
}
