package phoenix

import ag_binary "github.com/gagliardetto/binary"

type MarketSizeParams struct {
	BidsSize uint64
	AsksSize uint64
	NumSeats uint64
}

func (obj MarketSizeParams) MarshalWithEncoder(encoder *ag_binary.Encoder) error {
	return nil
}

func (obj MarketSizeParams) UnmarshalWithDecoder(decoder *ag_binary.Decoder) error {
	err := decoder.Decode(&obj.BidsSize)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.AsksSize)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.NumSeats)
	if err != nil {
		return err
	}
	return nil
}
