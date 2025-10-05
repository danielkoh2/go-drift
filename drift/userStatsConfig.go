package drift

import (
	"driftgo/drift/types"
	"github.com/gagliardetto/solana-go"
)

type UserStatsConfig struct {
	AccountSubscription       types.UserStatsSubscriptionConfig
	DriftClient               types.IDriftClient
	UserStatsAccountPublicKey solana.PublicKey
}
