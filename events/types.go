package events

import (
	"driftgo/utils"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type EventSubscriptionOptions struct {
	Address           solana.PublicKey
	EventTypes        []EventType
	MaxEventsPerType  *int
	Commitment        *rpc.CommitmentType
	MaxTx             *int
	LogProviderConfig *GeyserLogProviderConfig
	// when the subscription starts, client might want to backtrack and fetch old tx's
	// this specifies how far to backtrack
	UntilTx *rpc.TransactionSignature
}

var DefaultEventSubscriptionOptions = EventSubscriptionOptions{
	Address: solana.MPK(DriftProgramId),
	EventTypes: []EventType{
		EventTypeDepositRecord,
		EventTypeFundingPaymentRecord,
		EventTypeLiquidationRecord,
		EventTypeFundingRateRecord,
		EventTypeOrderRecord,
		EventTypeOrderActionRecord,
		EventTypeSettlePnlRecord,
		EventTypeNewUserRecord,
		EventTypeLPRecord,
		EventTypeInsuranceFundRecord,
		EventTypeSpotInterestRecord,
		EventTypeInsuranceFundStakeRecord,
		EventTypeCurveRecord,
		EventTypeSwapRecord,
	},
	MaxEventsPerType: utils.NewPtr(4096),
	Commitment:       utils.NewPtr(rpc.CommitmentConfirmed),
	MaxTx:            utils.NewPtr(4096),
	LogProviderConfig: &GeyserLogProviderConfig{
		ResubTimeoutMs:       utils.NewPtr(int64(6000)),
		MaxReconnectAttempts: nil,
		FallbackFrequency:    nil,
		fallbackBatchSize:    nil,
	},
	UntilTx: nil,
}

type Event struct {
	Data      interface{}
	EventType EventType
}
type WrappedEvent struct {
	Event
	TxSig solana.Signature
	Slot  uint64
}

type EventType string

const (
	EventTypeDepositRecord            EventType = "deposit_record"
	EventTypeFundingPaymentRecord     EventType = "funding_payment_record"
	EventTypeLiquidationRecord        EventType = "liquidation_record"
	EventTypeFundingRateRecord        EventType = "funding_rate_record"
	EventTypeOrderRecord              EventType = "order_record"
	EventTypeOrderActionRecord        EventType = "order_action_record"
	EventTypeSettlePnlRecord          EventType = "settle_pnl_record"
	EventTypeNewUserRecord            EventType = "new_user_record"
	EventTypeLPRecord                 EventType = "lp_record"
	EventTypeInsuranceFundRecord      EventType = "insurance_fund_record"
	EventTypeSpotInterestRecord       EventType = "spot_interest_record"
	EventTypeInsuranceFundStakeRecord EventType = "insurance_fund_stake_record"
	EventTypeCurveRecord              EventType = "curve_record"
	EventTypeSwapRecord               EventType = "swap_record"
	EventTypeRaw                      EventType = "raw"
)

type LogProviderCallback func(txSig solana.Signature, slot uint64, logs []string, mostRecentBlockTime int64)

type ILogProvider interface {
	IsSubscribed() bool
	Subscribe(callback LogProviderCallback, skipHistory ...bool) bool
	Unsubscribe(external ...bool) bool
}

type ILogProviderConfig interface{}

type GeyserLogProviderConfig struct {
	ILogProviderConfig
	ResubTimeoutMs       *int64
	MaxReconnectAttempts *int64
	FallbackFrequency    *int64
	fallbackBatchSize    *int64
}
