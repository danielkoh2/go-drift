package web3

import (
	"github.com/gagliardetto/solana-go"
	addresslookuptable "github.com/gagliardetto/solana-go/programs/address-lookup-table"
)

type TransactionMessage struct {
	PayerKey        solana.PublicKey
	Instructions    []solana.CompiledInstruction
	RecentBlockhash solana.Hash
}

func Decompile(message *solana.Message, addressLookupTableAccounts []addresslookuptable.KeyedAddressLookupTable) *TransactionMessage {
	var transactionMessage TransactionMessage
	numWritableSignedAccounts := message.Header.NumRequiredSignatures - message.Header.NumReadonlySignedAccounts
	numWritableUnsignedAccounts := len(message.AccountKeys) - int(message.Header.NumRequiredSignatures-message.Header.NumReadonlyUnsignedAccounts)
	accountKeys := message.GetAllKeys()
	transactionMessage.PayerKey = accountKeys[0]
	transactionMessage.Instructions = message.Instructions
	transactionMessage.RecentBlockhash = message.RecentBlockhash

}
