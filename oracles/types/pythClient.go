package types

import (
	"context"
	"driftgo/constants"
	"driftgo/lib/pyth"
	"driftgo/utils"
	"fmt"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
	"math/big"
)

type PythClient struct {
	IOracleClient
	connection *rpc.Client
	multiple   *big.Int
	stableCoin bool
}

type PythPriceData struct {
	pyth.PriceAccount
	Drv1               *big.Int
	PreviousPrice      *big.Int
	PreviousConfidence *big.Int
	Drv5               *big.Int
	Price              decimal.Decimal
	Confidence         decimal.Decimal
}

func CreatePythClient(
	connection *rpc.Client,
	multiple *big.Int,
	stableCoin bool,
) IOracleClient {
	return &PythClient{
		connection: connection,
		multiple:   multiple,
		stableCoin: stableCoin,
	}
}

func (p *PythClient) GetOraclePriceData(pricePublicKey solana.PublicKey) *OraclePriceData {
	out, err := p.connection.GetAccountInfo(context.TODO(), pricePublicKey)
	if err != nil {
		return nil
	}
	return p.GetOraclePriceDataFromBuffer(out.GetBinary())
}

func (p *PythClient) GetOraclePriceDataFromBuffer(buffer []byte) *OraclePriceData {
	var priceData PythPriceData

	err := bin.NewBinDecoder(buffer).Decode(&priceData.PriceAccount)
	if err != nil {
		return nil
	}
	priceData.Drv1 = utils.MulX(
		utils.BN(priceData.PriceAccount.Drv1Component),
		utils.PowX(
			utils.BN(10),
			utils.BN(priceData.PriceAccount.Exponent),
		),
	)
	priceData.Drv5 = utils.MulX(
		utils.BN(priceData.PriceAccount.Drv5Component),
		utils.PowX(
			utils.BN(10),
			utils.BN(priceData.PriceAccount.Exponent),
		),
	)
	priceData.PreviousPrice = utils.MulX(
		utils.BN(priceData.PreviousPriceComponent),
		utils.PowX(
			utils.BN(10),
			utils.BN(priceData.Exponent),
		),
	)
	priceData.PreviousConfidence = utils.MulX(
		utils.BN(priceData.PreviousConfidenceComponent),
		utils.PowX(
			utils.BN(10),
			utils.BN(priceData.Exponent),
		),
	)
	aggregatePrice, aggregateConfidence, ok := priceData.Aggregate.Value(priceData.PriceAccount.Exponent)
	if ok {
		priceData.Price = aggregatePrice
		priceData.Confidence = aggregateConfidence
	}
	//if priceData.PriceAccount.Aggregate.Status == 1 {
	//
	//}
	//spew.Dump(priceAccount)
	var confidence *big.Int
	confidence = convertPythPrice(
		priceData.Confidence,
		priceData.Exponent,
		p.multiple,
	)
	minPublishers := priceData.NumComponentPrices
	if minPublishers > 3 {
		minPublishers = 3
	}
	price := convertPythPrice(
		aggregatePrice,
		priceData.Exponent,
		p.multiple,
	)
	if p.stableCoin {
		price = getStableCoinPrice(price, confidence)
	}
	if price == nil || price.Cmp(constants.ZERO) == 0 {
		fmt.Println("O-Zero - confidence=", aggregateConfidence.String(), ",price=", aggregatePrice.String(), ",status=", priceData.PriceAccount.Aggregate.Status)
	}
	return &OraclePriceData{
		Price:      price,
		Slot:       priceData.LastSlot,
		Confidence: confidence,
		Twap: convertPythPrice(
			priceData.Twap.GetValue(priceData.Exponent),
			priceData.Exponent,
			p.multiple,
		),
		TwapConfidence: convertPythPrice(
			priceData.Twac.GetValue(priceData.Exponent),
			priceData.Exponent,
			p.multiple,
		),
		HasSufficientNumberOfDataPoints: priceData.NumQuoters >= minPublishers,
	}
}
