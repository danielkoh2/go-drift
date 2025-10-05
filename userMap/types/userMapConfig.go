package types

import (
	"driftgo/drift/types"
	"driftgo/lib/geyser"
	"github.com/gagliardetto/solana-go/rpc"
)

// passed into UserMap.getUniqueAuthorities to filter users
type UserAccountFilterCriteria struct {
	// only return users that have open orders
	HasOpenOrders bool
}

type UserMapSubscriptionConfig struct {
	ResubTimeoutMs int64
	Commitment     rpc.CommitmentType
}

type UserMapConfig struct {
	DriftClient types.IDriftClient
	// connection object to use specifically for the UserMap. If undefined, will use the driftClient's connection
	Connection *geyser.Connection

	SubscriptionConfig UserMapSubscriptionConfig
	// True to skip the initial load of userAccounts via getProgramAccounts
	SkipInitialLoad bool

	// True to include idle users when loading. Defaults to false to decrease # of accounts subscribed to.
	IncludeIdle bool

	// Whether to skip loading available perp/spot positions and open orders
	FastDecode bool

	// If true, will not do a full sync whenever StateAccount.numberOfSubAccounts changes.
	// default behavior is to do a full sync on changes.
	DisableSyncOnTotalAccountsChange bool

	Commitment rpc.CommitmentType
}
