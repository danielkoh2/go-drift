package types

import (
	"driftgo/accounts"
	"driftgo/common"
	drifttype "driftgo/drift/types"
	"driftgo/lib/drift"
	"github.com/gagliardetto/solana-go"
)

type UserStatus struct {
	LastOrdersHash [16]byte
	LastUpdated    int64
	UpdatedCount   int64
	UpdateRate     int64
}
type IUserMap interface {
	Subscribe()
	Unsubscribe()
	AddPubkey(
		userAccountPublicKey solana.PublicKey,
		userAccount *drift.User,
		slot *uint64,
		subscriptionConfig *drifttype.UserSubscriptionConfig,
		buffer []byte,
	)
	Has(key string) bool
	Get(key string) drifttype.IUser
	GetUserStatus(key string) *UserStatus
	GetWithSlot(key string) *accounts.DataAndSlot[drifttype.IUser]
	MustGet(
		key string,
		accountSubscription *drifttype.UserSubscriptionConfig,
	) drifttype.IUser
	MustGetWithSlot(
		key string,
		accountSubscription *drifttype.UserSubscriptionConfig,
	) *accounts.DataAndSlot[drifttype.IUser]
	GetUserAuthority(key string) solana.PublicKey

	//UpdateWithOrderRecord(record *OrderRecord)

	Values() *common.Generator[drifttype.IUser, int]
	ValuesWithSlot() *common.Generator[*accounts.DataAndSlot[drifttype.IUser], int]
	Entries() *common.Generator[drifttype.IUser, string]
	EntriesWithSlot() *common.Generator[*accounts.DataAndSlot[drifttype.IUser], string]
	Size() int
	GetUniqueAuthorities(
		filterCriteria *UserAccountFilterCriteria,
	) []solana.PublicKey
	Sync()
}
