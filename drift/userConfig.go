package drift

import (
	drifttype "driftgo/drift/types"
	"driftgo/lib/event"
	"github.com/gagliardetto/solana-go"
)

type UserConfig struct {
	AccountSubscription  drifttype.UserSubscriptionConfig
	DriftClient          drifttype.IDriftClient
	UserAccountPublicKey solana.PublicKey
	EventEmitter         *event.EventEmitter
}
