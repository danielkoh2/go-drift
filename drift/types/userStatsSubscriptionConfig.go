package types

import (
	"driftgo/accounts"
	"github.com/gagliardetto/solana-go/rpc"
)

type UserStatsSubscriptionConfig struct {
	ResubTimeoutMs *int64
	Commitment     *rpc.CommitmentType
	UsePolling     bool
	AccountLoader  *accounts.BulkAccountLoader
}
