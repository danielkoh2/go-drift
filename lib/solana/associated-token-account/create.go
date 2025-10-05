package associatedtokenaccount

import (
	"errors"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/text/format"
	"github.com/gagliardetto/treeout"
)

type Create struct {
	Payer                  solana.PublicKey `bin:"-" borsh_skip:"true"`
	Mint                   solana.PublicKey `bin:"-" borsh_skip:"true"`
	Wallet                 solana.PublicKey `bin:"-" borsh_skip:"true"`
	AssociatedTokenAccount solana.PublicKey `bin:"-" borsh_skip:"true"`

	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

// NewCreateInstructionBuilder creates a new `Create` instruction builder.
func NewCreateInstructionBuilder() *Create {
	nd := &Create{}
	return nd
}

func (inst *Create) SetPayer(payer solana.PublicKey) *Create {
	inst.Payer = payer
	return inst
}

func (inst *Create) SetWallet(wallet solana.PublicKey) *Create {
	inst.Wallet = wallet
	return inst
}

func (inst *Create) SetMint(mint solana.PublicKey) *Create {
	inst.Mint = mint
	return inst
}

func (inst *Create) SetAssociatedToken(token solana.PublicKey) *Create {
	inst.AssociatedTokenAccount = token
	return inst
}

func (inst Create) Build() *associatedtokenaccount.Instruction {

	keys := []*solana.AccountMeta{
		{
			PublicKey:  inst.Payer,
			IsSigner:   true,
			IsWritable: true,
		},
		{
			PublicKey:  inst.AssociatedTokenAccount,
			IsSigner:   false,
			IsWritable: true,
		},
		{
			PublicKey:  inst.Wallet,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  inst.Mint,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  solana.SystemProgramID,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  solana.TokenProgramID,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  solana.SysVarRentPubkey,
			IsSigner:   false,
			IsWritable: false,
		},
	}

	inst.AccountMetaSlice = keys

	return &associatedtokenaccount.Instruction{BaseVariant: bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.NoTypeIDDefaultID,
	}}
}

// ValidateAndBuild validates the instruction accounts.
// If there is a validation error, return the error.
// Otherwise, build and return the instruction.
func (inst Create) ValidateAndBuild() (*associatedtokenaccount.Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

func (inst *Create) Validate() error {
	if inst.Payer.IsZero() {
		return errors.New("Payer not set")
	}
	if inst.Wallet.IsZero() {
		return errors.New("Wallet not set")
	}
	if inst.Mint.IsZero() {
		return errors.New("Mint not set")
	}
	if inst.AssociatedTokenAccount.IsZero() {
		return errors.New("Mint not set")
	}
	return nil
}

func (inst *Create) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program(associatedtokenaccount.ProgramName, associatedtokenaccount.ProgramID)).
		//
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child(format.Instruction("Create")).
				//
				ParentFunc(func(instructionBranch treeout.Branches) {

					// Parameters of the instruction:
					instructionBranch.Child("Params[len=0]").ParentFunc(func(paramsBranch treeout.Branches) {})

					// Accounts of the instruction:
					instructionBranch.Child("Accounts[len=7").ParentFunc(func(accountsBranch treeout.Branches) {
						accountsBranch.Child(format.Meta("                 payer", inst.AccountMetaSlice.Get(0)))
						accountsBranch.Child(format.Meta("associatedTokenAddress", inst.AccountMetaSlice.Get(1)))
						accountsBranch.Child(format.Meta("                wallet", inst.AccountMetaSlice.Get(2)))
						accountsBranch.Child(format.Meta("             tokenMint", inst.AccountMetaSlice.Get(3)))
						accountsBranch.Child(format.Meta("         systemProgram", inst.AccountMetaSlice.Get(4)))
						accountsBranch.Child(format.Meta("          tokenProgram", inst.AccountMetaSlice.Get(5)))
						accountsBranch.Child(format.Meta("            sysVarRent", inst.AccountMetaSlice.Get(6)))
					})
				})
		})
}

func (inst Create) MarshalWithEncoder(encoder *bin.Encoder) error {
	return encoder.WriteBytes([]byte{}, false)
}

func NewCreateInstruction(
	payer solana.PublicKey,
	walletAddress solana.PublicKey,
	splTokenMintAddress solana.PublicKey,
	associatedTokenAddress solana.PublicKey,
) *Create {
	return NewCreateInstructionBuilder().
		SetPayer(payer).
		SetWallet(walletAddress).
		SetMint(splTokenMintAddress).
		SetAssociatedToken(associatedTokenAddress)
}
