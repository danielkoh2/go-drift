package phoenix

import (
	ag_binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type TokenParams struct {
	Decimals  uint32
	VaultBump uint32
	MintKey   solana.PublicKey
	VaultKey  solana.PublicKey
}

func (obj TokenParams) MarshalWithEncoder(encoder *ag_binary.Encoder) error {
	return nil
}

func (obj TokenParams) UnmarshalWithDecoder(decoder *ag_binary.Decoder) error {
	err := decoder.Decode(&obj.Decimals)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.VaultBump)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.MintKey)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.VaultKey)
	if err != nil {
		return err
	}
	return nil
}
