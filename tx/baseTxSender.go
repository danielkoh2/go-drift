package tx

import (
	"context"
	drift "driftgo"
	"github.com/gagliardetto/solana-go"
	addresslookuptable "github.com/gagliardetto/solana-go/programs/address-lookup-table"
	"github.com/gagliardetto/solana-go/rpc"
	"time"
)

const DEFAULT_TIMEOUT = 35000

type BaseTxSender struct {
	ITxSender
	TxSender

	connection      *rpc.Client
	opts            drift.ConfirmOptions
	timeout         int64
	timeoutCount    uint64
	RecentSlot      uint64
	RecentBlockHash solana.Hash
}

func CreateBaseTxSender(
	connection *rpc.Client,
	wallet solana.Wallet,
	opts *drift.ConfirmOptions,
	timeout int64,
) *BaseTxSender {
	txSender := &BaseTxSender{
		TxSender: TxSender{
			Wallet: wallet,
		},
		connection:   connection,
		opts:         *opts,
		timeout:      timeout,
		timeoutCount: 0,
	}
	txSender.SubscribeBlockHash()
	return txSender
}

func (p *BaseTxSender) Send(
	tx *solana.Transaction,
	opts *drift.ConfirmOptions,
	preSigned bool,
	extraConfirmationOptions *ExtraConfirmationOptions,
) (*TxSigAndSlot, error) {
	if opts == nil {
		opts = &p.opts
	}
	signedTx := p.prepareTx(tx, &opts.TransactionOpts, preSigned)
	if extraConfirmationOptions != nil {
		extraConfirmationOptions.OnSignedCb()
	}
	rawTx, err := signedTx.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return p.SendRawTransaction(rawTx, opts)
}

func (p *BaseTxSender) prepareTx(
	tx *solana.Transaction,
	opts *rpc.TransactionOpts,
	preSigned bool,
) *solana.Transaction {
	if preSigned {
		return tx
	}
	signedTx := *tx
	_, _ = signedTx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if p.Wallet.PublicKey().Equals(key) {
			return &p.Wallet.PrivateKey
		}
		return nil
	})
	return &signedTx
}

func (p *BaseTxSender) GetTransaction(
	ixs []solana.Instruction,
	lookupTableAccounts []addresslookuptable.KeyedAddressLookupTable,
	opts *drift.ConfirmOptions,
	blockhash string,
	sign bool,
) (*solana.Transaction, error) {
	var addressTables = make(map[solana.PublicKey]solana.PublicKeySlice)
	for _, table := range lookupTableAccounts {
		addressTables[table.Key] = table.State.Addresses
	}
	hash, err := solana.HashFromBase58(blockhash)
	tx, err := solana.NewTransaction(
		ixs,
		hash,
		solana.TransactionPayer(p.Wallet.PublicKey()),
		solana.TransactionAddressTables(addressTables),
	)
	if err != nil {
		return nil, err
	}
	if sign {
		_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
			if p.Wallet.PublicKey().Equals(key) {
				return &p.Wallet.PrivateKey
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return tx, nil
}

func (p *BaseTxSender) SendTransaction(
	tx *solana.Transaction,
	opts *drift.ConfirmOptions,
	preSigned bool,
	extraConfirmationOptions *ExtraConfirmationOptions,
) (*TxSigAndSlot, error) {
	var signedTx *solana.Transaction
	if preSigned {
		signedTx = tx
	} else {
		_, _ = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
			if p.Wallet.PublicKey().Equals(key) {
				return &p.Wallet.PrivateKey
			}
			return nil
		})
		signedTx = &(*tx)
	}
	if extraConfirmationOptions != nil {
		extraConfirmationOptions.OnSignedCb()
	}
	if opts == nil {
		opts = &p.opts
	}
	rawTx, err := signedTx.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return p.SendRawTransaction(rawTx, opts)
}

func (p *BaseTxSender) SendRawTransaction(
	rawTransaction []byte,
	opts *drift.ConfirmOptions,
) (*TxSigAndSlot, error) {
	txSig, err := p.connection.SendRawTransactionWithOpts(context.TODO(), rawTransaction, opts.TransactionOpts)
	if err != nil {
		return nil, err
	}
	return &TxSigAndSlot{
		TxSig: txSig,
		Slot:  0,
	}, nil
}

func (p *BaseTxSender) SimulateTransaction(
	tx *solana.Transaction,
) bool {
	return true
}

func (p *BaseTxSender) GetTimeoutCount() uint64 {
	return p.timeoutCount
}

func (p *BaseTxSender) SubscribeBlockHash() {
	ticker := time.NewTicker(time.Millisecond * 100)
	go func() {
		for {
			select {
			case <-ticker.C:
				out, err := p.connection.GetRecentBlockhash(
					context.TODO(),
					rpc.CommitmentFinalized,
				)
				if err == nil {
					p.RecentSlot = out.Context.Slot
					p.RecentBlockHash = out.Value.Blockhash
				}
			}
		}
	}()
}
