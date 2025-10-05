package orderSubscriber

import (
	"driftgo/drift/types"
	"driftgo/lib/drift"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type OrderSubscriberConfig struct {
	DriftClient      types.IDriftClient
	SkipInitialLoad  *bool
	ResubTimeoutMs   *int64
	ResyncIntervalMs *int64
	Commitment       *rpc.CommitmentType
	FastDecode       *bool
	DecodeData       *bool
}

type OrderSubscriberEvents interface {
	OrderCreated(*drift.User, []*drift.Order, solana.PublicKey, uint64, string)
	UserUpdated(*drift.User, solana.PublicKey, uint64, string)
	UpdateReceived(solana.PublicKey, uint64, string)
}

type OrderUserUpdatedEvent struct {
	UserAccount *drift.User
	AccountId   solana.PublicKey
	Slot        uint64
}

type OrderCreatedEvent struct {
	UserAccount *drift.User
	NewOrders   []*drift.Order
	AccountId   solana.PublicKey
	Slot        uint64
}
