package types

import (
	"driftgo/accounts"
	"github.com/gagliardetto/solana-go/rpc"
)

type UserSubscriptionConfig struct {
	ResubTimeoutMs   *int64
	Commitment       *rpc.CommitmentType
	UseCustom        bool
	CustomSubscriber accounts.IUserAccountSubscriber
}
