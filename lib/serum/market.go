package serum

import (
	"context"
	"fmt"
	agBinary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

type Market struct {
	Address    solana.PublicKey `json:"address"`
	ProgramId  solana.PublicKey
	Deprecated bool `json:"deprecated"`
	QuoteMint  *token.Mint
	BaseMint   *token.Mint
	Data       *MarketStateV3
}

func (p *Market) BaseSplTokenMultiplier() uint64 {
	return solana.DecimalsInBigInt(uint32(p.BaseMint.Decimals)).Uint64()
}

func (p *Market) QuoteSplTokenMultiplier() uint64 {
	return solana.DecimalsInBigInt(uint32(p.QuoteMint.Decimals)).Uint64()
}

func (p *Market) Reload(buffer []byte) *Market {
	decoder := agBinary.NewBinDecoder(buffer)
	var marketState MarketStateV3
	version := GetLayoutVersion(p.ProgramId.String())
	var err error
	if version == 1 {
		err = decoder.Decode(&marketState.MarketState)
	} else if version == 2 {
		err = decoder.Decode(&marketState.MarketStateV2)
	} else {
		err = decoder.Decode(&marketState.MarketStateV2)
	}
	if err != nil {
		fmt.Println("Serum market reload failed: version=", version, ",err=", err)
		return p
	}
	p.Data = &marketState
	return p
}

func (p *Market) LoadQuoteMint(connection *rpc.Client) {
	if p.Data == nil {
		return
	}
	var quoteMint token.Mint
	accountInfo, err := connection.GetAccountInfo(context.TODO(), p.Data.QuoteMint)
	if err != nil {
		return
	}
	err = agBinary.NewBinDecoder(accountInfo.GetBinary()).Decode(&quoteMint)
	if err == nil {
		p.QuoteMint = &quoteMint
	}
}

func (p *Market) LoadBaseMint(connection *rpc.Client) {
	if p.Data == nil {
		return
	}
	var baseMint token.Mint
	accountInfo, err := connection.GetAccountInfo(context.TODO(), p.Data.BaseMint)
	if err != nil {
		return
	}
	err = agBinary.NewBinDecoder(accountInfo.GetBinary()).Decode(&baseMint)
	if err == nil {
		p.BaseMint = &baseMint
	}

}

func (p *Market) LoadOrderbookFromBuffer(buffer []byte) *Orderbook {
	var orderBook Orderbook
	err := agBinary.NewBinDecoder(buffer).Decode(&orderBook)
	if err != nil {
		fmt.Println("Serum asks load failed: pubkey=", p.Data.Asks, ",err=", err, ",len=", len(buffer))
		return nil
	}
	return &orderBook
}

func (p *Market) LoadAsks(connection *rpc.Client) *Orderbook {
	accountInfo, err := connection.GetAccountInfo(context.TODO(), p.Data.Asks)
	if err != nil {
		return nil
	}
	return p.LoadOrderbookFromBuffer(accountInfo.GetBinary())
}

func (p *Market) LoadBids(connection *rpc.Client) *Orderbook {
	accountInfo, err := connection.GetAccountInfo(context.TODO(), p.Data.Bids)
	if err != nil {
		return nil
	}
	return p.LoadOrderbookFromBuffer(accountInfo.GetBinary())
}

func LoadMarketFromAddress(
	connection *rpc.Client,
	address solana.PublicKey,
	programId solana.PublicKey,
) *Market {
	accountInfo, err := connection.GetAccountInfo(context.TODO(), address)
	if err != nil {
		return nil
	}
	market := &Market{
		Address:   address,
		ProgramId: programId,
	}
	market.Reload(accountInfo.GetBinary())
	market.LoadBaseMint(connection)
	market.LoadQuoteMint(connection)
	return market
}

func LoadMarketFromBuffer(address solana.PublicKey, programId solana.PublicKey, buffer []byte) *Market {
	market := &Market{
		Address:   address,
		ProgramId: programId,
	}
	market.Reload(buffer)
	return market
}
