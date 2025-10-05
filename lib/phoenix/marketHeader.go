package phoenix

import (
	ag_binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type MarketHeader struct {
	Discriminant                    [8]byte
	Status                          uint64
	MarketSizeParams                MarketSizeParams
	BaseParams                      TokenParams
	BaseLotSize                     uint64
	QuoteParams                     TokenParams
	QuoteLotSize                    uint64
	TickSizeInQuoteAtomsPerBaseUnit uint64
	Authority                       solana.PublicKey
	FeeRecipient                    solana.PublicKey
	MarketSequenceNumber            uint64
	Successor                       solana.PublicKey
	RawBaseUnitsPerBaseUnit         uint32
	Padding1                        uint32
	Padding2                        [32]uint64
}

func (obj MarketHeader) MarshalWithEncoder(encoder *ag_binary.Encoder) error {
	return nil
}

func (obj MarketHeader) UnmarshalWithDecoder(decoder *ag_binary.Decoder) error {
	discriminator, err := decoder.ReadTypeID()
	if err != nil {
		return err
	}
	obj.Discriminant = discriminator

	err = decoder.Decode(&obj.Status)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.MarketSizeParams)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.BaseParams)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.BaseLotSize)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.QuoteParams)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.QuoteLotSize)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.TickSizeInQuoteAtomsPerBaseUnit)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.Authority)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.FeeRecipient)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.MarketSequenceNumber)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.Successor)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.RawBaseUnitsPerBaseUnit)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.Padding1)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.Padding2)
	if err != nil {
		return err
	}
	return nil
}
