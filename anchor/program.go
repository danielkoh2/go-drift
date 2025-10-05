package anchor

import (
	"driftgo/anchor/namespace"
	"driftgo/anchor/types"
	"github.com/gagliardetto/solana-go"
)

type Program struct {
	types.IProgram
	ProgramId solana.PublicKey
	Provider  types.IProvider
	Account   *namespace.AccountNamespace
}

func CreateProgram(
	programId solana.PublicKey,
	provider types.IProvider,
) *Program {
	program := &Program{
		ProgramId: programId,
		Provider:  provider,
		Account:   namespace.CreateAccountNamespace(provider),
	}
	provider.SetProgram(program)
	return program
}

func (p *Program) GetProgramId() solana.PublicKey {
	return p.ProgramId
}

func (p *Program) GetProvider() types.IProvider {
	return p.Provider
}

func (p *Program) GetAccounts(t any, accountName string) types.IAccountClient {
	return p.Account.Client(t, accountName)
}
