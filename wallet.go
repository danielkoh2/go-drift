package go_drift

import "github.com/gagliardetto/solana-go"

type Wallet struct {
	IWallet
	PrivateKey solana.PrivateKey
}

func (p *Wallet) GetPublicKey() solana.PublicKey {
	return p.PrivateKey.PublicKey()
}

func (p *Wallet) GetPrivateKey() solana.PrivateKey {
	return p.PrivateKey
}

func (p *Wallet) GetWallet() solana.Wallet {
	return solana.Wallet{PrivateKey: p.PrivateKey}
}
func (p *Wallet) SignTransaction(tx *solana.Transaction) *solana.Transaction {
	_, _ = tx.PartialSign(func(key solana.PublicKey) *solana.PrivateKey {
		if p.PrivateKey.PublicKey().Equals(key) {
			return &p.PrivateKey
		}
		return nil
	})
	return tx
}

func (p *Wallet) SignAllTransactions(txs []*solana.Transaction) []*solana.Transaction {
	for _, tx := range txs {
		_, _ = tx.PartialSign(func(key solana.PublicKey) *solana.PrivateKey {
			if p.PrivateKey.PublicKey().Equals(key) {
				return &p.PrivateKey
			}
			return nil
		})
	}
	return txs
}
