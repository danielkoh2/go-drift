package types

import (
	drift "driftgo/drift/types"
	drift2 "driftgo/lib/drift"
	"github.com/gagliardetto/solana-go"
	"math/big"
)

type DLOBSubscriptionConfig struct {
	DriftClient                    drift.IDriftClient
	DlobSource                     IDLOBSource
	SlotSource                     ISlotSource
	UpdateFrequency                int64
	DisableChangeByUserOrderUpdate bool
	DisableHandleOrderRecord       bool
}

type IDLOBSubscribeEvents interface {
	Update(dlob IDLOB)
	Error(e error)
}

type IDLOBSource interface {
	GetDLOB(slot uint64) IDLOB
}

type ISlotSource interface {
	GetSlot() uint64
}

type BulkOrder struct {
	Order            *drift2.Order
	UserAccount      solana.PublicKey
	Slot             uint64
	BaseAmountFilled *big.Int
}
