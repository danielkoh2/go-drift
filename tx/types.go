package tx

import (
	go_drift "driftgo"
	"github.com/gagliardetto/solana-go"
	addresslookuptable "github.com/gagliardetto/solana-go/programs/address-lookup-table"
)

type ExtraConfirmationOptions struct {
	OnSignedCb func()
}

type TxSigAndSlot struct {
	TxSig solana.Signature
	Slot  uint64
}

type ITxSender interface {
	Send(
		tx *solana.Transaction,
		opts *go_drift.ConfirmOptions,
		preSigned bool,
		extraConfirmationOptions *ExtraConfirmationOptions,
	) (*TxSigAndSlot, error)

	SendTransaction(
		tx *solana.Transaction,
		opts *go_drift.ConfirmOptions,
		preSigned bool,
		extraConfirmationOptions *ExtraConfirmationOptions,
	) (*TxSigAndSlot, error)

	GetTransaction(
		ixs []solana.Instruction,
		lookupTableAccounts []addresslookuptable.KeyedAddressLookupTable,
		opts *go_drift.ConfirmOptions,
		blockhash string,
		sign bool,
	) (*solana.Transaction, error)

	SendRawTransaction(
		rawTransaction []byte,
		opts *go_drift.ConfirmOptions,
	) (*TxSigAndSlot, error)
	SimulateTransaction(
		tx *solana.Transaction,
	) bool

	GetTimeoutCount() uint64
}

type TxSender struct {
	Wallet solana.Wallet
}
