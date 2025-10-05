package dlob

import (
	"driftgo/lib/drift"
	"github.com/gagliardetto/solana-go"
)

type DLOBOrder struct {
	User  solana.PublicKey
	Order *drift.Order
}

type DLOBOrders []*DLOBOrder
