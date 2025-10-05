package spl_token

import (
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
)

func CreateInitializeAccountInstruction(
	account solana.PublicKey,
	mint solana.PublicKey,
	owner solana.PublicKey,
	programId solana.PublicKey,
) solana.Instruction {
	return token.NewInitializeAccountInstructionBuilder().
		SetAccount(account).
		SetMintAccount(mint).
		SetOwnerAccount(owner).
		SetSysVarRentPubkeyAccount(solana.SysVarRentPubkey).Build()

}
